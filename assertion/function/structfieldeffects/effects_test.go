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

package structfieldeffects

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/nilawaytest"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis/analysistest"
)

// _expectEffectsPrefix precedes a fixture function's expected boundary effects. Each trailing token
// is "<kind>:<idx>:<path>", where kind identifies Writes, ParamReads, or ReturnReads. A function
// with no effects carries an empty comment.
const _expectEffectsPrefix = "expect_effects:"

func TestComputeBoundaryFieldEffects(t *testing.T) {
	t.Parallel()
	err := config.Analyzer.Flags.Set(config.ExperimentalStructInitV2EnableFlag, "true")
	require.NoError(t, err)
	defer func() {
		err := config.Analyzer.Flags.Set(config.ExperimentalStructInitV2EnableFlag, "false")
		require.NoError(t, err)
	}()

	testdata := analysistest.TestData()
	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/paramfieldeffects")
	require.Len(t, r, 1)
	pass := r[0].Pass
	result := r[0].Result.(*analysishelper.Result[*BoundaryFieldEffects])
	require.NoError(t, result.Err)
	effects := result.Res

	// Read the expected boundary effects from fixture comments and split them by effect kind.
	wantParam := make(map[*types.Func][]IndexedFieldPath)
	wantReturn := make(map[*types.Func][]IndexedFieldPath)
	wantWrites := make(map[*types.Func][]IndexedFieldPath)
	for node, tokens := range nilawaytest.FindExpectedValues(pass, _expectEffectsPrefix) {
		fd, ok := node.(*ast.FuncDecl)
		require.True(t, ok)
		funcObj, ok := pass.TypesInfo.ObjectOf(fd.Name).(*types.Func)
		require.True(t, ok)
		for _, token := range tokens {
			kind, key := parseExpectedEffect(t, token)
			switch kind {
			case "param_writes":
				wantWrites[funcObj] = append(wantWrites[funcObj], key)
			case "param_reads":
				wantParam[funcObj] = append(wantParam[funcObj], key)
			case "return_reads":
				wantReturn[funcObj] = append(wantReturn[funcObj], key)
			default:
				t.Fatalf("unknown effect kind %q in token %q", kind, token)
			}
		}
	}

	requireEffects(t, effects.ParamReads, wantParam)
	requireEffects(t, effects.ReturnReads, wantReturn)
	requireEffects(t, effects.ParamWrites, wantWrites)
}

// parseExpectedEffect splits a "<kind>:<idx>:<path>" expect_effects token into its kind and boundary key.
func parseExpectedEffect(t *testing.T, token string) (string, IndexedFieldPath) {
	parts := strings.SplitN(token, ":", 3)
	require.Lenf(t, parts, 3, "malformed expect_effects token %q", token)
	idx, err := strconv.Atoi(parts[1])
	require.NoErrorf(t, err, "malformed index in expect_effects token %q", token)
	return parts[0], IndexedFieldPath{Idx: idx, Path: parts[2]}
}

// requireEffects asserts the computed effect set matches want for every function in either map. so
// both missing and unexpected effects fail the test.
func requireEffects(t *testing.T, got fieldEffects, want map[*types.Func][]IndexedFieldPath) {
	funcs := make(map[*types.Func]bool)
	for funcObj := range got {
		funcs[funcObj] = true
	}
	for funcObj := range want {
		funcs[funcObj] = true
	}
	for funcObj := range funcs {
		var gotKeys []IndexedFieldPath
		for key := range got[funcObj] {
			gotKeys = append(gotKeys, key)
		}
		require.ElementsMatchf(t, want[funcObj], gotKeys, "effects mismatch for %s", funcObj.Name())
	}
}
