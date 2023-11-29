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

package anonymousfunc

import (
	"go/ast"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

// _wantClosurePrefix is a prefix that we use in the test file to specify the expected collected closure from the analyzer.
const _wantClosurePrefix = "expect_closure:"

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/anonymousfunc")
	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, Result{}, result)

	// Iterate over the result of the anonymous function analyzer to see if there
	// is a missmatch between the expected result written in the test file with the
	// collected result from the analyzer.
	funcLitMap := result.(Result).FuncLitMap
	require.NotZero(t, len(funcLitMap))

	// Get the expected closure vars from comments written for each function literal in the test file.
	expectedClosure := findExpectedClosure(pass)
	require.Equal(t, len(expectedClosure), len(funcLitMap))

	for funcLit, expectedVars := range expectedClosure {
		info, ok := funcLitMap[funcLit]
		require.True(t, ok)

		// Check if the expected closure vars are collected. We intentionally omit the default
		// length for `resultVars` to avoid comparing nil slices against empty slices.
		var resultVars []string
		for _, closureVar := range info.ClosureVars {
			resultVars = append(resultVars, closureVar.Ident.Name)
			require.NotNil(t, closureVar.Obj)
		}
		pos := pass.Fset.Position(funcLit.Pos())
		require.Equal(t, expectedVars, resultVars,
			"closure mismatch at %s:%d:%d", pos.Filename, pos.Line, pos.Column,
		)

		// Check if the fake func decl nodes are properly created.
		require.NotNil(t, info.FakeFuncDecl)
		// The fake func name should be an illegal identifier to avoid collisions, it should
		// also be unexported just to avoid accidental uses of such variables in the rest of
		// the system.
		require.True(t, strings.HasPrefix(info.FakeFuncDecl.Name.Name, _fakeFuncDeclPrefix))
		require.False(t, token.IsIdentifier(info.FakeFuncDecl.Name.Name))
		require.False(t, token.IsExported(info.FakeFuncDecl.Name.Name))

		// The func lit body and the fake func decl body should be shared.
		require.Equal(t, funcLit.Body, info.FakeFuncDecl.Body)

		// The parameter list should be extended to include closure variables.
		require.NotNil(t, info.FakeFuncDecl.Type)
		require.NotNil(t, info.FakeFuncDecl.Type.Params)
		funcLitParams, funcDeclParams := funcLit.Type.Params.List, info.FakeFuncDecl.Type.Params.List
		require.Len(t, funcDeclParams, len(funcLitParams)+len(info.ClosureVars))
		// We only need to check regular parameters when the func lit has regular parameters. Note
		// that directly checking `funcLitParams == funcDeclParams[:len(funcLitParams)]` does not
		// work since `require.Equal` distinguishes nil slices and empty slices by design. When
		// `len(funcLitParams) == 0`, LHS == nil and RHS == empty slice. In case there are
		// unexpected parameters in front of fake parameters, the error will be captured when we
		// strictly check the list of closure variables.
		if len(funcLitParams) != 0 {
			require.Equal(t, funcLitParams, funcDeclParams[:len(funcLitParams)])
		}
		// The rest of the param list should be closure variables.
		for i, v := range funcDeclParams[len(funcLitParams):] {
			// The fake params are all separate, i.e., we do not generate `(a, b any)`, but rather
			// `(a any, b any)`.
			require.Len(t, v.Names, 1)
			require.Equal(t, v.Names[0].Name, info.ClosureVars[i].Ident.Name)
		}

		// The fake type should be properly populated.
		require.False(t, info.FakeFuncObj.Exported())
		require.Equal(t, info.FakeFuncObj.Name(), info.FakeFuncDecl.Name.Name)
	}
}

// findExpectedClosure inspects the files and gather the comment strings at the same line of the
// *ast.FuncLit nodes, so that we know which *ast.FuncLit node corresponds to which anonymous
// function comment in the source.
func findExpectedClosure(pass *analysis.Pass) map[*ast.FuncLit][]string {
	results := make(map[*ast.FuncLit][]string)

	for _, file := range pass.Files {

		// Store a mapping between single comment's line number to its text.
		comments := make(map[int]string)
		for _, group := range file.Comments {
			if len(group.List) != 1 {
				continue
			}
			comment := group.List[0]
			comments[pass.Fset.Position(comment.Pos()).Line] = comment.Text
		}

		// Now, find all *ast.FuncLit nodes and find their comment.
		ast.Inspect(file, func(node ast.Node) bool {
			n, ok := node.(*ast.FuncLit)
			if !ok {
				return true
			}
			text, ok := comments[pass.Fset.Position(n.Pos()).Line]
			if !ok {
				// It is ok to not leave annotations for a func lit node - it simply does not use
				// any closure variables. We still need to traverse further since there could be
				// comments for nested func lit nodes.
				results[n] = nil
				return true
			}

			// Trim the trailing slashes and extra spaces and extract the set of expected values.
			text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
			text = strings.TrimSpace(strings.TrimPrefix(text, _wantClosurePrefix))
			// If no closure variables are written after _wantClosurePrefix, we simply ignore it.
			results[n] = nil
			if len(text) != 0 {
				results[n] = strings.Split(text, " ")
			}
			return true
		})
	}

	return results
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
