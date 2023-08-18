//  Copyright (c) 2023 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software distributed under the
// License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing permissions and
// limitations under the License.

// Package functioncontracts implements a sub-analyzer to analyze function contracts in a package,
// i.e., parsing specified function contracts written as special comments before function
// declarations, or automatically inferring function contracts from the function body.
package functioncontracts

import (
	"context"
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"runtime/debug"
	"sync"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const _doc = "Read the contracts of each function in this package, returning the results."

// Result is the result struct for the Analyzer.
type Result struct {
	// FunctionContractsMap is the map generated from reading the function contracts in the source
	// code.
	FunctionContracts Map
	// Errors is the slice of errors if errors happened during analysis. We put the errors here as
	// part of the result of this sub-analyzer so that the upper-level analyzers can decide what
	// to do with them.
	Errors []error
}

// Analyzer here is the analyzer that reads function contracts.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_function_contracts_analyzer",
	Doc:        _doc,
	Run:        run,
	FactTypes:  []analysis.Fact{new(Cache)},
	ResultType: reflect.TypeOf((*Result)(nil)).Elem(),
	Requires:   []*analysis.Analyzer{buildssa.Analyzer, config.Analyzer},
}

// functionResult is the struct that is received from the channel for each function.
type functionResult struct {
	funcObj   *types.Func
	contracts []*FunctionContract
	err       error
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
				retResult.FunctionContracts = Map{}
				retResult.Errors = append(retResult.Errors, e)
			} else {
				result = Result{FunctionContracts: Map{}, Errors: []error{e}}
			}
		}
	}()

	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return Result{FunctionContracts: Map{}}, nil
	}

	return Result{FunctionContracts: collectUpstreamAndCurrent(pass)}, nil
}

// Cache stores the contracts collected from the current package. This information can be used by
// downstream packages to avoid re-collection from the same package.
type Cache struct {
	CurPkgContracts map[string][]*FunctionContract
}

// AFact enables use of the facts passing mechanism in Go's analysis framework
func (*Cache) AFact() {}

// collectUpstreamAndCurrent collects all the contracts from upstream packages and the current
// package, and also exports the contracts in the current package for downstream packages to use.
func collectUpstreamAndCurrent(pass *analysis.Pass) Map {
	// collect all contracts from upstream
	totalCtrts := map[string][]*FunctionContract{}
	// populate totalCtrts by importing contracts passed from upstream packages
	facts := pass.AllPackageFacts()
	if len(facts) > 0 {
		for _, f := range facts {
			switch c := f.Fact.(type) {
			case *Cache:
				for funcID, contracts := range c.CurPkgContracts {
					totalCtrts[funcID] = append(totalCtrts[funcID], contracts...)
				}
			}
		}
	}

	curPkgCtrts := collectFunctionContracts(pass)
	// export new contracts from this package; only necessary for those exportable functions.
	exported := map[string][]*FunctionContract{}
	for funcObj, contracts := range curPkgCtrts {
		if !funcObj.Exported() {
			// skip non-exported functions
			continue
		}
		funcID := funcObj.FullName()
		exported[funcID] = append(exported[funcID], contracts...)
	}
	if len(curPkgCtrts) > 0 {
		pass.ExportPackageFact(&Cache{CurPkgContracts: exported})
	}

	// include contracts in the current package
	for funcObj, contracts := range curPkgCtrts {
		totalCtrts[funcObj.FullName()] = contracts
	}
	return totalCtrts
}

// collectFunctionContracts collects all the function contracts and returns a map that associates
// every function with its contracts if it has any. We prefer to parse handwritten contracts from
// the comments at the top of each function. Only when there are no handwritten contracts there,
// do we try to automatically infer contracts.
func collectFunctionContracts(pass *analysis.Pass) map[*types.Func][]*FunctionContract {
	// Collect ssa for every function.
	conf := pass.ResultOf[config.Analyzer].(*config.Config)
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
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	funcChan := make(chan functionResult)

	m := map[*types.Func][]*FunctionContract{}
	for _, file := range pass.Files {
		if !conf.IsFileInScope(file) || !util.DocContainsFunctionContractsCheck(file.Doc) {
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
			if funcDecl.Type.Params.NumFields() == 0 ||
				funcDecl.Type.Results.NumFields() == 0 ||
				allParamOrRetTypesBarNilness(funcObj) ||
				funcObj.Type().(*types.Signature).Variadic() {
				// We ignore any function without any parameters or return values since they cannot
				// have any contracts.
				// We ignore any function that has a parameter or return value with a type that
				// cannot have nil as a valid value, e.g., an int.

				// TODO: We ignore variadic parameters since they are not well handled when
				//  creating triggers. We will need to create a Always Nilable producer if no
				//  argument is passed to a site that is supposed to be a variadic parameter.
				//  We leave this as future work.
				continue
			}
			fnssa, ok := ssaOfFunc[funcObj]
			if !ok {
				// For some reason, we cannot find the ssa for this function. We ignore this
				// function.
				continue
			}
			wg.Add(1)
			// Infer contracts for a function that does not have any contracts specified.
			go inferContractsToChannel(funcObj, fnssa, funcChan, &wg)
		}
	}

	// Spawn another goroutine that will close the channel when all analyses are done. This makes
	// sure the channel receive logic in the main thread (below) can properly terminate.
	go func() {
		wg.Wait()
		close(funcChan)
	}()

	// Collect inferred contracts from the channel.
	for r := range funcChan {
		if len(r.contracts) != 0 {
			ctrt := r.contracts[0]
			if ctrt == nil {
				continue
			}
			m[r.funcObj] = r.contracts
		}
	}
	return m
}

// allParamOrRetTypesBarNilness checks if all parameter or return values of a function have types
// that cannot have nil as a valid value, e.g., all ints.
func allParamOrRetTypesBarNilness(funcObj *types.Func) bool {
	params := funcObj.Type().(*types.Signature).Params()
	results := funcObj.Type().(*types.Signature).Results()
	return allInTupleTypesBarNilness(params) || allInTupleTypesBarNilness(results)
}

// allInTupleTypesBarNilness checks if all types in a tuple cannot have nil as a valid value, e.g.,
// all ints.
func allInTupleTypesBarNilness(vars *types.Tuple) bool {
	for i := 0; i < vars.Len(); i++ {
		if !util.TypeBarsNilness(vars.At(i).Type()) {
			return false
		}
	}
	return true
}

// inferContractsToChannel infers contracts for a function that does not have any contracts
// specified and sends the result to the channel.
func inferContractsToChannel(
	funcObj *types.Func,
	fnssa *ssa.Function,
	fnChan chan functionResult,
	wg *sync.WaitGroup,
) {
	// As a last resort, convert the panics into errors and return.
	defer func() {
		if r := recover(); r != nil {
			e := fmt.Errorf("INTERNAL PANIC: %s\n%s", r, string(debug.Stack()))
			fnChan <- functionResult{err: e, funcObj: funcObj, contracts: []*FunctionContract{}}
		}
	}()
	defer wg.Done()

	fnChan <- functionResult{
		funcObj:   funcObj,
		contracts: inferContracts(fnssa),
	}
}
