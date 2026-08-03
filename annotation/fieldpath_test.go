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

package annotation

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFieldPathConstruction(t *testing.T) {
	t.Parallel()

	root := FieldPath{}
	require.True(t, root.IsRoot())
	require.Equal(t, "", root.String())
	require.Nil(t, root.Segments())
	require.Equal(t, 0, root.NumSegments())
	require.Equal(t, root, NewFieldPath())

	deep := NewFieldPath("Mid", "Child")
	require.False(t, deep.IsRoot())
	require.Equal(t, "Mid.Child", deep.String())
	require.Equal(t, []string{"Mid", "Child"}, deep.Segments())
	require.Equal(t, 2, deep.NumSegments())

	require.Equal(t, deep, root.Child("Mid").Child("Child"))
	require.Equal(t, deep, NewFieldPath("Mid").Child("Child"))
}

func TestFieldPathJoin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		prefix, sub FieldPath
		want        FieldPath
	}{
		{name: "root and root", prefix: FieldPath{}, sub: FieldPath{}, want: FieldPath{}},
		{name: "root prefix is identity", prefix: FieldPath{}, sub: NewFieldPath("f"), want: NewFieldPath("f")},
		{name: "root sub is identity", prefix: NewFieldPath("inner"), sub: FieldPath{}, want: NewFieldPath("inner")},
		{name: "both non-root", prefix: NewFieldPath("inner"), sub: NewFieldPath("f"), want: NewFieldPath("inner", "f")},
		{name: "deep both sides", prefix: NewFieldPath("a", "b"), sub: NewFieldPath("c", "d"), want: NewFieldPath("a", "b", "c", "d")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.prefix.Join(tt.sub))
		})
	}
}

func TestFieldPathTrimPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		path, prefix FieldPath
		want         FieldPath
		wantOK       bool
	}{
		{name: "root prefix is identity", path: NewFieldPath("Mid", "Child"), prefix: FieldPath{}, want: NewFieldPath("Mid", "Child"), wantOK: true},
		{name: "exact match trims to root", path: NewFieldPath("Mid"), prefix: NewFieldPath("Mid"), want: FieldPath{}, wantOK: true},
		{name: "proper prefix", path: NewFieldPath("Mid", "Child"), prefix: NewFieldPath("Mid"), want: NewFieldPath("Child"), wantOK: true},
		{name: "segment boundary respected", path: NewFieldPath("Middle"), prefix: NewFieldPath("Mid"), wantOK: false},
		{name: "prefix longer than path", path: NewFieldPath("Mid"), prefix: NewFieldPath("Mid", "Child"), wantOK: false},
		{name: "disjoint", path: NewFieldPath("Other"), prefix: NewFieldPath("Mid"), wantOK: false},
		{name: "root path under non-root prefix", path: FieldPath{}, prefix: NewFieldPath("Mid"), wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sub, ok := tt.path.TrimPrefix(tt.prefix)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.want, sub)
		})
	}
}

// TestFieldPathJoinTrimRoundTrip pins the inverse contract: prefix.Join(sub).TrimPrefix(prefix)
// recovers sub for every combination of root and non-root operands.
func TestFieldPathJoinTrimRoundTrip(t *testing.T) {
	t.Parallel()

	paths := []FieldPath{{}, NewFieldPath("a"), NewFieldPath("a", "b")}
	for _, prefix := range paths {
		for _, sub := range paths {
			got, ok := prefix.Join(sub).TrimPrefix(prefix)
			require.True(t, ok)
			require.Equal(t, sub, got)
		}
	}
}

func TestFieldPathCompare(t *testing.T) {
	t.Parallel()

	require.Equal(t, 0, NewFieldPath("a", "b").Compare(NewFieldPath("a", "b")))
	require.Negative(t, FieldPath{}.Compare(NewFieldPath("a")))
	require.Positive(t, NewFieldPath("b").Compare(NewFieldPath("a")))
}

func TestFieldPathGobRoundTrip(t *testing.T) {
	t.Parallel()

	type carrier struct {
		Path  FieldPath
		Other FieldPath
	}
	want := carrier{Path: NewFieldPath("Mid", "Child")}
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(want))
	var got carrier
	require.NoError(t, gob.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&got))
	require.Equal(t, want, got)
}
