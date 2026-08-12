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
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestTypeSwitchGuardNoPanic verifies that collectClosure handles type-switch
// guard variables (e.g., `switch x := y.(type)`) without panicking. The guard
// ident has Obj.Kind == ast.Var per the parser, but TypesInfo.ObjectOf returns
// nil because the object is stored in Implicits, not Defs/Uses.
func TestTypeSwitchGuardNoPanic(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/typeswitchclosure")
	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	result, ok := r[0].Result.(*analysishelper.Result[map[*ast.FuncLit]*FuncLitInfo])
	require.True(t, ok, "result type mismatch")
	require.NoError(t, result.Err, "analyzer panicked on type-switch guard variable")
	require.NotNil(t, result.Res, "analyzer must return a result map")
}
