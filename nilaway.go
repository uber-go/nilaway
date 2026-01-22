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
	"fmt"
	"regexp"

	"go.uber.org/nilaway/accumulation"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
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

func run(p *analysis.Pass) (interface{}, error) {
	pass := analysishelper.NewEnhancedPass(p)
	conf := pass.ResultOf[config.Analyzer].(*config.Config)
	deferredErrors := pass.ResultOf[accumulation.Analyzer].([]analysis.Diagnostic)
	for _, e := range deferredErrors {
		if conf.PrettyPrint {
			e.Message = prettyPrintErrorMessage(e.Message)
		}
		pass.Report(e)
	}

	return nil, nil
}

var codeReferencePattern = regexp.MustCompile("\\`(.*?)\\`")
var pathPattern = regexp.MustCompile(`"(.*?)"`)
var nilabilityPattern = regexp.MustCompile(`([\(|^\t](?i)(found\s|must\sbe\s)(nilable|nonnil)[\)]?)`)

// prettyPrintErrorMessage is used in error reporting to post process and pretty print the output with colors
func prettyPrintErrorMessage(msg string) string {
	// TODO: below string parsing should not be required after  is implemented
	errorStr := fmt.Sprintf("\x1b[%dm%s\x1b[0m", 31, "error: ")      // red
	codeStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 95, "`${1}`")    // magenta
	pathStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 36, "${1}")      // cyan
	nilabilityStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 1, "${1}") // bold

	msg = nilabilityPattern.ReplaceAllString(msg, nilabilityStr)
	msg = codeReferencePattern.ReplaceAllString(msg, codeStr)
	msg = pathPattern.ReplaceAllString(msg, pathStr)
	msg = errorStr + msg
	return msg
}
