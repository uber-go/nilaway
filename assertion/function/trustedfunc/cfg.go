//  Copyright (c) 2024 Uber Technologies, Inc.
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

package trustedfunc

import (
	"go/ast"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

var _cfgTrimSuccs = map[trustedFuncSig]bool{
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^log$`),
		funcNameRegex:  regexp.MustCompile(`^Fatal(f)?$`),
	}: true,
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^os$`),
		funcNameRegex:  regexp.MustCompile(`^Exit$`),
	}: true,
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^runtime$`),
		funcNameRegex:  regexp.MustCompile(`^Goexit$`),
	}: true,
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^testing.(T|B|F|TB)$`),
		funcNameRegex:  regexp.MustCompile(`^(FailNow|Skip|Skipf|SkipNow)$`),
	}: true,
}

// TrimSuccsOn returns true if the call should be treated as a no-return call and hence should have
// the `Succs` field trimmed on its residing CFG block.
//
// Note that this does not really need to be done for most analyzer drivers, as the CFG
// construction (via the [ctrlflow] analyzer) already handles this via the `noReturn` facts (by
// analyzing the sources of stdlib). However, in bazel/nogo, the stdlib is installed via
// `go install`, and hence nogo analyzers do not have access to the stdlib sources, leading to no
// `noReturn` facts being generated for stdlib. Hence, this hook is a workaround to handle
// no-return functions only in stdlib for bazel/nogo. It would be redundant (but harmless) for
// other drivers.
// [ctrlflow]: https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/ctrlflow
func TrimSuccsOn(pass *analysis.Pass, call *ast.CallExpr) bool {
	for f := range _cfgTrimSuccs {
		if f.match(call, pass) {
			return true
		}
	}
	return false
}
