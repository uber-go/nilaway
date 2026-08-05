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

package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed .custom-gcl.template.yaml
var customGCLTemplate string

//go:embed .golangci.yaml
var golangCIConfig string

// PositionInProject converts a diagnostic position to a project-relative position.
func PositionInProject(dir, filename string, line int) (Position, error) {
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(dir, filename)
	}
	relative, err := filepath.Rel(dir, filename)
	if err != nil {
		return Position{}, fmt.Errorf("make diagnostic path %q relative to %q: %w", filename, dir, err)
	}
	return Position{Filename: filepath.ToSlash(relative), Line: line}, nil
}

// PrepareTestProject creates a buildable copy of the integration test project.
func PrepareTestProject(repoRoot, tempRoot string) (string, error) {
	projectDir := filepath.Join(tempRoot, "go.uber.org")
	stubsDir := filepath.Join(tempRoot, "stubs")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("create project directory: %w", err)
	}
	if err := os.MkdirAll(stubsDir, 0755); err != nil {
		return "", fmt.Errorf("create stubs directory: %w", err)
	}
	if err := os.CopyFS(projectDir, os.DirFS(filepath.Join(repoRoot, "testdata", "src", "go.uber.org"))); err != nil {
		return "", fmt.Errorf("copy go.uber.org test corpus: %w", err)
	}
	if err := os.CopyFS(stubsDir, os.DirFS(filepath.Join(repoRoot, "testdata", "src", "stubs"))); err != nil {
		return "", fmt.Errorf("copy test stubs: %w", err)
	}
	if err := MakeCorpusBuildable(projectDir); err != nil {
		return "", err
	}

	goVersion, err := RepositoryGoVersion(repoRoot)
	if err != nil {
		return "", err
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
			return "", fmt.Errorf("write %q: %w", path, err)
		}
	}

	return projectDir, nil
}

// MakeCorpusBuildable applies temporary compatibility shims needed by real Go build drivers.
// analysistest accepts bodyless function declarations and uses the print builtin with values that
// the compiler rejects; neither construct changes the nil flows under test.
func MakeCorpusBuildable(projectDir string) error {
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

// RepositoryGoVersion returns the Go version declared by the repository's go.mod.
func RepositoryGoVersion(repoRoot string) (string, error) {
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
