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

package affiliation

import (
	"reflect"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Track interface declarations and their implementations by identifying sites of type casts of interface " +
	"into a concrete type (e.g., var i I = &A{}, where I is an interface and A is a struct implementing I). Generate " +
	"potential triggers for flagging covariance and contravariance errors for return types and parameter types, respectively."

// Analyzer here is the analyzer that tracks interface implementations and analyzes for nilability
// variance, and passes them onto the accumulator to be added to existing assertions to be matched
// against annotations.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_affiliation_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	FactTypes:  []analysis.Fact{new(AffliliationCache)},
	ResultType: reflect.TypeOf((*analysishelper.Result[[]annotation.FullTrigger])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

func run(pass *analysis.Pass) ([]annotation.FullTrigger, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return nil, nil
	}

	a := &Affiliation{conf: conf}
	a.extractAffiliations(pass)
	return a.triggers, nil
}
