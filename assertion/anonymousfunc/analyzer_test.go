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
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/nilawaytest"
	"golang.org/x/tools/go/analysis/analysistest"
)

// _wantClosurePrefix is a prefix that we use in the test file to specify the expected collected closure from the analyzer.
const _wantClosurePrefix = "expect_closure:"

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	// Intentionally give a nil pass variable to trigger a panic, but we should recover from it
	// and convert it to an error via the result struct.
	r, err := Analyzer.Run(nil /* pass */)
	require.NoError(t, err)
	require.ErrorContains(t, r.(*analysishelper.Result[map[*ast.FuncLit]*FuncLitInfo]).Err, "INTERNAL PANIC")
}

func TestClosureCollection(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/anonymousfunc")
	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, &analysishelper.Result[map[*ast.FuncLit]*FuncLitInfo]{}, result)

	// Iterate over the result of the anonymous function analyzer to see if there
	// is a missmatch between the expected result written in the test file with the
	// collected result from the analyzer.
	funcLitMap := result.(*analysishelper.Result[map[*ast.FuncLit]*FuncLitInfo]).Res
	require.NotZero(t, len(funcLitMap))

	// Get the expected closure vars from comments written for each function literal in the test file.
	// FindExpectedValues inspects test files and gathers comment strings at the same line of the
	// *ast.FuncLit nodes, so that we know which *ast.FuncLit node corresponds to which anonymous
	// function comment in the source.
	expectedValues := nilawaytest.FindExpectedValues(pass, _wantClosurePrefix)
	require.Equal(t, len(expectedValues), len(funcLitMap))

	funcLitExpectedClosure := make(map[*ast.FuncLit][]string)
	for node, closureVars := range expectedValues {
		if funcLit, ok := node.(*ast.FuncLit); ok {
			funcLitExpectedClosure[funcLit] = closureVars
		}
	}

	for funcLit, expectedVars := range funcLitExpectedClosure {
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

func TestMain(m *testing.M) {
	// Enable anonymous function flag for tests. It is OK to not unset this flag since Go builds
	// tests for each package into separate binaries and execute them in parallel [1]. So the
	// config.Analyzer here is actually not shared with other tests in other packages.
	// [1]: https://pkg.go.dev/cmd/go/internal/test
	err := config.Analyzer.Flags.Set(config.ExperimentalAnonymousFunctionFlag, "true")
	if err != nil {
		log.Fatalf("Error setting anonymous function flag for tests: %q", err)
	}
	goleak.VerifyTestMain(m)
}
