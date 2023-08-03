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
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
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
}

// Analyzer here is the analyzer than generates assertions and passes them onto the accumulator to
// be matched against annotations
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_function_analyzer",
	Doc:        _doc,
	Run:        run,
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
}

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
			// TODO: enable struct initialization flag.
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
	for r := range funcChan {
		if r.err != nil {
			errs = append(errs, r.err)
		} else {
			funcTriggers[r.index] = r.triggers
			triggerCount += len(r.triggers)
		}
	}

	// Flatten the triggers
	triggers := make([]annotation.FullTrigger, 0, triggerCount)
	for _, s := range funcTriggers {
		triggers = append(triggers, s...)
	}

	return Result{FullTriggers: triggers, Errors: errs}, nil
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
			funcChan <- functionResult{err: e, index: index}
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
	}
}
