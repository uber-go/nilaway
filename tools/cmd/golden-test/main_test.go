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
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/config"
)

func TestParseDiagnostics(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	// Check that invalid JSON is handled.
	buf.WriteString(`{`)
	diagnostics, err := ParseDiagnostics(&buf)
	require.Error(t, err)
	require.Empty(t, diagnostics)

	// Now check a valid case.
	buf.Reset()
	buf.WriteString(`{
	"pkg1":{"nilaway":[{"posn":"src/file1:10:2","message":"nil pointer dereference"}]},
	"pkg2":{"nilaway":[{"posn":"src/file2:10:2","message":"foo"}, {"posn":"src/file2:11:2","message":"bar"}]}
}`)
	diagnostics, err = ParseDiagnostics(&buf)
	require.NoError(t, err)
	require.Equal(t, map[Diagnostic]bool{
		{Posn: "src/file1:10:2", Message: "nil pointer dereference"}: true,
		{Posn: "src/file2:10:2", Message: "foo"}:                     true,
		{Posn: "src/file2:11:2", Message: "bar"}:                     true,
	}, diagnostics)
}

func TestWriteDiff(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	base := map[Diagnostic]bool{
		// Same in both.
		{Posn: "src/file1:10:2", Message: "nil pointer dereference"}: true,
	}
	test := map[Diagnostic]bool{
		// Same in both.
		{Posn: "src/file1:10:2", Message: "nil pointer dereference"}: true,
	}
	branches := [2]*BranchResult{
		{Name: "base", ShortSHA: "123456", Result: base},
		{Name: "test", ShortSHA: "456789", Result: test},
	}
	WriteDiff(&buf, branches)
	require.Contains(t, buf.String(), "## Golden Test") // Must contain the title.
	require.Contains(t, buf.String(), "are **identical**")

	// Add two different diagnostics to base and test and check that they are reported.
	base[Diagnostic{Posn: "src/file2:10:2", Message: "nil pointer dereference"}] = true
	base[Diagnostic{Posn: "src/z:10:2", Message: config.InternalPanicPrefix + ": boom"}] = true
	test[Diagnostic{Posn: "src/file4:10:2", Message: "bar error"}] = true
	buf.Reset()
	WriteDiff(&buf, branches)
	s := buf.String()
	require.Contains(t, buf.String(), "## Golden Test") // Must contain the title.
	require.Contains(t, s, "are **different**")
	require.Contains(t, s, "- src/file2:10:2: nil pointer dereference")
	require.Contains(t, s, "+ src/file4:10:2: bar error")
	panicIndex := strings.Index(s, "- src/z:10:2: "+config.InternalPanicPrefix+": boom")
	normalIndex := strings.Index(s, "+ src/file4:10:2: bar error")
	require.NotEqual(t, -1, panicIndex)
	require.NotEqual(t, -1, normalIndex)
	require.Less(t, panicIndex, normalIndex)
}

func TestDiff(t *testing.T) {
	t.Parallel()

	base := map[Diagnostic]bool{
		// Same in both.
		{Posn: "src/file1:10:2", Message: "nil pointer dereference"}: true,
		// Internal panics must be ordered first regardless of position.
		{Posn: "src/z:10:2", Message: config.InternalPanicPrefix + ": boom"}: true,
		// Differs in position.
		{Posn: "src/file2:10:2", Message: "nil pointer dereference"}: true,
		// Differs in message.
		{Posn: "src/file4:10:2", Message: "foo error"}: true,
	}
	test := map[Diagnostic]bool{
		// Same in both.
		{Posn: "src/file1:10:2", Message: "nil pointer dereference"}: true,
		// Differs in position.
		{Posn: "src/file3:10:2", Message: "nil pointer dereference"}: true,
		// Differs in message.
		{Posn: "src/file4:10:2", Message: "bar error"}: true,
	}

	minuses := Diff(base, test)
	require.Equal(t, []Diagnostic{
		// Internal panic.
		{Posn: "src/z:10:2", Message: config.InternalPanicPrefix + ": boom"},
		// Differs in position.
		{Posn: "src/file2:10:2", Message: "nil pointer dereference"},
		// Differs in message.
		{Posn: "src/file4:10:2", Message: "foo error"},
	}, minuses)

	pluses := Diff(test, base)
	require.Equal(t, []Diagnostic{
		// Differs in position.
		{Posn: "src/file3:10:2", Message: "nil pointer dereference"},
		// Differs in message.
		{Posn: "src/file4:10:2", Message: "bar error"},
	}, pluses)
}

func TestCheckInternalPanics(t *testing.T) {
	t.Parallel()

	branches := [2]*BranchResult{
		{Name: "base", ShortSHA: "123456", Result: map[Diagnostic]bool{
			{Message: "nil pointer dereference"}: true,
		}},
		{Name: "test", ShortSHA: "456789", Result: map[Diagnostic]bool{}},
	}
	require.NoError(t, CheckInternalPanics(branches))

	branches[1].Result[Diagnostic{
		Message: "INTERNAL ERROR(s):\n" + config.InternalPanicPrefix + " from analyzer",
	}] = true
	err := CheckInternalPanics(branches)
	require.ErrorContains(t, err, config.InternalPanicPrefix+" diagnostic(s)")
	require.ErrorContains(t, err, "test (456789): 1")
}

func TestMustFprint(t *testing.T) {
	t.Parallel()

	require.Panics(t, func() {
		MustFprint(0, errors.New("test"))
	})
	require.NotPanics(t, func() {
		MustFprint(0, nil)
	})
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
