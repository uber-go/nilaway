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
	"reflect"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Read the contracts of each function in this package, returning the results."

// Analyzer here is the analyzer than reads function contracts. It returns the map generated from
// reading the function contracts in the source code.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_function_contracts_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[Map])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

func run(pass *analysis.Pass) (Map, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return Map{}, nil
	}

	return collectFunctionContracts(pass), nil
}
