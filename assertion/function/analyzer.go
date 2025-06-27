//  Copyright (c) 2023 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package function implements a sub-analyzer to create full triggers for each function declaration.
package function

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion/anonymousfunc"
	"go.uber.org/nilaway/assertion/function/assertiontree"
	"go.uber.org/nilaway/assertion/function/functioncontracts"
	"go.uber.org/nilaway/assertion/structfield"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
)

const _doc = "Build the trees of assertions for each function in this package, propagating them to " +
	"entry and then matching them with possible sources of production to create a list of triggers " +
	"that can then be matched against a set of annotations to generate nil flow errors"

// Analyzer here is the analyzer than generates assertions and passes them onto the accumulator to
// be matched against annotations
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_function_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[[]annotation.FullTrigger])(nil)),
	Requires: []*analysis.Analyzer{
		config.Analyzer,
		ctrlflow.Analyzer,
		structfield.Analyzer,
		anonymousfunc.Analyzer,
		functioncontracts.Analyzer,
	},
}

// This limit is in place to prevent the expensive assertions analyzer from being run on
// overly-sized functions. A possible alternative to this is capping on size of CFG in nodes
// instead.
// TODO: test how often (if ever) this is hit
const _maxFuncSizeInBytes = 10000

// functionResult is the struct that stores the results for analyzing a function declaration.
type functionResult struct {
	// triggers is the slice of triggers generated from analyzing a particular function.
	triggers []annotation.FullTrigger
	// err stores any error occurred during the analysis.
	err error
	// index is the index of the function declaration in the package. This is particularly
	// important since currently we have hidden coupling in NilAway that requires the generated
	// triggers be placed in order of their declarations. Here, the index will ensure that we can
	// place the triggers in their original order, even though the analyses of function
	// declarations can be parallelized.
	// TODO: remove this.
	index int
	// funcDecl is the function declaration itself.
	funcDecl *ast.FuncDecl
}

func run(pass *analysis.Pass) ([]annotation.FullTrigger, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)
	if !conf.IsPkgInScope(pass.Pkg) {
		return nil, nil
	}

	// Construct experimental features. By default, enable all features on NilAway itself.
	functionConfig := assertiontree.FunctionConfig{}
	if strings.HasPrefix(pass.Pkg.Path(), config.NilAwayPkgPathPrefix) { //nolint:revive
		// TODO: enable struct initialization flag (tracked in Issue #23).
		// TODO: enable anonymous function flag.
	} else {
		functionConfig.EnableStructInitCheck = conf.ExperimentalStructInitEnable
		functionConfig.EnableAnonymousFunc = conf.ExperimentalAnonymousFuncEnable
	}

	ctrlflowResult := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	anonymousFuncResult := pass.ResultOf[anonymousfunc.Analyzer].(*analysishelper.Result[map[*ast.FuncLit]*anonymousfunc.FuncLitInfo])
	contractsResult := pass.ResultOf[functioncontracts.Analyzer].(*analysishelper.Result[functioncontracts.Map])
	if err := errors.Join(anonymousFuncResult.Err, contractsResult.Err); err != nil {
		return nil, err
	}

	funcLitMap, funcContracts := anonymousFuncResult.Res, contractsResult.Res

	// Create a fake ident map for the fake func decl nodes to be shared for all function contexts.
	pkgFakeIdentMap := make(map[*ast.Ident]types.Object)
	for _, info := range funcLitMap {
		pkgFakeIdentMap[info.FakeFuncDecl.Name] = info.FakeFuncObj
	}

	// Set up variables for synchronization and communication.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	funcChan := make(chan functionResult)
	// We use this to keep track of the index of the function declaration we are analyzing.
	// TODO: remove this once  is done.
	var funcIndex int
	for _, file := range pass.Files {
		// Skip if a file is marked to be ignored, or it is not in scope of our analysis.
		if !conf.IsFileInScope(file) {
			continue
		}

		// Collect all function declarations and function literals if anonymous function support
		// is enabled.
		var funcs []ast.Node
		for _, decl := range file.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok {
				funcs = append(funcs, f)
			}
		}
		if functionConfig.EnableAnonymousFunc {
			// We need a stable order of triggers for inference. However, the
			// fake func decl nodes generated from the anonymous function analyzer are stored in
			// a map. Hence, here we traverse the file and append the fake func decl nodes in
			// depth-first order.
			ast.Inspect(file, func(node ast.Node) bool {
				if f, ok := node.(*ast.FuncLit); ok {
					funcs = append(funcs, f)
				}
				return true
			})
		}

		for _, fun := range funcs {
			// Retrieve the auxiliary information about a function to be analyzed, since it is
			// slightly different to do so for function declarations and function literals.
			var (
				funcDecl *ast.FuncDecl
				funcLit  *ast.FuncLit
				graph    *cfg.CFG
			)
			switch f := fun.(type) {
			case *ast.FuncDecl:
				funcDecl, funcLit, graph = f, nil, ctrlflowResult.FuncDecl(f)
			case *ast.FuncLit:
				info, ok := funcLitMap[f]
				if !ok {
					panic(fmt.Sprintf("no func lit info found for anonymous function %v", pass.Fset.Position(f.Pos())))
				}

				funcDecl, funcLit, graph = info.FakeFuncDecl, f, ctrlflowResult.FuncLit(f)
			default:
				panic(fmt.Sprintf("unrecognized function type %T", f))
			}

			// Skip if function declaration has an empty body.
			if funcDecl.Body == nil {
				continue
			}
			// Skip if the function is too large.
			funcSizeInBytes := int(funcDecl.Body.Rbrace - funcDecl.Body.Lbrace)
			if funcSizeInBytes > _maxFuncSizeInBytes {
				continue
			}

			// Now, analyze the function declarations concurrently.
			wg.Add(1)
			funcContext := assertiontree.NewFunctionContext(
				pass, funcDecl, funcLit, functionConfig, funcLitMap, pkgFakeIdentMap, funcContracts)
			go analyzeFunc(ctx, pass, funcDecl, funcContext, graph, funcIndex, funcChan, &wg)
			funcIndex++
		}
	}

	// Spawn another goroutine that will close the channel when all analyses are done. This makes
	// sure the channel receive logic in the main thread (below) can properly terminate.
	go func() {
		wg.Wait()
		close(funcChan)
	}()

	// Now we collect the results for each function analysis. Note that due to hidden couplings in
	// NilAway, the order of the triggers must align with the order of the function declarations (
	// as if the analyses were done serially). So we first store the result triggers in order,
	// then flatten the slice.
	// TODO: remove this extra logic once  is done.
	var err error
	funcTriggers := make([][]annotation.FullTrigger, funcIndex)
	triggerCount := 0
	funcResults := map[*types.Func]*functionResult{}
	for r := range funcChan {
		if r.err != nil {
			err = errors.Join(err, r.err)
		} else {
			funcTriggers[r.index] = r.triggers
			triggerCount += len(r.triggers)

			funcObj, ok := pass.TypesInfo.ObjectOf(r.funcDecl.Name).(*types.Func)
			if !ok {
				continue
			}
			funcRes := r
			funcResults[funcObj] = &funcRes
		}
	}

	// Duplicate triggers in contracted functions in the callers of the function
	if len(funcContracts) != 0 {
		duplicateFullTriggersFromContractedFunctionsToCallers(pass, funcContracts, funcTriggers,
			funcResults)
	}

	// Flatten the triggers
	triggers := make([]annotation.FullTrigger, 0, triggerCount)
	for _, s := range funcTriggers {
		triggers = append(triggers, s...)
	}

	return triggers, err
}

// duplicateFullTriggersFromContractedFunctionsToCallers duplicates all the full triggers that have
// FuncParam producer or UseAsReturn consumer or both, from the contracted functions to the callers
// of all the contracted functions. This is necessary because we have created new
// producers/consumers for argument pass or result return at every call site of the contracted
// function. In order to connect such producers/consumers back to the contracted functions, we
// create new full triggers that duplicates the original full triggers of the contracted functions
// but uses the new producers/consumers instead.
func duplicateFullTriggersFromContractedFunctionsToCallers(
	pass *analysis.Pass,
	funcContracts functioncontracts.Map,
	funcTriggers [][]annotation.FullTrigger,
	funcResults map[*types.Func]*functionResult,
) {

	// Find all the calls to contracted functions
	// callsByCtrtFunc is a mapping: contracted function -> caller -> all the call expressions
	callsByCtrtFunc := map[*types.Func]map[*types.Func][]*ast.CallExpr{}
	for funcObj, r := range funcResults {
		for ctrFunc, calls := range findCallsToContractedFunctions(r.funcDecl, pass, funcContracts) {
			for _, call := range calls {
				// TODO: Ideally, we should do
				//
				// if _, ok := callsByCtrtFunc[ctrFunc]; !ok {
				//  callsByCtrtFunc[ctrFunc] = map[*types.Func][]*ast.CallExpr{}
				// }
				// callsByCtrtFunc[ctrFunc][funcObj] = append(callsByCtrtFunc[ctrFunc][funcObj], call)
				//
				// However, NilAway complains that callsByCtrtFunc[ctrFunc] can be nil. Thus, we
				// introduce an intermediate variable v and the following instead.
				v, ok := callsByCtrtFunc[ctrFunc]
				if !ok {
					v = map[*types.Func][]*ast.CallExpr{}
					callsByCtrtFunc[ctrFunc] = v
				}
				v[funcObj] = append(v[funcObj], call)
			}
		}
	}

	// For every contracted function, duplicate some of its full triggers (that involves param or
	// return) into all the callers
	dupTriggers := map[*types.Func][]annotation.FullTrigger{}
	for ctrtFunc, calls := range callsByCtrtFunc {
		r := funcResults[ctrtFunc]
		if r == nil {
			// The contracted function is imported from upstream, and the local package analysis
			// does not involve it.
			continue
		}
		for _, trigger := range r.triggers {
			// If the full trigger has a FuncParam producer or a UseAsReturn consumer, then create
			// a duplicated (possibly controlled) full trigger from it and add the created full
			// trigger to every caller.
			_, isParamProducer := trigger.Producer.Annotation.(*annotation.FuncParam)
			_, isReturnConsumer := trigger.Consumer.Annotation.(*annotation.UseAsReturn)
			if !isParamProducer && !isReturnConsumer {
				// No need to duplicate the full trigger
				continue
			}
			// Duplicate the full trigger in every caller
			for caller, callExprs := range calls {
				for _, callExpr := range callExprs {
					dupTrigger := duplicateFullTrigger(trigger, ctrtFunc, callExpr, pass,
						isParamProducer, isReturnConsumer)

					// Store the duplicated full trigger
					dupTriggers[caller] = append(dupTriggers[caller], dupTrigger)
				}
			}
		}
	}

	// Update funcTriggers with duplicated triggers
	for funcObj, triggers := range dupTriggers {
		r := funcResults[funcObj]
		if r == nil {
			// Should not happen since we would not have created the duplicated triggers if the
			// contracted function is not involved in the analysis of local package.
			panic(fmt.Sprintf("did not find the contracted function %s in funcResults", funcObj.Id()))
		}
		funcTriggers[r.index] = append(funcTriggers[r.index], triggers...)
	}
}

// duplicateFullTrigger creates a (possibly controlled) full trigger from the given full trigger
// with FuncParam producer or UseAsReturn consumer or both.
// Precondition: isParamProducer or isReturnConsumer is true; also they can be both true.
func duplicateFullTrigger(
	trigger annotation.FullTrigger,
	callee *types.Func,
	callExpr *ast.CallExpr,
	pass *analysis.Pass,
	isParamProducer bool,
	isReturnConsumer bool,
) annotation.FullTrigger {
	// TODO: what if we have more than one parameter, planned in future revisions
	argExpr := callExpr.Args[0]
	argLoc := util.PosToLocation(argExpr.Pos(), pass)

	// Create the duplicated full trigger
	// TODO: we just copy the pointer for producer and consumer because I don't see a problem when
	//  two full triggers share a producer or consumer. We do deep duplication for the param or
	//  return related producer/consumer, i.e., FuncParam, FuncReturn, ArgPass, UseAsReturn, and I
	//  don't see other conditional producer/consumer that can be shared between two call sites.
	//  If we did see such cases in the future, we would want to add a deep copy function for every
	//  ProduceTrigger or ConsumeTrigger type and make the deep copy here. In this case, we would
	//  also want to see if it is OK to share the underlying site of the producer/consumer in
	//  inference engine because we would not want to see a conflict at this site due to different
	//  call sites.
	dupTrigger := annotation.FullTrigger{
		Producer:               trigger.Producer,
		Consumer:               trigger.Consumer,
		Controller:             nil,
		CreatedFromDuplication: true,
	}
	if isParamProducer {
		dupTrigger.Producer = annotation.DuplicateParamProducer(trigger.Producer, argLoc)
	}
	if isReturnConsumer {
		retLoc := util.PosToLocation(callExpr.Pos(), pass)
		dupTrigger.Consumer = annotation.DuplicateReturnConsumer(trigger.Consumer, retLoc)
		// Set up the site that controls the controlled full trigger to be created
		c := annotation.NewCallSiteParamKey(callee, 0, argLoc)
		dupTrigger.Controller = c
	}

	return dupTrigger
}

// findCallsToContractedFunctions finds all the calls to the contracted functions in the given
// function, and returns a map from every called contracted function to the call expressions that
// call it.
func findCallsToContractedFunctions(
	funcNode *ast.FuncDecl,
	pass *analysis.Pass,
	functionContracts functioncontracts.Map,
) map[*types.Func][]*ast.CallExpr {
	calls := map[*types.Func][]*ast.CallExpr{}
	ast.Inspect(funcNode, func(n ast.Node) bool {
		callExpr, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		ident := util.FuncIdentFromCallExpr(callExpr)
		if ident == nil {
			return true
		}

		funcObj, ok := pass.TypesInfo.ObjectOf(ident).(*types.Func)
		if !ok {
			return true
		}

		// TODO: for now we find the functions with only a single contract nonnil -> nonnil. If we
		//  want to support multiple contracts or contracts with multiple/other values not only we
		//  should update here, but we should also make changes to other parts of duplicating
		//  triggers.
		if !hasOnlyNonNilToNonNilContract(functionContracts, funcObj) {
			return true
		}
		calls[funcObj] = append(calls[funcObj], callExpr)
		return true
	})
	return calls
}

// hasOnlyNonNilToNonNilContract returns whether the given function has only one contract that is
// nonnil->nonnil.
func hasOnlyNonNilToNonNilContract(funcContracts functioncontracts.Map, funcObj *types.Func) bool {
	contracts, ok := funcContracts[funcObj]
	if !ok || len(contracts) != 1 {
		return false
	}
	ctr := contracts[0]
	return len(ctr.Ins) == 1 && ctr.Ins[0] == functioncontracts.NonNil &&
		len(ctr.Outs) == 1 && ctr.Outs[0] == functioncontracts.NonNil
}

// analyzeFunc analyzes a given function declaration and emit generated triggers, or an error if
// something went wrong during the analysis. It is mainly a wrapper function for
// assertiontree.BackpropAcrossFunc with synchronization and communication support for concurrency.
// The actual result will be sent via the channel.
func analyzeFunc(
	ctx context.Context,
	pass *analysis.Pass,
	funcDecl *ast.FuncDecl,
	funcContext assertiontree.FunctionContext,
	graph *cfg.CFG,
	index int,
	funcChan chan functionResult,
	wg *sync.WaitGroup,
) {
	// Deferred statements are pushed to a stack, which are executed in LIFO order. Calling
	// wg.Done() would signal the main process that this goroutine is done, and the main process
	// will close the result channel. However, our panic recovery handler still needs access to
	// the result channel to send the error back. Therefore, we _must_ call `wg.Done()` after the
	// panic recovery handler (meaning we defer it first).
	defer wg.Done()

	// As a last resort, convert the panics into errors and return.
	defer func() {
		if r := recover(); r != nil {
			e := fmt.Errorf("INTERNAL PANIC: %s\n%s", r, string(debug.Stack()))
			funcChan <- functionResult{err: e, index: index, funcDecl: funcDecl}
		}
	}()

	// Do the actual backpropagation.
	funcTriggers, _, _, err := assertiontree.BackpropAcrossFunc(ctx, pass, funcDecl, funcContext, graph)

	// If any error occurs in back-propagating the function, we wrap the error with more information.
	if err != nil {
		pos := pass.Fset.Position(funcDecl.Pos())
		err = fmt.Errorf("analyzing function %s at %s:%d.%d: %w", funcDecl.Name, pos.Filename, pos.Line, pos.Column, err)
	}

	funcChan <- functionResult{
		triggers: funcTriggers,
		err:      err,
		index:    index,
		funcDecl: funcDecl,
	}
}
