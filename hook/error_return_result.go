//  Copyright (c) 2026 Uber Technologies, Inc.
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

package hook

import (
	"go/ast"
	"regexp"

	"go.uber.org/nilaway/util/analysishelper"
)

// ErrorReturnNonnilResult inspects a call to a trusted function whose `error` return, when checked
// to be nil, guarantees that one of its result values is non-nil. It returns the index of that
// result, or -1 if the call does not match any known contract. For example, for
// `(*http.Client).Do` it returns 0, modeling the documented contract [1] that the returned
// `*http.Response` is non-nil whenever the returned error is nil.
//
// This is intentionally a different hook category from ErrorReturnNonnilArg (which guards an
// argument pointee) and from AssumeReturn (which unconditionally marks a result non-nil). Here the
// non-nil guarantee is *conditional* on the error return being nil, so the caller feeds it as a
// RichCheckEffect to the FuncErrRet path (see
// assertion/function/assertiontree/rich_check_effect.go): the i-th result is produced non-nil in
// the branch where the error is checked to be nil, exactly as an explicit `result != nil` check
// would.
//
// [1]: https://pkg.go.dev/net/http#Client.Do
func ErrorReturnNonnilResult(pass *analysishelper.EnhancedPass, call *ast.CallExpr) int {
	for sig, idx := range _errorReturnNonnilResults {
		if sig.matchCall(pass, call) {
			return idx
		}
	}
	return -1
}

// _errorReturnNonnilResults defines the map of trusted functions and the index of the result that
// is non-nil when the function's error return is nil.
//
// Every function registered here must return `(T, ..., error)` where the result at the given index
// is a pointer/named type that is otherwise nilable, and the documented contract guarantees
// `result != nil` when `error == nil`. This keeps the hook minimal; richer multi-result contracts
// are out of scope until concrete evidence requires them.
var _errorReturnNonnilResults = map[trustedSig]int{
	// `(*net/http.Client).Do` returns `(*http.Response, error)`. Per the package docs [1]: "If the
	// returned error is nil, the Response will contain a non-nil Body", and a non-nil Response
	// paired with a non-nil error only arises from a CheckRedirect failure. It follows that when
	// the error return is nil, the Response result itself is non-nil. We model exactly that
	// conditional half: when the error return is checked to be nil, result 0 (the response) is
	// produced non-nil. We intentionally do NOT model the response as unconditionally non-nil,
	// since the error path may also yield a non-nil response.
	//
	// [1]: https://pkg.go.dev/net/http#Client.Do
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?net/http\.Client$`),
		nameRegex:      regexp.MustCompile(`^Do$`),
	}: 0,
}
