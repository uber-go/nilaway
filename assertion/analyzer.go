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

// Package assertion implements a sub-analyzer that collects full triggers from the sub-analyzers
// and combine them into a list of full triggers for the entire package.
package assertion

import (
	"errors"
	"reflect"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion/affiliation"
	"go.uber.org/nilaway/assertion/function"
	"go.uber.org/nilaway/assertion/global"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Build the trees of assertions for each function in this package, propagating them to " +
	"entry and then matching them with possible sources of production to create a list of triggers " +
	"that can then be matched against a set of annotations to generate nil flow errors"

// Analyzer here is the analyzer than generates assertions and passes them onto the accumulator to
// be matched against annotations
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_assertion_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[[]annotation.FullTrigger])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer, function.Analyzer, affiliation.Analyzer, global.Analyzer},
}

func run(pass *analysis.Pass) ([]annotation.FullTrigger, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return nil, nil
	}

	// Collect and merge the results from sub-analyzers.
	r1 := pass.ResultOf[function.Analyzer].(*analysishelper.Result[[]annotation.FullTrigger])
	r2 := pass.ResultOf[affiliation.Analyzer].(*analysishelper.Result[[]annotation.FullTrigger])
	r3 := pass.ResultOf[global.Analyzer].(*analysishelper.Result[[]annotation.FullTrigger])
	if err := errors.Join(r1.Err, r2.Err, r3.Err); err != nil {
		return nil, err
	}

	// Merge full triggers.
	triggers := make([]annotation.FullTrigger, 0, len(r1.Res)+len(r2.Res)+len(r3.Res))
	for _, t := range [...][]annotation.FullTrigger{r1.Res, r2.Res, r3.Res} {
		triggers = append(triggers, t...)
	}

	return triggers, nil
}
