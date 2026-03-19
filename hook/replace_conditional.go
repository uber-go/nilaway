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
		if sig.match(pass, call) {
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

// _jsonUnmarshalAction replaces a call to `json.Unmarshal(data, &v)` or `xml.Unmarshal(data, &v)` with
// `json.Unmarshal(data, &v) && v != nil`. This handles the contract where these functions allocate
// maps/slices when they're nil, so when the error is nil, the output parameter is non-nil.
// This is similar to errors.As in concept - we keep the call expression and add an implicit nil check.
var _jsonUnmarshalAction replaceConditionalAction = func(_ *analysishelper.EnhancedPass, call *ast.CallExpr) ast.Expr {
	if len(call.Args) != 2 {
		return nil
	}

	unaryExpr, ok := call.Args[1].(*ast.UnaryExpr)
	if !ok || unaryExpr.Op != token.AND {
		return nil
	}

	return &ast.BinaryExpr{
		X:     call,
		Op:    token.LAND,
		OpPos: call.Pos(),
		Y:     newNilBinaryExpr(unaryExpr.X, token.NEQ),
	}
}

var _replaceConditionals = map[trustedFuncSig]replaceConditionalAction{
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^errors$`),
		funcNameRegex:  regexp.MustCompile(`^As$`),
	}: _errorAsAction,

	// `encoding/json.Unmarshal` and `encoding/xml.Unmarshal`
	// When error is nil, the output parameter (second argument, a pointer to map/slice) is guaranteed to be non-nil
	// since these functions allocate maps/slices when they're nil.
	// Similar to errors.As, we keep the original call expression and add an implicit nil check.
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(encoding/json|encoding/xml)$`),
		funcNameRegex:  regexp.MustCompile(`^Unmarshal$`),
	}: _jsonUnmarshalAction,
}
