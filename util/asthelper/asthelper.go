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
	"io"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// PrintExpr converts AST expression to string, and shortens long expressions if isShortenExpr is true
func PrintExpr(e ast.Expr, pass *analysis.Pass, isShortenExpr bool) (string, error) {
	builder := &strings.Builder{}
	var err error

	if !isShortenExpr {
		err = printer.Fprint(builder, pass.Fset, e)
	} else {
		// traverse over the AST expression's subtree and shorten long expressions
		// (e.g., s.foo(longVarName, anotherLongVarName, someOtherLongVarName) --> s.foo(...))
		err = printExpr(builder, pass.Fset, e)
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
		_, err = writer.Write([]byte(node.Name))

	case *ast.SelectorExpr:
		if err = printExpr(writer, fset, node.X); err != nil {
			return
		}
		output := []byte{'.'}
		output = append(output, node.Sel.Name...)
		_, err = writer.Write(output)

	case *ast.CallExpr:
		if err = printExpr(writer, fset, node.Fun); err != nil {
			return
		}
		output := make([]byte, 0, 5)
		output = append(output, '(')
		if len(node.Args) > 0 {
			isShorten := true
			if len(node.Args) == 1 {
				if arg, ok := fullExpr(node.Args[0]); ok {
					output = append(output, arg...)
					isShorten = false
				}
			}
			if isShorten {
				output = append(output, '.', '.', '.') // ellipsis
			}
		}
		output = append(output, ')')
		_, err = writer.Write(output)

	case *ast.IndexExpr:
		if err = printExpr(writer, fset, node.X); err != nil {
			return
		}
		output := make([]byte, 0, 5)
		output = append(output, '[')
		if v, ok := fullExpr(node.Index); ok {
			output = append(output, v...)
		} else {
			output = append(output, '.', '.', '.') // ellipsis
		}
		output = append(output, ']')
		_, err = writer.Write(output)

	default:
		err = printer.Fprint(writer, fset, e)
	}
	return
}
