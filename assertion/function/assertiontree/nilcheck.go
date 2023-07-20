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

package assertiontree

import (
	"go/ast"
	"go/token"

	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/ast/astutil"
)

type nilCheckType uint8

const (
	_none nilCheckType = iota
	_positiveNilCheck
	_negativeNilCheck
)

// asNilCheckExpr takes an AST expression and determines if it is a nil check expression, such as `x != nil` or `x == nil`
// If true, it returns the non-literal operand of the expression (e.g., `x` of `x != nil`) and the type of nil check, i.e.,
// "positive" for `x == nil` and "negative" for `x != nil`
func asNilCheckExpr(expr ast.Expr) (ast.Expr, nilCheckType) {
	expr = astutil.Unparen(expr)

	if e, ok := expr.(*ast.BinaryExpr); ok && (e.Op == token.NEQ || e.Op == token.EQL) {
		t := _none
		if e.Op == token.NEQ {
			t = _negativeNilCheck
		} else if e.Op == token.EQL {
			t = _positiveNilCheck
		}

		// if expr is `v != nil` or `nil != v`, then return `v, <nilability type>`
		if util.IsLiteral(e.Y, "nil") {
			return e.X, t
		} else if util.IsLiteral(e.X, "nil") {
			return e.Y, t
		}
	}

	// If the unary expression encloses a nil check binary expression, then the below code negates the outcome produced by the recursive call to asNilCheckExpr.
	// For example, if`!(v == nil)`, then asNilCheckExpr on the inner expression `(v == nil)` returns `v, _positiveNilCheck`. But since it is preceded with a negation (!),
	// the below code negates the outcome, and returns `v, _negativeNilCheck`, implying a negative nil check.
	// Likewise, if `!(v != nil)`, then the below code correctly returns `v, _positiveNilCheck`, implying a positive nil check.
	if e, ok := expr.(*ast.UnaryExpr); ok && e.Op == token.NOT {
		if retExpr, retType := asNilCheckExpr(e.X); retType != _none {
			if retType == _positiveNilCheck {
				return retExpr, _negativeNilCheck
			}
			return retExpr, _positiveNilCheck
		}
	}

	// not a nil check expression
	return nil, _none
}
