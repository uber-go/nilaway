//	Copyright (c) 2023 Uber Technologies, Inc.
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
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestCompareDiagnostics(t *testing.T) {
	t.Parallel()

	tc := []struct {
		description string
		truth       map[Position]*regexp.Regexp
		collected   map[Position]string
		errContains []string
	}{
		{
			description: "empty",
			truth:       map[Position]*regexp.Regexp{},
			collected:   map[Position]string{},
			errContains: nil,
		},
		{
			description: "perfect match",
			truth: map[Position]*regexp.Regexp{
				{Filename: "file1", Line: 10}: regexp.MustCompile("foo"),
				{Filename: "file2", Line: 11}: regexp.MustCompile("bar"),
			},
			collected: map[Position]string{
				{Filename: "file1", Line: 10}: "foo",
				{Filename: "file2", Line: 11}: "bar",
			},
			errContains: nil,
		},
		{
			description: "mismatch",
			truth: map[Position]*regexp.Regexp{
				{Filename: "file1", Line: 10}: regexp.MustCompile("foo"),
				{Filename: "file2", Line: 11}: regexp.MustCompile("bar"),
			},
			collected: map[Position]string{
				{Filename: "file1", Line: 10}: "foo",
				{Filename: "file2", Line: 11}: "baz",
			},
			errContains: []string{"mismatch", "file2:11", "baz"},
		},
		{
			description: "missing",
			truth: map[Position]*regexp.Regexp{
				{Filename: "file1", Line: 10}: regexp.MustCompile("foo"),
				{Filename: "file2", Line: 11}: regexp.MustCompile("bar"),
			},
			collected: map[Position]string{
				{Filename: "file1", Line: 10}: "foo",
			},
			errContains: []string{"missing", "file2:11", "bar"},
		},
		{
			description: "extra",
			truth: map[Position]*regexp.Regexp{
				{Filename: "file1", Line: 10}: regexp.MustCompile("foo"),
			},
			collected: map[Position]string{
				{Filename: "file1", Line: 10}: "foo",
				{Filename: "file2", Line: 11}: "bar",
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

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
