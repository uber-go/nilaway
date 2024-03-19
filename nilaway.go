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

// Package nilaway implements the top-level analyzer that simply retrieves the diagnostics from
// the accumulation analyzer and reports them.
package nilaway

import (
	"go.uber.org/nilaway/accumulation"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Run NilAway on this package to report any possible flows of nil values to erroneous" +
	" sites that our system can detect"

// Analyzer is the top-level instance of Analyzer - it coordinates the entire dataflow to report
// nil flow errors in this package. It is needed here for nogo to recognize the package.
var Analyzer = &analysis.Analyzer{
	Name:      "nilaway",
	Doc:       _doc,
	Run:       run,
	FactTypes: []analysis.Fact{},
	Requires:  []*analysis.Analyzer{config.Analyzer, accumulation.Analyzer},
}

func run(pass *analysis.Pass) (interface{}, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)
	deferredErrors := pass.ResultOf[accumulation.Analyzer].([]analysis.Diagnostic)
	for _, e := range deferredErrors {
		if conf.PrettyPrint {
			e.Message = util.PrettyPrintErrorMessage(e.Message)
		}
		pass.Report(e)
	}

	return nil, nil
}
