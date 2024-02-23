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

package annotation

import (
	"reflect"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Read the annotations for each struct, interface, and function in this package, returning" +
	" the results so that they may be matched against assertions by an accumulator"

// Analyzer here is the analyzer than reads annotations and passes them onto the accumulator to
// be matched against assertions. It returns the map generated from reading the annotations in the
// source code
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_annotation_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[*ObservedMap])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

func run(pass *analysis.Pass) (*ObservedMap, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return new(ObservedMap), nil
	}

	return newObservedMap(pass, pass.Files), nil
}
