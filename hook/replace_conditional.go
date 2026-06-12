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

package hook

import (
	"go/ast"
	"go/token"
	"regexp"

	"go.uber.org/nilaway/util/analysishelper"
)

// ReplaceConditional replaces a call to a matched function with the returned expression. This is
// useful for modeling stdlib and 3rd party functions that return a single boolean value, which
// implies nilability of the arguments. For example, `errors.As(err, &target)` implies
// `target != nil`, so it can be replaced with `target != nil`.
//
// If the call does not match any known function, nil is returned.
func ReplaceConditional(pass *analysishelper.EnhancedPass, call *ast.CallExpr) ast.Expr {
	for sig, act := range _replaceConditionals {
		if sig.matchCall(pass, call) {
			return act(pass, call)
		}
	}
	return nil
}

type replaceConditionalAction func(pass *analysishelper.EnhancedPass, call *ast.CallExpr) ast.Expr

// _errorAsAction replaces a call to `errors.As(err, &target)` with an equivalent expression
// `errors.As(err, &target) && target != nil`. Keeping the `errors.As(err, &target)` is important
// since `err` may contain complex expressions that may have nilness issues.
//
// Note that technically `target` can still be nil even if `errors.As(err, &target)` is true. For
// example, if err is a typed nil (e.g., `var err *exec.ExitError`), then `errors.As` would
// actually find a match, but `target` would be set to the typed nil value, resulting in a `nil`
// target. However, in practice this should rarely happen such that even the official documentation
// assumes the target is non-nil after such check [1]. So here we make this assumption as well.
//
// [1] https://pkg.go.dev/errors#As
var _errorAsAction replaceConditionalAction = func(_ *analysishelper.EnhancedPass, call *ast.CallExpr) ast.Expr {
	if len(call.Args) != 2 {
		return nil
	}
	unaryExpr, ok := call.Args[1].(*ast.UnaryExpr)
	if !ok {
		return nil
	}
	if unaryExpr.Op != token.AND {
		return nil
	}
	return &ast.BinaryExpr{
		X:     call,
		Op:    token.LAND,
		OpPos: call.Pos(),
		Y:     newNilBinaryExpr(unaryExpr.X, token.NEQ),
	}
}

// _assertConditionalAction replaces bool-returning testify assertions used as conditionals, e.g.,
// `if assert.NoError(t, err) {...}`, with `<call> && <implied expr>` (here:
// `assert.NoError(t, err) && err == nil`). The implied expression is the same one `_splitBlockOn`
// derives for the call in statement position, so the per-assertion semantics (including the
// asserted argument's position) are defined only there. The assertion returns true iff it passed,
// so the implied expression holds in the then-branch; the else-branch gains no information from
// the conjunction, which is conservative. Note that, unlike the statement-position modeling in
// `_splitBlockOn`, no assumption about test termination is involved, so this is sound for the
// non-fatal `assert` package as well.
var _assertConditionalAction replaceConditionalAction = func(pass *analysishelper.EnhancedPass, call *ast.CallExpr) ast.Expr {
	implied := SplitBlockOn(pass, call)
	if implied == nil {
		return nil
	}
	return &ast.BinaryExpr{
		X:     call,
		Op:    token.LAND,
		OpPos: call.Pos(),
		Y:     implied,
	}
}

var _replaceConditionals = map[trustedSig]replaceConditionalAction{
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^errors$`),
		nameRegex:      regexp.MustCompile(`^As$`),
	}: _errorAsAction,
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/cockroachdb/errors$`),
		nameRegex:      regexp.MustCompile(`^As$`),
	}: _errorAsAction,

	// Bool-returning testify assertions used as conditionals. `require` is absent since its
	// functions do not return values and hence cannot appear in a conditional.
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/assert$`),
		nameRegex:      regexp.MustCompile(`^(Nil(f)?|NotNil(f)?|NoError(f)?|Error(f)?|ErrorContains(f)?|EqualError(f)?|True(f)?|False(f)?)$`),
	}: _assertConditionalAction,
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions)$`),
		nameRegex:      regexp.MustCompile(`^(Nil(f)?|NotNil(f)?|NoError(f)?|Error(f)?|ErrorContains(f)?|EqualError(f)?|True(f)?|False(f)?)$`),
	}: _assertConditionalAction,
}
