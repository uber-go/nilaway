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
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// astExprToString converts AST expression to string using the `printer` package
func astExprToString(e ast.Expr, pass *analysis.Pass) string {
	var buf bytes.Buffer
	err := printer.Fprint(&buf, pass.Fset, e)
	if err != nil {
		panic(fmt.Sprintf("Failed to convert AST expression to string: %v\n", err))
	}
	return buf.String()
}

// PrintExpr converts AST expression to string, and shortens long expressions if isShortenExpr is true
func PrintExpr(e ast.Expr, pass *analysis.Pass, isShortenExpr bool) string {
	if !isShortenExpr {
		astExprToString(e, pass)
	}

	// traverse over the AST expression's subtree and shorten long expressions (e.g., s.foo(longVarName, anotherLongVarName, someOtherLongVarName) --> s.foo(...))
	s := strings.Builder{}
	printExprHelper(e, pass, &s)

	return s.String()
}

func printExprHelper(e ast.Expr, pass *analysis.Pass, s *strings.Builder) {
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
		s.WriteString(node.Name)

	case *ast.SelectorExpr:
		printExprHelper(node.X, pass, s)
		s.WriteString(".")
		s.WriteString(node.Sel.Name)

	case *ast.CallExpr:
		printExprHelper(node.Fun, pass, s)
		s.WriteString("(")
		if len(node.Args) > 0 {
			isShorten := true
			if len(node.Args) == 1 {
				if arg, ok := fullExpr(node.Args[0]); ok {
					s.WriteString(arg)
					isShorten = false
				}
			}
			if isShorten {
				s.WriteString("...")
			}
		}
		s.WriteString(")")

	case *ast.IndexExpr:
		printExprHelper(node.X, pass, s)
		s.WriteString("[")
		if v, ok := fullExpr(node.Index); ok {
			s.WriteString(v)
		} else {
			s.WriteString("...")
		}
		s.WriteString("]")

	default:
		s.WriteString(astExprToString(e, pass))
	}
}
