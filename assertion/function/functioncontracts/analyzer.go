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

// Package functioncontracts implements a sub-analyzer to analyze function contracts in a package,
// i.e., parsing specified function contracts written as special comments before function
// declarations, or automatically inferring function contracts from the function body.
package functioncontracts

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"runtime/debug"
	"sync"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const _doc = "Read the contracts of each function in this package, returning the results."

// Analyzer here is the analyzer than reads function contracts. It returns the map generated from
// reading the function contracts in the source code.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_function_contracts_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[Map])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer, buildssa.Analyzer},
}

func run(pass *analysis.Pass) (Map, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return Map{}, nil
	}

	contracts, err := collectFunctionContracts(pass)
	if err != nil {
		return nil, err
	}
	return contracts, nil
}

// functionResult is the struct that is received from the channel for each function.
type functionResult struct {
	funcObj   *types.Func
	contracts []*Contract
	err       error
}

// collectFunctionContracts collects all the function contracts and returns a map that associates
// every function with its contracts if it has any. We prefer to parse handwritten contracts from
// the comments at the top of each function. Only when there are no handwritten contracts there,
// do we try to automatically infer contracts.
func collectFunctionContracts(pass *analysis.Pass) (Map, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	// Collect ssa for every function.
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	ssaOfFunc := make(map[*types.Func]*ssa.Function, len(ssaInput.SrcFuncs))
	for _, fnssa := range ssaInput.SrcFuncs {
		if fnssa == nil {
			// should be guaranteed to be non-nil; otherwise it would have paniced in the library
			// https://cs.opensource.google/go/x/tools/+/refs/tags/v0.12.0:go/analysis/passes/buildssa/buildssa.go;l=99
			continue
		}
		if funcObj, ok := fnssa.Object().(*types.Func); ok {
			ssaOfFunc[funcObj] = fnssa
		}
	}

	// Set up variables for synchronization and communication.
	var wg sync.WaitGroup
	funcChan := make(chan functionResult)

	m := Map{}
	for _, file := range pass.Files {
		if !conf.IsFileInScope(file) {
			continue
		}
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				// Ignore any non-function declaration
				// TODO: If we want to support contracts for anonymous functions (function
				//  literals) in the future, then we need to handle more types here.
				continue
			}
			funcObj := pass.TypesInfo.ObjectOf(funcDecl.Name).(*types.Func)

			// First, we try to parse the contracts from the comments at the top of the function.
			// If there are any, we do not need to infer contracts for this function.
			if parsedContracts := parseContracts(funcDecl.Doc); len(parsedContracts) != 0 {
				m[funcObj] = parsedContracts
				continue
			}

			// If we reach here, it means that there are no handwritten contracts for this
			// function. We need to infer contracts for this function.
			if funcDecl.Type.Params.NumFields() != 1 ||
				funcDecl.Type.Results.NumFields() != 1 ||
				util.TypeBarsNilness(funcObj.Type().(*types.Signature).Params().At(0).Type()) ||
				util.TypeBarsNilness(funcObj.Type().(*types.Signature).Results().At(0).Type()) ||
				funcObj.Type().(*types.Signature).Variadic() {
				// We definitely want to ignore any function without any parameters or return
				// values since they cannot have any contracts.

				// TODO: However, we want to analyze for multiple param/return in the future; for
				//  now we consider contract(nonnil->nonnil) only.

				// TODO: If the function has only one parameter and the parameter is variadic, then
				//  it may happen that no argument is passed when calling the function. Such cases
				//  are not handled well when duplicating full triggers from contracted functions,
				//  so we don't infer contract(nonnil->nonnil) for such a function although we can
				//  already.
				continue
			}
			fnssa, ok := ssaOfFunc[funcObj]
			if !ok {
				// For some reason, we cannot find the ssa for this function. We ignore this
				// function.
				continue
			}
			if len(fnssa.Blocks) == 0 {
				// For external functions whose function bodies are defined outside Go (e.g.,
				// assembly), we do not actually have Go source code for them, and there will be no
				// blocks (see the documentation of the ssa package). Therefore, we ignore such functions.
				continue
			}

			// Infer contracts for a function that does not have any contracts specified.
			wg.Add(1)
			go func() {
				defer wg.Done()

				// As a last resort, convert the panics into errors and return.
				defer func() {
					if r := recover(); r != nil {
						e := fmt.Errorf("INTERNAL PANIC: %s\n%s", r, string(debug.Stack()))
						funcChan <- functionResult{err: e, funcObj: funcObj, contracts: []*Contract{}}
					}
				}()

				if contracts := inferContracts(fnssa); len(contracts) != 0 {
					funcChan <- functionResult{
						funcObj:   funcObj,
						contracts: contracts,
					}
				}
			}()
		}
	}

	// Spawn another goroutine that will close the channel when all analyses are done. This makes
	// sure the channel receive logic in the main thread (below) can properly terminate.
	go func() {
		wg.Wait()
		close(funcChan)
	}()

	// Collect inferred contracts from the channel.
	var err error
	for r := range funcChan {
		m[r.funcObj] = r.contracts
		err = errors.Join(err, r.err)
	}

	return m, err
}
