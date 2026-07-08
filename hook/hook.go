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

// Package hook implements a hook framework for NilAway where it hooks into different parts to
// provide additional context for certain function calls. This is useful for well-known standard
// or 3rd party libraries where we can encode certain knowledge about them (e.g.,
// `assert.Nil(t, x)` implies `x == nil`) and use that to provide better analysis.
package hook

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"

	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/typeshelper"
)

// trustedKind indicates the kind of the trusted entity:
// (1) _method: it is a method of a struct;
// (2) _func: it is a top-level function of a package;
// (3) _var: it is a package-level variable.
type trustedKind uint8

const (
	_method trustedKind = iota
	_func
	_var
)

// trustedSig defines the signature of a function, method, or package-level variable that we "trust"
// to have a certain known effect or nilability.
type trustedSig struct {
	kind           trustedKind
	enclosingRegex *regexp.Regexp
	nameRegex      *regexp.Regexp
}

// matchCall checks if a given call expression invokes a trusted function or method (i.e., a
// signature of kind _func or _method). It performs a strict match on the called name and a regex
// match on the enclosing path:
//   - _func:   match enclosing "<pkg path>". E.g., for `assert.Error(err)`, path = github.com/stretchr/testify/assert
//   - _method: match "<pkg path>.<struct name>". E.g., for `u.Require().Error(err)`, path = github.com/stretchr/testify/require.Assertions
//
// Trusted package-level variables (kind _var) are matched separately via matchSel, since they are
// read as bare selectors rather than calls.
func (t *trustedSig) matchCall(pass *analysishelper.EnhancedPass, call *ast.CallExpr) bool {
	if t.kind != _func && t.kind != _method {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !t.nameRegex.MatchString(sel.Sel.Name) {
		return false
	}

	funcObj, ok := pass.TypesInfo.ObjectOf(sel.Sel).(*types.Func)
	if !ok || funcObj.Pkg() == nil {
		return false
	}
	recv := funcObj.Type().(*types.Signature).Recv()
	path := funcObj.Pkg().Path()

	// return early if the kind of `t` and `funcObj` don't match. Both should be functions (or methods) for the match to be performed
	// `recv != nil` implies `funcObj` is a method, while `recv == nil` means it is a function
	if (t.kind == _func && recv != nil) || (t.kind == _method && recv == nil) {
		return false
	}

	// add struct name to the path
	if recv != nil {
		if n, ok := typeshelper.UnwrapPtr(recv.Type()).(*types.Named); ok {
			path = path + "." + n.Obj().Name()
		} else {
			// we should likely never hit this case, but is only added for extra safety since
			// `util.TypeAsDeeplyNamed` can return nil
			return false
		}
	}
	return t.enclosingRegex.MatchString(path)
}

// matchSel checks if a given selector expression reads a trusted package-level variable (i.e., a
// signature of kind _var). E.g., for `os.Stdout`, path = os. It requires the object to be a genuine
// package-level variable (its enclosing scope is the package scope), ruling out struct fields and
// locals so that, e.g., a `Stdout` field on some local struct does not match.
//
// This is intentionally independent of how the variable is later used: a read like `os.Stdout`,
// `os.Stdout.Write(...)` (as a method receiver), or `os.Args[0]` (as an index operand) all parse the
// bare selector as a producer, so all are covered here without involving matchCall.
func (t *trustedSig) matchSel(pass *analysishelper.EnhancedPass, sel *ast.SelectorExpr) bool {
	if t.kind != _var || !t.nameRegex.MatchString(sel.Sel.Name) {
		return false
	}
	varObj, ok := pass.TypesInfo.ObjectOf(sel.Sel).(*types.Var)
	if !ok || varObj.Pkg() == nil || varObj.Parent() != varObj.Pkg().Scope() {
		return false
	}
	return t.enclosingRegex.MatchString(varObj.Pkg().Path())
}

// newNilBinaryExpr creates a new binary expression "expr op nil".
func newNilBinaryExpr(expr ast.Expr, op token.Token) *ast.BinaryExpr {
	return &ast.BinaryExpr{
		X:     expr,
		OpPos: expr.Pos(),
		Op:    op,
		Y: &ast.Ident{
			NamePos: expr.Pos(),
			Name:    "nil",
		},
	}
}
