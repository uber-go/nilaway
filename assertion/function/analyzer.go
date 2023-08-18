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
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
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
	"go.uber.org/nilaway/inference"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/types/objectpath"
)

const _doc = "Build the trees of assertions for each function in this package, propagating them to " +
	"entry and then matching them with possible sources of production to create a list of triggers " +
	"that can then be matched against a set of annotations to generate nil flow errors"

// Result is the result struct for the Analyzer.
type Result struct {
	// FullTriggers is the slice of full triggers generated from the assertion analysis.
	FullTriggers []annotation.FullTrigger
	// Errors is the slice of errors if errors happened during analysis. We put the errors here as
	// part of the result of this sub-analyzer so that the upper-level analyzers can decide what
	// to do with them.
	Errors []error
	// Implications is the extra implications generated from the function contracts analysis, to be
	// used in the inference stage.
	Implications []*inference.Implication
}

// Analyzer here is the analyzer than generates assertions and passes them onto the accumulator to
// be matched against annotations
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_function_analyzer",
	Doc:        _doc,
	Run:        run,
	FactTypes:  []analysis.Fact{new(Cache)},
	ResultType: reflect.TypeOf((*Result)(nil)).Elem(),
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

// Cache stores the implications of all the contracted functions defined in upstream packages,
// which represents the full triggers that need to be duplicated for each contracted function.
type Cache struct {
	// ImplicationsToDup is a map from the id of the contracted function to the implications
	// (duplicable full triggers) of that function.
	ImplicationsToDup map[string][]*inference.Implication
}

// AFact enables use of the facts passing mechanism in Go's analysis framework.
func (*Cache) AFact() {}

func run(pass *analysis.Pass) (result interface{}, _ error) {
	// As a last resort, we recover from a panic when running the analyzer, convert the panic to
	// an error and return.
	defer func() {
		if r := recover(); r != nil {
			// Deferred functions are executed after a result is generated, so here we modify the
			// return value `result` in-place.
			e := fmt.Errorf("INTERNAL PANIC: %s\n%s", r, string(debug.Stack()))
			if retResult, ok := result.(Result); ok {
				retResult.Errors = append(retResult.Errors, e)
			} else {
				result = Result{Errors: []error{e}}
			}
		}
	}()

	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return Result{}, nil
	}

	ctrlflowResult := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	funcLitMap := pass.ResultOf[anonymousfunc.Analyzer].(anonymousfunc.Result).FuncLitMap
	funcContracts := pass.ResultOf[functioncontracts.Analyzer].(functioncontracts.Result).FunctionContracts

	// Create a fake ident map for the fake func decl nodes to be shared for all function contexts.
	pkgFakeIdentMap := make(map[*ast.Ident]types.Object)
	for _, info := range funcLitMap {
		pkgFakeIdentMap[info.FakeFuncDecl.Name] = info.FakeFuncObj
	}

	// Set up variables for synchronization and communication.
	ctx, cancel := context.WithTimeout(context.Background(), config.BackpropTimeout)
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

		// Construct config for analyzing the functions in this file. By default, enable all checks
		// on NilAway itself.
		functionConfig := assertiontree.FunctionConfig{}
		if strings.HasPrefix(pass.Pkg.Path(), config.NilAwayPkgPathPrefix) { //nolint:revive
			// TODO: enable struct initialization flag (tracked in Issue #23).
			// TODO: enable anonymous function flag.
		} else {
			functionConfig.StructInitCheckType = util.DocContainsStructInitCheck(file.Doc)
			functionConfig.EnableAnonymousFunc = util.DocContainsAnonymousFuncCheck(file.Doc)
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
			// Due to , we need a stable order of triggers for inference. However, the
			// fake func decl nodes generated from the anonymous function analyzer are stored in
			// a map. Hence, here we traverse the file and append the fake func decl nodes in
			// depth-first order.
			// TODO: remove this once  is done.
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
	var errs []error
	funcTriggers := make([][]annotation.FullTrigger, funcIndex)
	triggerCount := 0
	funcResults := map[*types.Func]*functionResult{}
	for r := range funcChan {
		if r.err != nil {
			errs = append(errs, r.err)
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
	var implicationsByFuncID map[string][]*inference.Implication
	if len(funcContracts) != 0 {
		primitivizer := NewPrimitivizer(pass)
		upstreamImplicationsToDup := pullUpstreamImplicationsToDup(pass)
		implicationsByFuncID = duplicateFullTriggersFromContractedFunctionsToCallers(
			pass, primitivizer, funcContracts, funcResults, upstreamImplicationsToDup)
		exportImplicationsToDup(pass, primitivizer, funcResults, funcContracts)
	}

	// Flatten the triggers and implications
	triggers := make([]annotation.FullTrigger, 0, triggerCount)
	implicatons := make([]*inference.Implication, 0)
	for _, s := range funcTriggers {
		triggers = append(triggers, s...)
	}
	for _, impl := range implicationsByFuncID {
		implicatons = append(implicatons, impl...)
	}
	return Result{FullTriggers: triggers, Errors: errs, Implications: implicatons}, nil
}

func pullUpstreamImplicationsToDup(pass *analysis.Pass) map[string][]*inference.Implication {
	implsToDup := map[string][]*inference.Implication{}
	facts := pass.AllPackageFacts()
	if len(facts) == 0 {
		return implsToDup
	}
	for _, f := range facts {
		switch c := f.Fact.(type) {
		case *Cache:
			for funcID, impls := range c.ImplicationsToDup {
				implsToDup[funcID] = append(implsToDup[funcID], impls...)
			}
		}
	}
	return implsToDup
}

func exportImplicationsToDup(
	pass *analysis.Pass,
	primitivizer *Primitivizer,
	funcResults map[*types.Func]*functionResult,
	funcContracts functioncontracts.Map,
) {
	implsToExport := map[string][]*inference.Implication{}
	for funcObj, r := range funcResults {
		funcID := funcObj.FullName()
		if _, ok := funcContracts[funcID]; !ok {
			// Skip if the function has no contract.
			continue
		}
		ctrts := funcContracts[funcID]
		if ctrts == nil {
			// should not happen since ctrtFunc is a contracted function
			panic(fmt.Sprintf(
				"Did not find the contracted function %s in funcContracts",
				funcID))
		}
		contract := ctrts[0]
		ctrtParamIndex := contract.IndexOfNonnilIn()
		ctrtRetIndex := contract.IndexOfNonnilOut()
		for _, trigger := range r.triggers {
			p, isContractedParam := trigger.Producer.Annotation.(annotation.FuncParam)
			if isContractedParam {
				isContractedParam = ctrtParamIndex == p.TriggerIfNilable.Ann.(annotation.ParamAnnotationKey).ParamNum
			}
			c, isContractedReturn := trigger.Consumer.Annotation.(annotation.UseAsReturn)
			if isContractedReturn {
				isContractedReturn = ctrtRetIndex == c.TriggerIfNonNil.Ann.(annotation.RetAnnotationKey).RetNum
			}
			if !isContractedParam && !isContractedReturn {
				continue
			}
			// only export implications that will be duplicated by the callers in downstream
			// packages, i.e., the implications with param producers or return consumers.
			impl := ToImplication(primitivizer, trigger, ctrtParamIndex, ctrtRetIndex)
			implsToExport[funcID] = append(implsToExport[funcID], impl)
		}
	}
	pass.ExportPackageFact(&Cache{ImplicationsToDup: implsToExport})
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
	primitivizer *Primitivizer,
	funcContracts functioncontracts.Map,
	funcResults map[*types.Func]*functionResult,
	upstreamImplicationsToDup map[string][]*inference.Implication,
) map[string][]*inference.Implication {

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
	implications := map[string][]*inference.Implication{}
	for ctrtFunc, calls := range callsByCtrtFunc {
		r := funcResults[ctrtFunc]
		if r == nil {
			// Find in the upstream packages
			ctrtFuncID := ctrtFunc.FullName()
			implsToDup := upstreamImplicationsToDup[ctrtFuncID]
			if implsToDup == nil {
				// TODO: we do not have the duplicated implications for the contracted function
				//  because those functions do not have FuncParam producer or UseAsReturn consumer.
				//  They have some special alternatives like UseAsErrorResult, to which we do not
				//  add CallSiteRetAnnotationKey yet.

				// This could lead to some false negatives since we duplicated call sites but not
				// connected them to the sites in contracted functions. However, I think this
				// should be rare. Anyway we will add the support for all producers/consumers using
				// ParamAnnotationKey and RetAnnotationKey in the future.
				continue
			}
			ctrts := funcContracts[ctrtFuncID]
			if ctrts == nil {
				// should not happen since ctrtFunc is a contracted function
				panic(fmt.Sprintf(
					"Did not find the contracted function %s in funcContracts", ctrtFuncID))
			}
			for _, impl := range implsToDup {
				// Duplicate the full trigger in every caller
				for caller, callExprs := range calls {
					for _, callExpr := range callExprs {
						dupImpl := duplicateImplication(pass, primitivizer, impl, ctrtFunc, callExpr)
						// Store the implication
						callerID := caller.FullName()
						implications[callerID] = append(implications[callerID], dupImpl)
					}
				}
			}
			return implications
		}
		ctrts := funcContracts[ctrtFunc.FullName()]
		if ctrts == nil {
			// should not happen since ctrtFunc is a contracted function
			panic(fmt.Sprintf(
				"Did not find the contracted function %s in funcContracts",
				ctrtFunc.FullName()))
		}
		contract := ctrts[0]
		ctrtParamIndex := contract.IndexOfNonnilIn()
		ctrtRetIndex := contract.IndexOfNonnilOut()
		for _, trigger := range r.triggers {
			// If the full trigger has a FuncParam producer or a UseAsReturn consumer, then create
			// a duplicated (possibly controlled) full trigger from it and add the created full
			// trigger to every caller.
			p, isContractedParam := trigger.Producer.Annotation.(annotation.FuncParam)
			if isContractedParam {
				isContractedParam = ctrtParamIndex == p.TriggerIfNilable.Ann.(annotation.ParamAnnotationKey).ParamNum
			}
			c, isContractedReturn := trigger.Consumer.Annotation.(annotation.UseAsReturn)
			if isContractedReturn {
				isContractedReturn = ctrtRetIndex == c.TriggerIfNonNil.Ann.(annotation.RetAnnotationKey).RetNum
			}
			if !isContractedParam && !isContractedReturn {
				// We only duplicate the full trigger if it is the right parameter which is the one
				// with contract value NONNIL in a general nonnil->nonnil contract.
				// TODO: However, this could be changed in the future if we support multiple
				//  contracts and/or other kinds of contract values.
				continue
			}
			// Duplicate the full trigger in every caller
			for caller, callExprs := range calls {
				for _, callExpr := range callExprs {
					dupTrigger := duplicateFullTrigger(trigger, ctrtFunc, callExpr, pass,
						ctrtParamIndex, isContractedParam, isContractedReturn)
					// Convert the duplicated full trigger into an implication
					implication := ToImplication(primitivizer, dupTrigger, ctrtParamIndex, ctrtRetIndex)
					// Store the implication
					callerID := caller.FullName()
					implications[callerID] = append(implications[callerID], implication)
				}
			}
		}
	}

	return implications
}

func duplicateImplication(
	pass *analysis.Pass,
	primitivizer *Primitivizer,
	impl *inference.Implication,
	ctrtFunc *types.Func,
	callExpr *ast.CallExpr,
) *inference.Implication {
	dupImpl := &inference.Implication{
		Producer:         impl.Producer,
		Consumer:         impl.Consumer,
		Assertion:        impl.Assertion,
		Controlled:       false,
		IsProducerValid:  impl.IsProducerValid,
		IsConsumerValid:  impl.IsConsumerValid,
		HasParamProducer: impl.HasParamProducer,
		HasRetConsumer:   impl.HasRetConsumer,
		CtrtParamIndex:   impl.CtrtParamIndex,
		CtrtRetIndex:     impl.CtrtRetIndex,
	}

	argExpr := callExpr.Args[dupImpl.CtrtParamIndex]
	argLoc := util.PosToLocation(argExpr.Pos(), pass)

	assertion := dupImpl.Assertion
	if dupImpl.HasParamProducer {
		// Convert the producer site originally generated from ParamAnnotationKey to a new producer
		// site as if from CallSiteParamAnnotationKey.
		if dupImpl.CtrtParamIndex == -1 {
			panic("ParamIndex is supposed to be set if HasParamProducer is true!")
		}
		callSiteParamKey := annotation.NewCallSiteParamKey(ctrtFunc, dupImpl.CtrtParamIndex, argLoc)
		// FuncParam.Kind() == conditional so we pass in false
		dupImpl.Producer = primitivizer.site(callSiteParamKey, false)

		// Change the assertion
		if prestring, ok := assertion.ProducerRepr.(annotation.FuncParamPrestring); ok {
			// The Location field is the only difference in the prestring from ParamAnnotationKey
			// and CallSiteParamAnnotationKey. See function (u FuncParam).Prestring().
			prestring.Location = argLoc.String()
		} else {
			panic(fmt.Sprintf("Expected type %T but got %T", annotation.FuncParamPrestring{}, assertion.ProducerRepr))
		}
	}
	if dupImpl.IsConsumerValid {
		// Convert the producer site originally generated from ReturnAnnotationKey to a new producer
		// site as if from CallSiteReturnAnnotationKey.
		retLoc := util.PosToLocation(callExpr.Pos(), pass)
		callSiteReturnKey := annotation.NewCallSiteRetKey(ctrtFunc, dupImpl.CtrtRetIndex, retLoc)
		// UseAsReturn.Kind() == conditional so we pass in false
		dupImpl.Consumer = primitivizer.site(callSiteReturnKey, false)

		// Set up controller
		c := annotation.NewCallSiteParamKey(ctrtFunc, dupImpl.CtrtParamIndex, argLoc)
		dupImpl.Controller = primitivizer.site(c, false)
		dupImpl.Controlled = true

		// Change the assertion
		if prestring, ok := assertion.ConsumerRepr.(annotation.UseAsReturnPrestring); ok {
			// The Location field is the only difference in the prestring from RetAnnotationKey and
			// CallSiteRetAnnotationKey. See function (u UseAsReturn).Prestring().
			prestring.Location = retLoc.String()
		}
	}
	return dupImpl
}

// duplicateFullTrigger creates a (possibly controlled) full trigger from the given full trigger
// with FuncParam producer or UseAsReturn consumer or both.
// Precondition: isParamProducer or isReturnConsumer is true; also they can be both true.
func duplicateFullTrigger(
	trigger annotation.FullTrigger,
	callee *types.Func,
	callExpr *ast.CallExpr,
	pass *analysis.Pass,
	ctrtParamIndex int,
	isParamProducer bool,
	isReturnConsumer bool,
) annotation.FullTrigger {
	// TODO: what if we have other kinds of contracts than a general nonnil->nonnil contract,
	//  planned in the future.

	// Assume the contract is a general nonnil->nonnil contract
	argExpr := callExpr.Args[ctrtParamIndex]
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
		c := annotation.NewCallSiteParamKey(callee, ctrtParamIndex, argLoc)
		dupTrigger.Controller = &c
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

// hasOnlyNonNilToNonNilContract returns true if the given function has only a single contract
// and the contract is a general nonnil -> nonnil contract, i.e., the contract has only one
// nonnil in input and only one nonnil in output, and all the other values are any.
func hasOnlyNonNilToNonNilContract(funcContracts functioncontracts.Map, funcObj *types.Func) bool {
	contracts, ok := funcContracts[funcObj.FullName()]
	if !ok || len(contracts) != 1 {
		return false
	}
	return contracts[0].IsGeneralNonnnilToNonnil()
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
	// As a last resort, convert the panics into errors and return.
	defer func() {
		if r := recover(); r != nil {
			e := fmt.Errorf("INTERNAL PANIC: %s\n%s", r, string(debug.Stack()))
			funcChan <- functionResult{err: e, index: index, funcDecl: funcDecl}
		}
	}()
	defer wg.Done()

	// Do the actual backpropagation.
	funcTriggers, err := assertiontree.BackpropAcrossFunc(ctx, pass, funcDecl, funcContext, graph)

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

// Primitivizer is able to convert full triggers and annotation sites to their primitive forms. It
// is useful for getting the correct primitive sites and positions for upstream objects due to the
// lack of complete position information in downstream analysis. For example:
//
// upstream/main.go:
// const GlobalVar *int
//
// downstream/main.go:
// func main() { print(*upstream.GlobalVar) }
//
// Here, when analyzing the upstream package we will export a primitive site for `GlobalVar` that
// encodes the package and site representations, and more importantly, the position information to
// uniquely identify the site. However, when analyzing the downstream package, the
// `upstream/main.go` file in the `analysis.Pass.Fset` will not have complete line and column
// information. Instead, the [importer] injects 65535 fake "lines" into the file, and the object
// we get for `upstream.GlobalVar` will contain completely different position information due to
// this hack (in practice, we have observed that the lines for the objects are correct, but not
// others). This leads to mismatches in our inference engine: we cannot find nilabilities of
// upstream objects in the imported InferredMap since their Position fields in the primitive sites
// are different. The Primitivizer here contains logic to fix such discrepancies so that the
// returned primitive sites for the same objects always contain correct (and same) Position information.
//
// [importer]: https://cs.opensource.google/go/x/tools/+/refs/tags/v0.7.0:go/internal/gcimporter/bimport.go;l=375-385;drc=c1dd25e80b559a5b0e8e2dd7d5bd1e946aa996a0;bpv=0;bpt=0
type Primitivizer struct {
	pass *analysis.Pass
	// upstreamObjPositions maps "pkg repr + object path" to the correct position.
	upstreamObjPositions map[string]token.Position
	files                map[string]fileInfo
	// execRoot is the cached bazel sandbox prefix for trimming the filenames.
	execRoot string
}

// NewPrimitivizer returns a new Primitivizer.
func NewPrimitivizer(pass *analysis.Pass) *Primitivizer {
	// To tackle the position discrepancies for upstream sites, we have added an ObjectPath field,
	// which can be used to uniquely identify an exported object relative to the package. Then,
	// we can simply cache the correct position information when importing InferredMaps, since the
	// positions collected in the upstream analysis are always correct. Later when querying upstream
	// objects in downstream analysis, we can look up the cache and fill in the correct position in
	// the returned primitive site instead.

	// Create a cache for upstream object positions.
	upstreamObjPositions := make(map[string]token.Position)
	for _, packageFact := range pass.AllPackageFacts() {
		cache, ok := packageFact.Fact.(*Cache)
		if !ok {
			continue
		}
		for _, impls := range cache.ImplicationsToDup {
			for _, impl := range impls {
				for _, site := range impl.AllSites() {
					if site.ObjectPath == "" {
						continue
					}

					objRepr := site.PkgRepr + "." + string(site.ObjectPath)
					if existing, ok := upstreamObjPositions[objRepr]; ok && existing != site.Position {
						/*config.WriteToLog(fmt.Sprintf(
						"conflicting position information on upstream object %q: existing: %v, got: %v",
						objRepr, existing, site.Position))*/
						panic(fmt.Sprintf(
							"conflicting position information on upstream object %q: existing: %v, got: %v",
							objRepr, existing, site.Position))
					}
					upstreamObjPositions[objRepr] = site.Position
				}
			}
		}
	}

	// Find the bazel execroot (i.e., random sandbox prefix) for trimming the file names.
	execRoot, err := os.Getwd()
	if err != nil {
		panic("cannot get current working directory")
	}
	// config.WriteToLog(fmt.Sprintf("exec root: %q", execRoot))

	// Iterate all files within the Fset (which includes upstream and current package files), and
	// store the mapping between its file name (modulo the bazel prefix) and the token.File object.
	files := make(map[string]fileInfo)
	pass.Fset.Iterate(func(file *token.File) bool {
		name, err := filepath.Rel(execRoot, file.Name())
		if err != nil {
			// For files in standard libraries, there is no bazel sandbox prefix, so we can just
			// keep the original name.
			name = file.Name()
		}
		files[name] = fileInfo{
			file:    file,
			isLocal: strings.HasSuffix(path.Dir(name), pass.Pkg.Path()),
		}
		return true
	})

	return &Primitivizer{
		pass:                 pass,
		upstreamObjPositions: upstreamObjPositions,
		files:                files,
		execRoot:             execRoot,
	}
}

// fullTrigger returns the primitive version of the full trigger.
func (p *Primitivizer) fullTrigger(trigger annotation.FullTrigger) inference.PrimitiveFullTrigger {
	// Expr is always nonnil, but our struct init analysis is capped at depth 1 so NilAway does not
	// know this fact. Here, we explicitly guard against such cases to provide a hint.
	if trigger.Consumer.Expr == nil {
		panic(fmt.Sprintf("consume trigger %v has a nil Expr", trigger.Consumer))
	}

	producer, consumer := trigger.Prestrings(p.pass)
	return inference.PrimitiveFullTrigger{
		ProducerRepr: producer,
		ConsumerRepr: consumer,
	}
}

// site returns the primitive version of the annotation site.
func (p *Primitivizer) site(key annotation.Key, isDeep bool) inference.PrimitiveSite {
	pkgRepr := ""
	if pkg := key.Object().Pkg(); pkg != nil {
		pkgRepr = pkg.Path()
	}

	objPath, err := objectpath.For(key.Object())
	if err != nil {
		// An error will occur when trying to get object path for unexported objects, in which case
		// we simply assign an empty object path.
		objPath = ""
	}

	var position token.Position
	// For upstream objects, we need to look up the local position cache for correct positions.
	if key.Object().Pkg() != nil && p.pass.Pkg != key.Object().Pkg() {
		// Correct upstream information may not always be in the cache: we may not even have it
		// since we skipped analysis for standard and 3rd party libraries.
		if p, ok := p.upstreamObjPositions[pkgRepr+"."+string(objPath)]; ok {
			// config.WriteToLog(fmt.Sprintf("found position cache for %s.%s: %v", pkgRepr, string(objPath), p))
			position = p
		} //else {
		// config.WriteToLog(fmt.Sprintf("did not find position cache for %s.%s", pkgRepr, string(objPath)))
		//}
	}

	// Default case (local objects or objects from skipped upstream packages), we can simply use
	// their Object.Pos() and retrieve the position information. However, we must trim the bazel
	// sandbox prefix from the filenames for cross-package references.
	if !position.IsValid() {
		position = p.pass.Fset.Position(key.Object().Pos())
		if name, err := filepath.Rel(p.execRoot, position.Filename); err == nil {
			position.Filename = name
		}
	}

	site := inference.PrimitiveSite{
		PkgRepr:    pkgRepr,
		Repr:       key.String(),
		IsDeep:     isDeep,
		Exported:   key.Object().Exported(),
		ObjectPath: objPath,
		Position:   position,
	}

	//if objPath != "" {
	// config.WriteToLog(fmt.Sprintf("objpath: %s.%s for site %v", pkgRepr, objPath, site))
	//}

	return site
}

// ToImplication converts a FullTrigger to an Implication. The FullTrigger must have a FuncParam
// Producer and a FuncReturn Consumer.
func ToImplication(
	primitivizer *Primitivizer,
	trigger annotation.FullTrigger,
	ctrtParamIndex int,
	ctrtRetIndex int,
) *inference.Implication {
	// Expr is always nonnil, but our struct init analysis is capped at depth 1 so NilAway does not
	// know this fact. Here, we explicitly guard against such cases to provide a hint.
	if trigger.Consumer.Expr == nil {
		panic(fmt.Sprintf("consume trigger %v has a nil Expr", trigger.Consumer))
	}

	var controller, producerSite, consumerSite inference.PrimitiveSite
	validProducer, validConsumer, paramProducer, returnConsumer := false, false, false, false
	if trigger.Controlled() {
		// trigger.Controller is of *CallSiteParamAnnotationKey type, which is enclosed in either
		// ArgPass or FuncParam, both with Kind() == Conditional. Thus, we can safely use false
		// here.
		controller = primitivizer.site(trigger.Controller, false)
	}
	prod, cons := trigger.Producer.Annotation, trigger.Consumer.Annotation
	pKind, cKind := prod.Kind(), cons.Kind()
	pSite, cSite := prod.UnderlyingSite(), cons.UnderlyingSite()
	if pKind == annotation.Conditional || pKind == annotation.DeepConditional {
		validProducer = true
		if pSite == nil {
			panic("trigger is conditional but the underlying site is nil")
		}
		producerSite = primitivizer.site(pSite, pKind == annotation.DeepConditional)
		_, paramProducer = prod.(annotation.FuncParam)
	}
	if cKind == annotation.Conditional || cKind == annotation.DeepConditional {
		validConsumer = true
		if cSite == nil {
			panic("trigger is conditional but the underlying site is nil")
		}
		consumerSite = primitivizer.site(cSite, cKind == annotation.DeepConditional)
		_, returnConsumer = cons.(annotation.UseAsReturn)
	}
	// At least one if branches above should have been executed, in other words, validProducer ||
	// validConsumer should be always true. This is because either producer is a FuncParam, or
	// consumer is a UseAsReturn, or both hold. See
	// assertion.function.duplicateFullTriggersFromContractedFunctionsToCallers for why.
	return &inference.Implication{
		Producer:         producerSite,
		Consumer:         consumerSite,
		Assertion:        primitivizer.fullTrigger(trigger),
		Controller:       controller,
		Controlled:       trigger.Controlled(),
		IsProducerValid:  validProducer,
		IsConsumerValid:  validConsumer,
		HasParamProducer: paramProducer,
		HasRetConsumer:   returnConsumer,
		CtrtParamIndex:   ctrtParamIndex,
		CtrtRetIndex:     ctrtRetIndex,
	}
}

type fileInfo struct {
	file    *token.File
	isLocal bool
}
