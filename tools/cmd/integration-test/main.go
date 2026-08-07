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
func CompareDiagnostics(truth map[Position][]*regexp.Regexp, collected map[Position][]string) error {
	remaining := make(map[Position][]*regexp.Regexp, len(truth))
	for pos, wants := range truth {
		remaining[pos] = append([]*regexp.Regexp(nil), wants...)
	}

	var errs []error
	for pos, diagnostics := range collected {
		wants := remaining[pos]
	diagnostic:
		for _, got := range diagnostics {
			for i, want := range wants {
				if !want.MatchString(got) {
					continue
				}
				wants[i] = wants[len(wants)-1]
				wants = wants[:len(wants)-1]
				continue diagnostic
			}
			if len(wants) == 0 {
				errs = append(errs, fmt.Errorf(
					"unexpected diagnostic at %s:%d:\n\tgot: %q",
					pos.Filename, pos.Line, got))
				continue
			}
			errs = append(errs, fmt.Errorf(
				"diagnostic mismatch at %s:%d:\n\twant one of: %q\n\tgot: %q",
				pos.Filename, pos.Line, wants, got))
		}
		remaining[pos] = wants
	}

	for pos, wants := range remaining {
		for _, want := range wants {
			errs = append(errs, fmt.Errorf(
				"missing diagnostic at %s:%d:\n\twant: %q",
				pos.Filename, pos.Line, want))
		}
	}

	sort.Slice(errs, func(i, j int) bool {
		return errs[i].Error() < errs[j].Error()
	})
	return errors.Join(errs...)
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

	tempRoot, err := os.MkdirTemp("", "nilaway-integration-test")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, os.RemoveAll(tempRoot))
	}()

	dir, err := PrepareTestProject(wd, tempRoot)
	if err != nil {
		return fmt.Errorf("prepare test project: %w", err)
	}

	drivers := []Driver{
		&StandaloneDriver{},
		&GoVetDriver{},
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
		expected := truths
		if _, ok := driver.(*GoVetDriver); ok {
			expected = make(map[Position][]*regexp.Regexp, len(truths))
			for pos, wants := range truths {
				expected[pos] = wants
			}
			// TODO: Remove these suppressions once incremental fact exporting is fixed in NilAway.
			// go vet cannot report diagnostics positioned in an imported package that are
			// discovered only while analyzing an importing package.
			for pos := range expected {
				switch filepath.ToSlash(filepath.Dir(pos.Filename)) {
				case "methodimplementation/multipackage/packageB", "nolint/upstream":
					delete(expected, pos)
				}
			}
		}
		if err := CompareDiagnostics(expected, collected); err != nil {
			fmt.Println("FAILED")
			return fmt.Errorf("diagnostics mismatch: \n%w", err)
		}
		fmt.Println("PASSED")
		var count int
		for _, diagnostics := range collected {
			count += len(diagnostics)
		}
		fmt.Printf("\t%d diagnostics matched\n", count)
	}

	return nil
}

func main() {
	if err := Run(); err != nil {
		fmt.Printf("FAILED: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("PASSED")
}
