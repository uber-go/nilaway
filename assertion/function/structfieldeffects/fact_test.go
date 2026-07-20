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
	"bytes"
	"encoding/gob"
	"fmt"
	"go/ast"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/nilawaytest"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/types/objectpath"
)

func TestFact(t *testing.T) { //nolint:paralleltest
	enableStructInitV2(t)

	results := analysistest.Run(t, analysistest.TestData(), Analyzer,
		"go.uber.org/paramfieldeffects/factexport/upstream",
		"go.uber.org/paramfieldeffects/factexport/downstream",
	)
	require.Len(t, results, 2)
	for _, result := range results {
		pass := result.Pass
		effects := result.Result.(*analysishelper.Result[*BoundaryFieldEffects])
		require.NoError(t, effects.Err)

		localReads := make(fieldEffects)
		localWrites := make(fieldEffects)
		for funcObj, reads := range effects.Res.ParamReads {
			if funcObj.Pkg() == pass.Pkg {
				localReads[funcObj] = reads
			}
		}
		for funcObj, writes := range effects.Res.ParamWrites {
			if funcObj.Pkg() == pass.Pkg {
				localWrites[funcObj] = writes
			}
		}
		requireEffects(t, localReads, expectedParamReads(t, pass))
		requireEffects(t, localWrites, expectedParamWrites(t, pass))
	}
}

func TestBoundaryFieldEffectsPackageFactCodec(t *testing.T) {
	t.Parallel()

	fact := newFact(100)
	var previous []byte
	for range 10 {
		var buf bytes.Buffer
		require.NoError(t, gob.NewEncoder(&buf).Encode(fact))
		encoded := append([]byte(nil), buf.Bytes()...)
		require.NotEmpty(t, encoded)
		require.Less(t, len(encoded), 15_000,
			"encoded facts contribute to artifact size; increase this cap only with justification")

		var decoded BoundaryFieldEffectsPackageFact
		require.NoError(t, gob.NewDecoder(bytes.NewReader(encoded)).Decode(&decoded))
		require.Equal(t, fact, &decoded)

		if previous != nil {
			require.Equal(t, previous, encoded, "encoded fact must be deterministic")
		}
		previous = encoded
	}
}

// BenchmarkBoundaryFieldEffectsPackageFactCodec measures one complete fact in a fresh gob stream.
func BenchmarkBoundaryFieldEffectsPackageFactCodec(b *testing.B) {
	for _, functionCount := range []int{1, 10, 100} {
		fact := newFact(functionCount)
		b.Run(fmt.Sprintf("encode/%d-functions", functionCount), func(b *testing.B) {
			b.ReportAllocs()
			var encodedBytes int
			b.ResetTimer()
			for b.Loop() {
				var buf bytes.Buffer
				if err := gob.NewEncoder(&buf).Encode(fact); err != nil {
					b.Fatal(err)
				}
				if buf.Len() == 0 {
					b.Fatal("encoded fact is empty")
				}
				encodedBytes = buf.Len()
			}
			b.ReportMetric(float64(encodedBytes), "encoded_bytes")
		})

		var encoded bytes.Buffer
		if err := gob.NewEncoder(&encoded).Encode(fact); err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("decode/%d-functions", functionCount), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				var decoded BoundaryFieldEffectsPackageFact
				if err := gob.NewDecoder(bytes.NewReader(encoded.Bytes())).Decode(&decoded); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func enableStructInitV2(t *testing.T) {
	t.Helper()
	require.NoError(t, config.Analyzer.Flags.Set(config.ExperimentalStructInitV2EnableFlag, "true"))
	t.Cleanup(func() {
		require.NoError(t, config.Analyzer.Flags.Set(config.ExperimentalStructInitV2EnableFlag, "false"))
	})
}

func expectedParamReads(t *testing.T, pass *analysis.Pass) map[*types.Func][]IndexedFieldPath {
	return expectedParamEffects(t, pass, "param_reads")
}

func expectedParamWrites(t *testing.T, pass *analysis.Pass) map[*types.Func][]IndexedFieldPath {
	return expectedParamEffects(t, pass, "param_writes")
}

func expectedParamEffects(t *testing.T, pass *analysis.Pass, wantKind string) map[*types.Func][]IndexedFieldPath {
	t.Helper()

	want := make(map[*types.Func][]IndexedFieldPath)
	for node, tokens := range nilawaytest.FindExpectedValues(pass, _expectEffectsPrefix) {
		funcDecl, ok := node.(*ast.FuncDecl)
		require.True(t, ok)
		funcObj, ok := pass.TypesInfo.ObjectOf(funcDecl.Name).(*types.Func)
		require.True(t, ok)
		for _, token := range tokens {
			kind, key := parseExpectedEffect(t, token)
			if kind == wantKind {
				want[funcObj] = append(want[funcObj], key)
			}
		}
	}
	return want
}

func newFact(functionCount int) *BoundaryFieldEffectsPackageFact {
	functions := make([]FunctionFieldEffects, functionCount)
	for i := range functions {
		functions[i] = FunctionFieldEffects{
			FunctionObjectPath: objectpath.Path(fmt.Sprintf("Function%03d", i)),
			ParamReads: []IndexedFieldPath{
				{Idx: -1, Path: "Receiver"},
				{Idx: 0, Path: "Child"},
				{Idx: 0, Path: "Child.Leaf"},
				{Idx: 1, Path: fmt.Sprintf("Argument%d.Field", i)},
			},
			ParamWrites: []IndexedFieldPath{
				{Idx: -1, Path: "Receiver.Child"},
				{Idx: 0, Path: "Child"},
				{Idx: 0, Path: "Child.Leaf"},
				{Idx: 1, Path: fmt.Sprintf("Argument%d.Field", i)},
			},
		}
	}
	return &BoundaryFieldEffectsPackageFact{Functions: functions}
}
