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

package assertiontree

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

func TestIsErrorReturnNonnilStdlibConstructors(t *testing.T) {
	t.Parallel()
	const source = `package test
import ("errors"; "fmt")
func test() (error, error) { return errors.New("x"), fmt.Errorf("x") }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, 0)
	require.NoError(t, err)
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Uses: make(map[*ast.Ident]types.Object)}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("test", fset, []*ast.File{file}, info)
	require.NoError(t, err)
	pass := analysishelper.NewEnhancedPass(&analysis.Pass{Fset: fset, Files: []*ast.File{file}, Pkg: pkg, TypesInfo: info})
	root := &RootAssertionNode{functionContext: FunctionContext{pass: pass}}

	var calls []*ast.CallExpr
	ast.Inspect(file, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok {
			calls = append(calls, call)
		}
		return true
	})
	require.Len(t, calls, 2)
	for _, call := range calls {
		require.True(t, isErrorReturnNonnil(root, call))
	}
}
