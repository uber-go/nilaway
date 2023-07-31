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
	"fmt"
	"reflect"
	"runtime/debug"

	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis"
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

// Analyzer here is the analyzer than reads function contracts
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_function_contracts_analyzer",
	Doc:        _doc,
	Run:        run,
	ResultType: reflect.TypeOf((*Result)(nil)).Elem(),
	Requires:   []*analysis.Analyzer{config.Analyzer},
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

	return Result{FunctionContracts: collectFunctionContracts(pass)}, nil
}
