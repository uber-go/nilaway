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

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestCompareDiagnostics(t *testing.T) {
	t.Parallel()

	tc := []struct {
		description string
		truth       GroundTruths
		collected   Diagnostics
		errContains []string
	}{
		{
			description: "empty",
			truth:       GroundTruths{},
			collected:   Diagnostics{},
			errContains: nil,
		},
		{
			description: "perfect match",
			truth: GroundTruths{
				{Filename: "file1", Line: 10}: {regexp.MustCompile("foo")},
				{Filename: "file2", Line: 11}: {regexp.MustCompile("bar")},
			},
			collected: Diagnostics{
				{Filename: "file1", Line: 10}: {"foo"},
				{Filename: "file2", Line: 11}: {"bar"},
			},
			errContains: nil,
		},
		{
			description: "multiple diagnostics with overlapping patterns",
			truth: GroundTruths{
				{Filename: "file1", Line: 10}: {
					regexp.MustCompile("dereferenced"),
					regexp.MustCompile("literal `nil` dereferenced"),
				},
			},
			collected: Diagnostics{
				{Filename: "file1", Line: 10}: {
					"literal `nil` dereferenced",
					"function parameter dereferenced",
				},
			},
			errContains: nil,
		},
		{
			description: "mismatch",
			truth: GroundTruths{
				{Filename: "file1", Line: 10}: {regexp.MustCompile("foo")},
				{Filename: "file2", Line: 11}: {regexp.MustCompile("bar")},
			},
			collected: Diagnostics{
				{Filename: "file1", Line: 10}: {"foo"},
				{Filename: "file2", Line: 11}: {"baz"},
			},
			errContains: []string{"mismatch", "file2:11", "baz"},
		},
		{
			description: "missing",
			truth: GroundTruths{
				{Filename: "file1", Line: 10}: {regexp.MustCompile("foo")},
				{Filename: "file2", Line: 11}: {regexp.MustCompile("bar")},
			},
			collected: Diagnostics{
				{Filename: "file1", Line: 10}: {"foo"},
			},
			errContains: []string{"missing", "file2:11", "bar"},
		},
		{
			description: "extra",
			truth: GroundTruths{
				{Filename: "file1", Line: 10}: {regexp.MustCompile("foo")},
			},
			collected: Diagnostics{
				{Filename: "file1", Line: 10}: {"foo"},
				{Filename: "file2", Line: 11}: {"bar"},
			},
			errContains: []string{"unexpected", "file2:11", "bar"},
		},
	}

	for _, tt := range tc {
		tt := tt
		t.Run(tt.description, func(t *testing.T) {
			t.Parallel()

			err := CompareDiagnostics(tt.truth, tt.collected)
			if len(tt.errContains) == 0 {
				require.NoError(t, err)
				return
			}
			for _, s := range tt.errContains {
				require.ErrorContains(t, err, s)
			}
		})
	}
}

func TestCollectGroundTruths(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "pkg", "test.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(`package pkg

func f() {
	println() // want "first" `+"`second`"+`
	// want +1 "later"
	println()
}
`), 0644))

	truths, err := CollectGroundTruths(dir)
	require.NoError(t, err)
	require.Equal(t, GroundTruths{
		{Filename: "pkg/test.go", Line: 4}: {
			regexp.MustCompile("first"),
			regexp.MustCompile("second"),
		},
		{Filename: "pkg/test.go", Line: 6}: {
			regexp.MustCompile("later"),
		},
	}, truths)
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
