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

// Package asthelper implements utility functions for AST.
package asthelper

import (
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"strings"
)

// DocContains returns true if the comment group contains the given string.
func DocContains(file *ast.File, s string) bool {
	for _, comment := range file.Comments {
		// The comment group here contains all comments in the file. However, we should only check
		// the comments before the package name (e.g., `package Foo`) line.
		if comment.Pos() > file.Name.Pos() {
			return false
		}

		if strings.Contains(comment.Text(), s) {
			return true
		}
	}

	return false
}

// PrintExpr converts AST expression to string, and shortens long expressions if isShortenExpr is true
func PrintExpr(e ast.Expr, fset *token.FileSet, isShortenExpr bool) (string, error) {
	builder := &strings.Builder{}
	var err error

	if !isShortenExpr {
		err = printer.Fprint(builder, fset, e)
	} else {
		// traverse over the AST expression's subtree and shorten long expressions
		// (e.g., s.foo(longVarName, anotherLongVarName, someOtherLongVarName) --> s.foo(...))
		err = printExpr(builder, fset, e)
	}

	return builder.String(), err
}

func printExpr(writer io.Writer, fset *token.FileSet, e ast.Expr) (err error) {
	// _shortenExprLen is the maximum length of an expression to be printed in full. The value is set to 3 to account for
	// the length of the ellipsis ("..."), which is used to shorten long expressions.
	const _shortenExprLen = 3

	// fullExpr returns true if the expression is short enough (<= _shortenExprLen) to be printed in full
	fullExpr := func(node ast.Node) (string, bool) {
		switch n := node.(type) {
		case *ast.Ident:
			if len(n.Name) <= _shortenExprLen {
				return n.Name, true
			}
		case *ast.BasicLit:
			if len(n.Value) <= _shortenExprLen {
				return n.Value, true
			}
		}
		return "", false
	}

	switch node := e.(type) {
	case *ast.Ident:
		_, err = io.WriteString(writer, node.Name)

	case *ast.SelectorExpr:
		if err = printExpr(writer, fset, node.X); err != nil {
			return
		}
		_, err = io.WriteString(writer, "."+node.Sel.Name)

	case *ast.CallExpr:
		if err = printExpr(writer, fset, node.Fun); err != nil {
			return
		}
		var argStr string
		if len(node.Args) > 0 {
			argStr = "..."
			if len(node.Args) == 1 {
				if a, ok := fullExpr(node.Args[0]); ok {
					argStr = a
				}
			}
		}
		_, err = io.WriteString(writer, "("+argStr+")")

	case *ast.IndexExpr:
		if err = printExpr(writer, fset, node.X); err != nil {
			return
		}

		indexExpr := "..."
		if v, ok := fullExpr(node.Index); ok {
			indexExpr = v
		}
		_, err = io.WriteString(writer, "["+indexExpr+"]")

	default:
		err = printer.Fprint(writer, fset, e)
	}
	return
}

// ExtractLHSRHS extracts the left-hand side and right-hand side of an assignment statement or a variable declaration
func ExtractLHSRHS(node ast.Node) (lhs, rhs []ast.Expr) {
	switch expr := node.(type) {
	case *ast.AssignStmt:
		lhs, rhs = expr.Lhs, expr.Rhs
	case *ast.ValueSpec:
		for _, name := range expr.Names {
			lhs = append(lhs, name)
		}
		rhs = expr.Values
	}
	return
}

// IsLiteral returns true if `expr` is a literal that matches with one of the given literal values (e.g., "nil", "true", "false)
func IsLiteral(expr ast.Expr, literals ...string) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		for _, literal := range literals {
			if ident.Name == literal {
				return true
			}
		}
	}
	return false
}

// CallExprFromExpr returns the call expression from the given expression. It recursively
// traverses the expression tree to find the call expression. If the expression is not a
// call expression, it returns nil.
func CallExprFromExpr(expr ast.Expr) *ast.CallExpr {
	switch e := expr.(type) {
	case *ast.CallExpr:
		return e
	case *ast.ParenExpr:
		return CallExprFromExpr(e.X)
	case *ast.UnaryExpr:
		return CallExprFromExpr(e.X)
	case *ast.SelectorExpr:
		return CallExprFromExpr(e.X)
	}
	return nil
}

// GetSelectorExprHeadIdent gets the head of the chained selector expression if it is an ident. Returns nil otherwise
func GetSelectorExprHeadIdent(selExpr *ast.SelectorExpr) *ast.Ident {
	if ident, ok := selExpr.X.(*ast.Ident); ok {
		return ident
	}
	if x, ok := selExpr.X.(*ast.SelectorExpr); ok {
		return GetSelectorExprHeadIdent(x)
	}
	return nil
}

// IsFieldSelectorChain returns true if the expr is chain of idents. e.g, x.y.z
// It returns for false for expressions such as x.y().z
func IsFieldSelectorChain(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return IsFieldSelectorChain(expr.X)
	default:
		return false
	}
}

// IsEmptyExpr checks if an expression is the empty identifier
func IsEmptyExpr(expr ast.Expr) bool {
	if id, ok := expr.(*ast.Ident); ok {
		if id.Name == "_" {
			return true
		}
	}
	return false
}

// GetFieldVal returns the assigned value for the field at index. compElts holds the  elements of the composite literal expression
// for struct initialization
func GetFieldVal(compElts []ast.Expr, fieldName string, numFields int, index int) ast.Expr {
	for _, elt := range compElts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok {
				if key.Name == fieldName {
					return kv.Value
				}
			}
		}
	}

	// In this case the initialization is serial e.g. a = &A{p, q}
	if numFields == len(compElts) {
		return compElts[index]
	}
	return nil
}

// FuncIdentFromCallExpr return a function identified from a call expression, nil otherwise
// nilable(result 0)
func FuncIdentFromCallExpr(expr *ast.CallExpr) *ast.Ident {
	switch fun := expr.Fun.(type) {
	case *ast.Ident:
		return fun
	case *ast.SelectorExpr:
		return fun.Sel
	default:
		// case of anonymous function
		return nil
	}
}

// GetFunctionParamNode returns the ast param node matching the variable searchParam
func GetFunctionParamNode(funcDecl *ast.FuncDecl, searchParam *types.Var) ast.Expr {
	for _, params := range funcDecl.Type.Params.List {
		for _, param := range params.Names {
			if searchParam.Name() == param.Name && param.Name != "" && param.Name != "_" {
				return param
			}
		}
	}

	return nil
}
