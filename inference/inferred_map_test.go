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

package inference

import (
	"bytes"
	"encoding/gob"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/annotation"
)

// BenchmarkGobEncoding benchmarks the gob encoding of an inferred map to test the overhead.
func BenchmarkGobEncoding(b *testing.B) {
	m := newBigInferredMap()

	b.ResetTimer()
	for b.Loop() {
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(m)
		require.NoError(b, err)
		require.NotEmpty(b, buf.Bytes())
	}
}

func TestEncoding_Size(t *testing.T) {
	t.Parallel()

	m := newBigInferredMap()
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(m)
	require.NoError(t, err)

	out := buf.Bytes()
	require.NotEmpty(t, out)
	require.Less(t, len(out), 30_000,
		"The gob encoding of a test inferred map is too large. We expect the encoded "+
			"map to be less than 30KB. This heavily affects the artifact sizes of the facts NilAway "+
			"produces, so the cap should only be increased with justification and thorough testing.",
	)
}

func TestEncoding_Deterministic(t *testing.T) {
	t.Parallel()

	m := newBigInferredMap()
	var previous []byte

	// Encode the inferred map 10 times and check that the result is always the same.
	for i := 0; i < 10; i++ {
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(m)
		require.NoError(t, err)
		require.NotEmpty(t, buf.Bytes())

		if len(previous) == 0 {
			previous = buf.Bytes()
			continue
		}
		require.Equal(t, previous, buf.Bytes())
	}
}

func TestDecoding(t *testing.T) {
	t.Parallel()

	m := NewInferredMap(nil /* primitive */)
	site := primitiveSite{
		Position: token.Position{
			Filename: "foo.go",
			Line:     1,
			Column:   2,
		},
	}
	value := TrueBecauseAnnotation{AnnotationPos: token.Position{Filename: "foo.go", Line: 1, Column: 2}}
	m.StoreDetermined(site, value)

	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(m)
	require.NoError(t, err)
	var decodedMap InferredMap
	err = gob.NewDecoder(&buf).Decode(&decodedMap)
	require.NoError(t, err)

	require.Equal(t, m.Len(), decodedMap.Len())
	v, ok := decodedMap.Load(site)
	require.True(t, ok)
	require.IsType(t, &DeterminedVal{}, v)
	require.Equal(t, value, v.(*DeterminedVal).Bool)
}

// newBigInferredMap creates an inferred map with 3000 sites, where the first 1000 are determined,
// and the next 2000 with implications between them for stress testing.
func newBigInferredMap() *InferredMap {
	m := NewInferredMap(nil /* primitivizer */)
	siteTemplate := primitiveSite{
		Position: token.Position{
			Filename: "foo.go",
			Line:     1,
			Column:   2,
		},
	}

	for i := 0; i < 1000; i++ {
		site1 := siteTemplate
		site1.Position.Line = i
		m.StoreDetermined(site1, TrueBecauseAnnotation{AnnotationPos: token.Position{Filename: "foo.go", Line: 1, Column: 2}})

		site2 := siteTemplate
		site2.Position.Line = 1000 + i
		site3 := siteTemplate
		site3.Position.Line = 2000 + i
		m.StoreImplication(site2, site3,
			primitiveFullTrigger{
				Position:     token.Position{Filename: "foo.go", Line: 1, Column: 2},
				ConsumerRepr: annotation.GlobalVarAssignPrestring{VarName: "foo"},
				ProducerRepr: annotation.GlobalVarAssignDeepPrestring{VarName: "bar"},
			},
		)
	}

	return m
}

func TestMain(m *testing.M) {
	// Register types to gob encoding for inferred maps.
	GobRegister()

	goleak.VerifyTestMain(m)
}
