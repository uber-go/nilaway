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

package nilawaytest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/unitchecker"
)

// _asVettoolEnvVar, when set, makes the test binary behave as a `go vet -vettool`
// worker instead of running the test suite.
const _asVettoolEnvVar = "NILAWAY_AS_VETTOOL"

var _sourceColumnRegexp = regexp.MustCompile(`(\.go:[0-9]+):[0-9]+`)

// Diagnostic is a normalized analysis diagnostic. Columns are intentionally omitted because
// source positions loaded from export data may not retain accurate column information under
// modular drivers.
type Diagnostic struct {
	File    string
	Line    int
	Message string
}

// RunAsModularDriver calls analyzer as a unitchecker worker when the current test binary was
// invoked by RunModularAnalysis. It must be called from TestMain before running the test suite.
func RunAsModularDriver(analyzer *analysis.Analyzer) {
	if os.Getenv(_asVettoolEnvVar) != "" {
		unitchecker.Main(analyzer) // calls os.Exit and never returns
	}
}

// Diagnostics returns the diagnostics reported by an analysistest run in a stable order.
func Diagnostics(gopath string, results []*analysistest.Result) []Diagnostic {
	var diagnostics []Diagnostic
	for _, result := range results {
		fset := result.Action.Package.Fset
		for _, diag := range result.Action.Diagnostics {
			position := fset.Position(diag.Pos)
			diagnostics = append(diagnostics,
				normalizeDiagnostic(gopath, position.Filename, position.Line, diag.Message))
		}
	}
	sortDiagnostics(diagnostics)
	return diagnostics
}

// RunModularAnalysis runs analyzer diagnostics on patterns using `go vet -vettool`. Unlike
// analysistest, this analyzes each package in a separate process and exchanges facts between
// packages through serialized fact files, matching modular drivers such as bazel/nogo.
//
// The vettool is the current test binary. The package's TestMain must call RunAsModularDriver
// with the analyzer under test to handle worker invocations.
func RunModularAnalysis(t *testing.T, gopath string, patterns ...string) []Diagnostic {
	t.Helper()

	exe, err := os.Executable()
	require.NoError(t, err)

	cmd := exec.Command("go", append([]string{"vet", "-vettool=" + exe, "-json"}, patterns...)...)
	// Mirror analysistest's environment for loading packages from testdata.
	cmd.Env = append(os.Environ(),
		_asVettoolEnvVar+"=1",
		"GOPATH="+gopath,
		"GO111MODULE=off",
		"GOPROXY=off",
		"GOFLAGS=",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	require.NoErrorf(t, cmd.Run(), "go vet -vettool: %s", stderr.String())

	// `go vet -json` writes a stream of per-package JSON objects
	// (package path -> analyzer -> diagnostics) and exits 0 even with findings. Depending on
	// the Go version, the stream may be written to stdout or stderr; stderr also prefixes each
	// package's object with a "# package/path" heading.
	var diagnostics []Diagnostic
	output := stripVetPackageHeadings(stdout.String() + "\n" + stderr.String())
	decoder := json.NewDecoder(strings.NewReader(output))
	for {
		var result map[string]map[string][]struct {
			Posn    string `json:"posn"`
			Message string `json:"message"`
		}
		if err := decoder.Decode(&result); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("decode go vet output: %s\noutput: %s", err, output)
		}
		for _, analyzers := range result {
			for _, diags := range analyzers {
				for _, diag := range diags {
					file, line, err := parseVetPosition(diag.Posn)
					require.NoError(t, err)
					diagnostics = append(diagnostics,
						normalizeDiagnostic(gopath, file, line, diag.Message))
				}
			}
		}
	}
	sortDiagnostics(diagnostics)
	return diagnostics
}

func stripVetPackageHeadings(output string) string {
	lines := strings.Split(output, "\n")
	lines = slices.DeleteFunc(lines, func(line string) bool {
		return strings.HasPrefix(line, "# ")
	})
	return strings.Join(lines, "\n")
}

func normalizeDiagnostic(gopath, file string, line int, message string) Diagnostic {
	return Diagnostic{
		File:    normalizeFile(gopath, file),
		Line:    line,
		Message: _sourceColumnRegexp.ReplaceAllString(message, "$1"),
	}
}

func normalizeFile(gopath, file string) string {
	srcRoot := filepath.Join(gopath, "src")
	file = filepath.Clean(file)
	if rel, err := filepath.Rel(srcRoot, file); err == nil && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	if filepath.IsAbs(file) {
		return filepath.ToSlash(file)
	}

	// A diagnostic reported while analyzing one package may point into another package. Modular
	// drivers can reduce that cross-package filename to a relative suffix, so resolve it against
	// the analysistest GOPATH when the suffix identifies exactly one testdata file.
	var matches []string
	_ = filepath.WalkDir(srcRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		pathSlash := filepath.ToSlash(path)
		if strings.HasSuffix(pathSlash, "/"+filepath.ToSlash(file)) {
			matches = append(matches, path)
		}
		return nil
	})
	if len(matches) == 1 {
		rel, err := filepath.Rel(srcRoot, matches[0])
		if err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(file)
}

func parseVetPosition(position string) (string, int, error) {
	columnSeparator := strings.LastIndexByte(position, ':')
	if columnSeparator < 0 {
		return "", 0, fmt.Errorf("malformed position %q", position)
	}
	lineSeparator := strings.LastIndexByte(position[:columnSeparator], ':')
	if lineSeparator < 0 {
		return "", 0, fmt.Errorf("malformed position %q", position)
	}
	line, err := strconv.Atoi(position[lineSeparator+1 : columnSeparator])
	if err != nil {
		return "", 0, fmt.Errorf("parse line from position %q: %w", position, err)
	}
	return position[:lineSeparator], line, nil
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.Slice(diagnostics, func(i, j int) bool {
		left, right := diagnostics[i], diagnostics[j]
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Message < right.Message
	})
}
