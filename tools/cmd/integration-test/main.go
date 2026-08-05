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

// Package main implements the integration test framework for checking NilAway with different
// analyzer drivers. It runs each driver on the analysistest corpus under
// testdata/src/go.uber.org and compares the reported diagnostics with its want comments.
package main

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/scanner"
)

// Position represents a line position in a file.
type Position struct {
	Filename string
	Line     int
}

// Driver is the analyzer driver interface that runs NilAway on the test project.
type Driver interface {
	// Run runs NilAway on the test project specified by dir and returns the diagnostics reported
	// by NilAway (in a map from Position to the diagnostic message).
	Run(dir string) (map[Position][]string, error)
}

// CollectGroundTruths collects diagnostics specified by want comments in Go files under dir.
func CollectGroundTruths(dir string) (map[Position][]*regexp.Regexp, error) {
	truths := make(map[Position][]*regexp.Regexp)
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %q: %w", path, err)
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				text := strings.TrimPrefix(comment.Text, "//")
				if text == comment.Text {
					text = strings.TrimPrefix(text, "/*")
					text = strings.TrimSuffix(text, "*/")
				}
				text = strings.TrimSpace(text)
				rest, ok := strings.CutPrefix(text, "want")
				if !ok {
					continue
				}

				pos := fset.Position(comment.Pos())
				relative, err := filepath.Rel(dir, pos.Filename)
				if err != nil {
					return fmt.Errorf("make %q relative to %q: %w", pos.Filename, dir, err)
				}
				key := Position{
					Filename: filepath.ToSlash(relative),
					Line:     pos.Line,
				}

				var scanErr string
				sc := new(scanner.Scanner).Init(strings.NewReader(rest))
				sc.Error = func(_ *scanner.Scanner, message string) {
					scanErr = message
				}
				sc.Mode = scanner.ScanStrings | scanner.ScanRawStrings
				for tok := sc.Scan(); tok != scanner.EOF; tok = sc.Scan() {
					if tok != scanner.String && tok != scanner.RawString {
						return fmt.Errorf(
							"%s:%d: got %s in want comment, expected quoted string",
							pos.Filename, pos.Line, scanner.TokenString(tok))
					}
					pattern, err := strconv.Unquote(sc.TokenText())
					if err != nil {
						return fmt.Errorf("%s:%d: unquote pattern: %w", pos.Filename, pos.Line, err)
					}
					rx, err := regexp.Compile(pattern)
					if err != nil {
						return fmt.Errorf("%s:%d: compile pattern %q: %w", pos.Filename, pos.Line, pattern, err)
					}
					truths[key] = append(truths[key], rx)
				}
				if scanErr != "" {
					return fmt.Errorf("%s:%d: scan want comment: %s", pos.Filename, pos.Line, scanErr)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk test corpus: %w", err)
	}
	return truths, nil
}

// CompareDiagnostics compares the ground truths with the collected diagnostics and returns a
// joined error containing the mismatched/missing/unexpected diagnostics (or nil if none).
func CompareDiagnostics(
	truth map[Position][]*regexp.Regexp,
	collected map[Position][]string,
) (err error) {
	positionSet := make(map[Position]struct{}, len(truth)+len(collected))
	for pos := range truth {
		positionSet[pos] = struct{}{}
	}
	for pos := range collected {
		positionSet[pos] = struct{}{}
	}
	positions := make([]Position, 0, len(positionSet))
	for pos := range positionSet {
		positions = append(positions, pos)
	}
	sort.Slice(positions, func(i, j int) bool {
		if positions[i].Filename != positions[j].Filename {
			return positions[i].Filename < positions[j].Filename
		}
		return positions[i].Line < positions[j].Line
	})

	for _, pos := range positions {
		wants := truth[pos]
		gots := collected[pos]
		unmatchedWants, unmatchedGots := unmatchedDiagnostics(wants, gots)
		for _, got := range unmatchedGots {
			if len(unmatchedWants) == 0 {
				err = errors.Join(err, fmt.Errorf(
					"unexpected diagnostic at %s:%d:\n\tgot: %q",
					pos.Filename, pos.Line, got))
				continue
			}
			err = errors.Join(err, fmt.Errorf(
				"diagnostic mismatch at %s:%d:\n\twant one of: %s\n\tgot: %q",
				pos.Filename, pos.Line, formatPatterns(unmatchedWants), got))
		}
		for _, want := range unmatchedWants {
			err = errors.Join(err, fmt.Errorf(
				"missing diagnostic at %s:%d:\n\twant: %q",
				pos.Filename, pos.Line, want))
		}
	}

	return err
}

// unmatchedDiagnostics returns the expected patterns and reported messages that cannot be paired.
// It uses bipartite matching so overlapping patterns do not make the result order-dependent.
func unmatchedDiagnostics(wants []*regexp.Regexp, gots []string) ([]*regexp.Regexp, []string) {
	if wants == nil {
		wants = []*regexp.Regexp{}
	}
	if gots == nil {
		gots = []string{}
	}

	wantToGot := make([]int, len(wants))
	for i := range wantToGot {
		wantToGot[i] = -1
	}

	var match func(int, []bool) bool
	match = func(got int, seen []bool) bool {
		for want, rx := range wants {
			if seen[want] || !rx.MatchString(gots[got]) {
				continue
			}
			seen[want] = true
			if wantToGot[want] == -1 || match(wantToGot[want], seen) {
				wantToGot[want] = got
				return true
			}
		}
		return false
	}

	gotMatched := make([]bool, len(gots))
	for got := range gots {
		if match(got, make([]bool, len(wants))) {
			gotMatched[got] = true
		}
	}

	var unmatchedWants []*regexp.Regexp
	for want, got := range wantToGot {
		if got == -1 {
			unmatchedWants = append(unmatchedWants, wants[want])
		}
	}
	var unmatchedGots []string
	for got, matched := range gotMatched {
		if !matched {
			unmatchedGots = append(unmatchedGots, gots[got])
		}
	}
	return unmatchedWants, unmatchedGots
}

func formatPatterns(patterns []*regexp.Regexp) string {
	formatted := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		formatted = append(formatted, strconv.Quote(pattern.String()))
	}
	return strings.Join(formatted, ", ")
}

func countDiagnostics(diagnostics map[Position][]string) int {
	var total int
	for _, messages := range diagnostics {
		total += len(messages)
	}
	return total
}

func positionInProject(dir, filename string, line int) (Position, bool, error) {
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(dir, filename)
	}
	relative, err := filepath.Rel(dir, filename)
	if err != nil {
		return Position{}, false, fmt.Errorf("make diagnostic path %q relative to %q: %w", filename, dir, err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return Position{}, false, nil
	}
	return Position{Filename: filepath.ToSlash(relative), Line: line}, true, nil
}

// Run runs the integration test.
func Run() (err error) {
	// Make sure we are at the root of the git repository.
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return fmt.Errorf("get root of git repository: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if dir := strings.TrimSpace(string(out)); dir != wd {
		return fmt.Errorf("not at the root of the git repository: %q != %q", dir, wd)
	}

	sourceDir := filepath.Join(wd, "testdata", "src", "go.uber.org")
	truths, err := CollectGroundTruths(sourceDir)
	if err != nil {
		return fmt.Errorf("collect want strings: %w", err)
	}

	dir, cleanup, err := prepareTestProject(wd)
	if err != nil {
		return fmt.Errorf("prepare test project: %w", err)
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()

	drivers := []Driver{
		&StandaloneDriver{},
		&GolangCILintDriver{},
	}
	for _, driver := range drivers {
		name := reflect.TypeOf(driver).Elem().Name()
		fmt.Printf("--- Running integration tests using %q driver...", name)
		collected, err := driver.Run(dir)
		if err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("%q driver: %w", name, err)
		}
		if err := CompareDiagnostics(truths, collected); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("diagnostics mismatch: \n%w", err)
		}
		fmt.Println("PASSED")
		fmt.Printf("\t%d diagnostics matched\n", countDiagnostics(collected))
	}

	return nil
}

func prepareTestProject(repoRoot string) (_ string, cleanup func() error, err error) {
	tempRoot, err := os.MkdirTemp("", "nilaway-integration-test")
	if err != nil {
		return "", nil, fmt.Errorf("create temp directory: %w", err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(tempRoot))
		}
	}()

	projectDir := filepath.Join(tempRoot, "go.uber.org")
	stubsDir := filepath.Join(tempRoot, "stubs")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", nil, fmt.Errorf("create project directory: %w", err)
	}
	if err := os.MkdirAll(stubsDir, 0755); err != nil {
		return "", nil, fmt.Errorf("create stubs directory: %w", err)
	}
	if err := os.CopyFS(projectDir, os.DirFS(filepath.Join(repoRoot, "testdata", "src", "go.uber.org"))); err != nil {
		return "", nil, fmt.Errorf("copy go.uber.org test corpus: %w", err)
	}
	if err := os.CopyFS(stubsDir, os.DirFS(filepath.Join(repoRoot, "testdata", "src", "stubs"))); err != nil {
		return "", nil, fmt.Errorf("copy test stubs: %w", err)
	}
	if err := makeCorpusBuildable(projectDir); err != nil {
		return "", nil, err
	}

	goVersion, err := repositoryGoVersion(repoRoot)
	if err != nil {
		return "", nil, err
	}
	projectGoMod := fmt.Sprintf(
		"module go.uber.org\n\ngo %s\n\nrequire stubs v0.0.0\n\nreplace stubs => ../stubs\n",
		goVersion,
	)
	stubsGoMod := fmt.Sprintf("module stubs\n\ngo %s\n", goVersion)
	files := map[string]string{
		filepath.Join(projectDir, "go.mod"):                    projectGoMod,
		filepath.Join(stubsDir, "go.mod"):                      stubsGoMod,
		filepath.Join(projectDir, ".custom-gcl.template.yaml"): customGCLTemplate,
		filepath.Join(projectDir, ".golangci.yaml"):            golangCIConfig,
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			return "", nil, fmt.Errorf("write %q: %w", path, err)
		}
	}

	return projectDir, func() error { return os.RemoveAll(tempRoot) }, nil
}

// makeCorpusBuildable applies temporary compatibility shims needed by real Go build drivers.
// analysistest accepts bodyless function declarations and uses the print builtin with values that
// the compiler rejects; neither construct changes the nil flows under test.
func makeCorpusBuildable(projectDir string) error {
	printShimPackages := map[string]string{
		"annotationparse":    "annotationparse",
		"multipleassignment": "multipleassignment",
		"nilcheck":           "nilcheck",
		"trustedfunc":        "trustedfunc",
	}
	for relativeDir, packageName := range printShimPackages {
		path := filepath.Join(projectDir, relativeDir, "integration_print.go")
		contents := fmt.Sprintf(`package %s

// nilable(args)
// nilable(args[])
func print(args ...any) {}
`, packageName)
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			return fmt.Errorf("write print compatibility shim %q: %w", path, err)
		}
	}

	replacements := []struct {
		path string
		old  string
		new  string
	}{
		{
			path: filepath.Join(projectDir, "globalvars", "globalvarinit.go"),
			old:  "func bodylessFunc()",
			new:  "func bodylessFunc() {}",
		},
		{
			path: filepath.Join(projectDir, "goquirks", "goquirks.go"),
			old:  "func external(v *int) *int",
			new:  "func external(v *int) *int { return v }",
		},
	}
	for _, replacement := range replacements {
		data, err := os.ReadFile(replacement.path)
		if err != nil {
			return fmt.Errorf("read %q: %w", replacement.path, err)
		}
		source := string(data)
		if count := strings.Count(source, replacement.old); count != 1 {
			return fmt.Errorf("replace %q in %q: found %d occurrences", replacement.old, replacement.path, count)
		}
		source = strings.Replace(source, replacement.old, replacement.new, 1)
		if err := os.WriteFile(replacement.path, []byte(source), 0644); err != nil {
			return fmt.Errorf("write %q: %w", replacement.path, err)
		}
	}
	return nil
}

func repositoryGoVersion(repoRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read repository go.mod: %w", err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			return fields[1], nil
		}
	}
	return "", errors.New("repository go.mod has no go directive")
}

func main() {
	if err := Run(); err != nil {
		fmt.Printf("FAILED: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("PASSED")
}
