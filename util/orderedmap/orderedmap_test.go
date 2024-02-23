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

package orderedmap_test

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/util/orderedmap"
)

func TestLoadStore(t *testing.T) {
	t.Parallel()

	// Specify expected k, v pairs.
	pairs := [][2]int{{1, 2}, {2, 3}, {3, 4}}
	m := orderedmap.New[int, int]()
	for _, p := range pairs {
		k, v := p[0], p[1]
		m.Store(k, v)
		loadedV, ok := m.Load(k)
		require.True(t, ok)
		require.Equal(t, v, loadedV)
		require.Equal(t, v, m.Value(k))
	}

	// Test loading a non-existent key.
	v, ok := m.Load(-1)
	require.False(t, ok)
	require.Empty(t, v)
	require.Empty(t, m.Value(-1))

	// Test Len.
	require.Equal(t, len(pairs), len(m.Pairs))
}

func TestRange(t *testing.T) {
	t.Parallel()

	// Create a map with 100 <i, i+1> pairs to have better chance of breaking ordered range.
	pairs := make([][2]int, 0, 100)
	for i := 0; i < 100; i++ {
		k, v := i, i+1
		pairs = append(pairs, [2]int{k, v})
	}

	m := orderedmap.New[int, int]()
	for _, p := range pairs {
		k, v := p[0], p[1]
		m.Store(k, v)
	}

	// Collect expected order of keys.
	expectedKeys := make([]int, 0, len(pairs))
	for _, p := range pairs {
		expectedKeys = append(expectedKeys, p[0])
	}

	// Run 5 concurrent subtests to ensure that the order is always the same.
	for i := 0; i < 5; i++ {
		t.Run(fmt.Sprintf("Run%d", i), func(t *testing.T) {
			t.Parallel()

			keys := make([]int, 0, len(pairs))
			for _, p := range m.Pairs {
				keys = append(keys, p.Key)
			}
			require.Equal(t, expectedKeys, keys)
		})
	}
}

// Define an interface and two structs that implement it for testing the ability to encode/decode
// from and to Go interfaces.

type I interface {
	Foo()
}

type A struct{ Number int }

func (a *A) Foo() {}

type B struct{}

func (b *B) Foo() {}

func TestStoringInterfaces(t *testing.T) {
	t.Parallel()

	m := orderedmap.New[int, I]()
	m.Store(1, &A{Number: 1})
	m.Store(2, &B{})

	v, ok := m.Load(1)
	require.True(t, ok)
	require.NotNil(t, v)
	require.IsType(t, &A{}, v)
	require.Equal(t, 1, v.(*A).Number)

	v, ok = m.Load(2)
	require.True(t, ok)
	require.NotNil(t, v)
	require.IsType(t, &B{}, v)
}

func TestEncoding(t *testing.T) {
	t.Parallel()

	m := orderedmap.New[A, I]()
	m.Store(A{Number: 1}, &A{})
	m.Store(A{Number: 2}, &B{})

	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(m)
	require.NoError(t, err)
	require.NotEmpty(t, buf.Bytes())

	// Now, we decode the map. Note that the decoding logic is usually invoked by library or
	// frameworks, meaning the map will be constructed via plain Go composite literals instead of
	// via our manually-defined constructor (i.e., orderedmap.New). Here, we mimic this behavior
	// and test our decoding logic for graceful handling of such cases.
	decodedMap := &orderedmap.OrderedMap[A, I]{}
	err = gob.NewDecoder(&buf).Decode(&decodedMap)
	require.NoError(t, err)

	// Ensure our decoded map is equal to the encoded one.
	v, ok := decodedMap.Load(A{Number: 1})
	require.True(t, ok)
	require.NotNil(t, v)
	require.IsType(t, &A{}, v)
	v, ok = decodedMap.Load(A{Number: 2})
	require.True(t, ok)
	require.NotNil(t, v)
	require.IsType(t, &B{}, v)

	// Use the decoded map as a regular ordered map.
	decodedMap.Store(A{Number: 3}, &A{Number: 4})
	v = decodedMap.Value(A{Number: 3})
	require.NotNil(t, v)
	require.IsType(t, &A{}, v)
	require.Equal(t, 4, v.(*A).Number)
}

func TestEncoding_Deterministic(t *testing.T) {
	t.Parallel()

	m := orderedmap.New[A, I]()
	m.Store(A{Number: 1}, &A{})
	m.Store(A{Number: 2}, &B{})

	// We encode the map 10 times and check that the result is always the same.
	var previous []byte
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

func TestEncode_Empty(t *testing.T) {
	t.Parallel()

	m := orderedmap.New[int, int]()
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(m)
	require.NoError(t, err)
	// Gob encodes type information even for empty maps, so the result is non-empty.
}

func TestMain(m *testing.M) {
	// Register structs that implement the interface I for gob encoding/decoding.
	gob.Register(&A{})
	gob.Register(&B{})

	goleak.VerifyTestMain(m)
}
