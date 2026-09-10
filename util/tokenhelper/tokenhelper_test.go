//  Copyright (c) 2025 Uber Technologies, Inc.
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

package tokenhelper

import (
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRelToCwd(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	testcases := []struct {
		give string
		want string
	}{
		{give: filepath.Join(cwd, "testdata", "foo.go"), want: filepath.Join("testdata", "foo.go")},
		{give: filepath.Join("testdata", "foo.go"), want: filepath.Join("testdata", "foo.go")},
	}
	for _, tc := range testcases {
		t.Run(tc.give, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, RelToCwd(tc.give))
		})
	}
}

func TestConverse(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		give token.Token
		want token.Token
	}{
		{give: token.EQL, want: token.EQL},
		{give: token.NEQ, want: token.NEQ},
		{give: token.LSS, want: token.GTR},
		{give: token.GTR, want: token.LSS},
		{give: token.LEQ, want: token.GEQ},
		{give: token.GEQ, want: token.LEQ},
	}
	for _, tc := range testcases {
		t.Run(tc.give.String(), func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, Converse(tc.give))
		})
	}
}

func TestConversePanicsForInvalidToken(t *testing.T) {
	require.Panics(t, func() {
		Converse(token.ADD)
	})
}

func TestInverse(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		give token.Token
		want token.Token
	}{
		{give: token.EQL, want: token.NEQ},
		{give: token.NEQ, want: token.EQL},
		{give: token.LSS, want: token.GEQ},
		{give: token.GTR, want: token.LEQ},
		{give: token.LEQ, want: token.GTR},
		{give: token.GEQ, want: token.LSS},
	}
	for _, tc := range testcases {
		t.Run(tc.give.String(), func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, Inverse(tc.give))
		})
	}
}

func TestInversePanicsForInvalidToken(t *testing.T) {
	require.Panics(t, func() {
		Inverse(token.ADD)
	})
}

func TestPortionAfterSep(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		name  string
		input string
		sep   string
		occ   int
		want  string
	}{
		{name: "no separator", input: "file.go", sep: "/", occ: 0, want: "file.go"},
		{name: "fewer separators than requested", input: "a/b", sep: "/", occ: 2, want: "a/b"},
		{name: "zero occurrences", input: "a/b/c", sep: "/", occ: 0, want: "c"},
		{name: "one occurrence", input: "a/b/c", sep: "/", occ: 1, want: "b/c"},
		{name: "multiple occurrences", input: "a/b/c/d", sep: "/", occ: 2, want: "b/c/d"},
		{name: "empty path component", input: "a/b//c", sep: "/", occ: 2, want: "b//c"},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, PortionAfterSep(tc.input, tc.sep, tc.occ))
		})
	}
}
