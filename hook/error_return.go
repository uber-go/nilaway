//  Copyright (c) 2025 Uber Technologies, Inc.
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
	"go/token"
	"regexp"

	"go.uber.org/nilaway/util/analysishelper"
)

// ErrorReturnNonnilArg inspects a call to a trusted function whose `error` return, when checked to
// be nil, guarantees that one of its (pointer) arguments is non-nil. It returns the "pointee"
// expression (the `x` in an `&x` argument) that should be guarded as non-nil once the error return
// is verified to be nil. For example, for `json.Unmarshal(data, &v)` it returns `v`, modeling the
// fact that `v` is non-nil after a successful unmarshal (i.e., `err == nil`).
//
// This is intentionally a different hook category from ReplaceConditional (used for `errors.As`).
// `errors.As(err, &target)` returns a `bool` that is consumed *directly* in the conditional, so it
// can be rewritten in place as `errors.As(...) && target != nil`. The trusted functions handled
// here instead return an `error` that is checked *separately* from the call (e.g.,
// `err := json.Unmarshal(...); if err != nil { ... }`). That check-then-effect pattern is modeled by
// the FuncErrRet rich-check-effect (see assertion/function/assertiontree/rich_check_effect.go),
// which this hook feeds: the returned expression is guarded as non-nil in the branch where the
// error is checked to be nil, just like a function's own return values are.
//
// If the call does not match any known function, nil is returned.
func ErrorReturnNonnilArg(pass *analysishelper.EnhancedPass, call *ast.CallExpr) ast.Expr {
	for sig, act := range _errorReturnNonnilArgs {
		if sig.matchCall(pass, call) {
			return act.action(call, act.argIndex)
		}
	}
	return nil
}

// errorReturnNonnilArgsAction computes the pointee expression guaranteed to be non-nil when the
// matched call's error return is nil, given the index of the relevant argument.
type errorReturnNonnilArgsAction func(call *ast.CallExpr, argIndex int) ast.Expr

// pointeeOfArg extracts the pointee expression `x` from an `&x` argument at the given index. This
// models functions that populate an out-parameter passed by address (e.g., `json.Unmarshal(data,
// &v)`), where a nil error return implies the pointee `x` is non-nil. If the argument is not of the
// `&x` form, nil is returned (we make no claim about it).
func pointeeOfArg(call *ast.CallExpr, argIndex int) ast.Expr {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	unaryExpr, ok := call.Args[argIndex].(*ast.UnaryExpr)
	if !ok || unaryExpr.Op != token.AND {
		return nil
	}
	return unaryExpr.X
}

// _errorReturnNonnilArgs defines the map of trusted functions and their corresponding actions on a
// particular argument.
var _errorReturnNonnilArgs = map[trustedSig]struct {
	action   errorReturnNonnilArgsAction
	argIndex int
}{
	// `encoding/json.Unmarshal(data, &v)` and `encoding/xml.Unmarshal(data, &v)` populate `v`, so a
	// nil error return implies `v != nil`.
	//
	// Note that this is technically unsound in a rare edge case: unmarshaling the JSON literal
	// `null` into a pointer leaves it nil while still returning a nil error [1]. In practice this is
	// rare enough that the convention is to treat the destination as populated after a successful
	// unmarshal.
	//
	// [1] https://pkg.go.dev/encoding/json#Unmarshal
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^encoding/(json|xml)$`),
		nameRegex:      regexp.MustCompile(`^Unmarshal$`),
	}: {action: pointeeOfArg, argIndex: 1},

	// `(cadence).Future.Get(ctx, &v)` blocks until the future is ready and, on success (a nil error
	// return), populates the value pointed to by `&v`, so a nil error return implies `v != nil`. The
	// `Future` interface lives in `go.uber.org/cadence/internal` and is re-exported (via a type alias)
	// as `go.uber.org/cadence/workflow.Future`, so the method's declaring package is the internal one.
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?go\.uber\.org/cadence/internal\.Future$`),
		nameRegex:      regexp.MustCompile(`^Get$`),
	}: {action: pointeeOfArg, argIndex: 1},
}
