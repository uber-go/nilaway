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

package analysishelper

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
)

func TestEnhancedPass_IsZero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{"zero literal", "0", true},
		{"non-zero literal", "1", false},
		{"negative zero", "-0", true},
		{"negative non-zero", "-1", false},
		{"binary expression evaluating to zero", "1 - 1", true},
		{"binary expression evaluating to non-zero", "1 + 1", false},
		{"complex binary expression evaluating to zero", "5 - 3 - 2", true},
		{"multiplication by zero", "42 * 0", true},
		{"zero string literal", `"0"`, false},
		{"zero float literal (IsZero only handles integers)", "0.0", false},
		{"parenthesized zero", "(0)", true},
		{"parenthesized expression evaluating to zero", "(2 - 2)", true},
		{"non-literal expression", "x", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a minimal Go program with the expression
			src := "package main\nfunc main() { var x int; print(x); _ = " + tt.code + " }"
			pass, file := newTestEnhancedPass(t, src)

			// Find the expression in the AST
			var expr ast.Expr
			ast.Inspect(file, func(n ast.Node) bool {
				if assign, ok := n.(*ast.AssignStmt); ok && len(assign.Rhs) > 0 {
					expr = assign.Rhs[0]
					return false
				}
				return true
			})
			require.NotNil(t, expr, "expression not found in AST")

			require.Equal(t, tt.expected, pass.IsZero(expr))
		})
	}
}

// newTestEnhancedPass creates an *analysishelper.EnhancedPass from the given Go source code for testing purposes.
func newTestEnhancedPass(t *testing.T, src string) (*EnhancedPass, *ast.File) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	require.NoError(t, err)

	conf := types.Config{}
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	_, err = conf.Check("test", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	pass := &analysis.Pass{TypesInfo: info}
	return NewEnhancedPass(pass), file
}
