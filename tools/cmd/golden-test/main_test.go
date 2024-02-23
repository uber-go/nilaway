package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
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
	test[Diagnostic{Posn: "src/file4:10:2", Message: "bar error"}] = true
	buf.Reset()
	WriteDiff(&buf, branches)
	s := buf.String()
	require.Contains(t, buf.String(), "## Golden Test") // Must contain the title.
	require.Contains(t, s, "are **different**")
	require.Contains(t, s, "- src/file2:10:2: nil pointer dereference")
	require.Contains(t, s, "+ src/file4:10:2: bar error")
}

func TestDiff(t *testing.T) {
	t.Parallel()

	base := map[Diagnostic]bool{
		// Same in both.
		{Posn: "src/file1:10:2", Message: "nil pointer dereference"}: true,
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
