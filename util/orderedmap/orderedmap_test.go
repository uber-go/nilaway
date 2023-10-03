package orderedmap_test

import (
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
	require.Equal(t, len(pairs), m.Len())
}

func TestOrderedRange(t *testing.T) {
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
			m.OrderedRange(func(key int, value int) bool {
				keys = append(keys, key)
				return true
			})
			require.Equal(t, expectedKeys, keys)
		})
	}
}

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

func TestGobEncoding(t *testing.T) {
	t.Parallel()

	m := orderedmap.New[A, I]()
	m.Store(A{Number: 1}, &A{})
	m.Store(A{Number: 2}, &B{})

	b, err := m.GobEncode()
	require.NoError(t, err)
	require.NotEmpty(t, b)

	decodedMap := orderedmap.New[A, I]()
	err = decodedMap.GobDecode(b)
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
}

func TestGobEncoding_Deterministic(t *testing.T) {
	t.Parallel()

	m := orderedmap.New[A, I]()
	m.Store(A{Number: 1}, &A{})
	m.Store(A{Number: 2}, &B{})

	// We encode the map 5 times and check that the result is always the same.
	var encoded []byte
	for i := 0; i < 5; i++ {
		b, err := m.GobEncode()
		require.NoError(t, err)
		require.NotEmpty(t, b)
		if len(encoded) == 0 {
			encoded = b
			continue
		}
		require.Equal(t, encoded, b)
	}
}

func TestGobEncode_Empty(t *testing.T) {
	t.Parallel()

	m := orderedmap.New[int, int]()
	b, err := m.GobEncode()
	require.NoError(t, err)
	require.Empty(t, b)
}

func TestMain(m *testing.M) {
	// Register structs that implement the interface I for gob encoding/decoding.
	gob.Register(&A{})
	gob.Register(&B{})

	goleak.VerifyTestMain(m)
}
