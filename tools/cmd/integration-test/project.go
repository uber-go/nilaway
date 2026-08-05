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
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"golang.org/x/mod/modfile"
)

//go:embed .golangci.yaml
var golangCIConfig string

//go:embed go.mod.template
var goModTemplate string

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
	goVersion, err := RepositoryGoVersion(repoRoot)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("go.mod").Parse(goModTemplate)
	if err != nil {
		return "", fmt.Errorf("parse go.mod template: %w", err)
	}
	var projectGoMod bytes.Buffer
	if err := tmpl.Execute(&projectGoMod, map[string]string{"GoVersion": goVersion}); err != nil {
		return "", fmt.Errorf("execute go.mod template: %w", err)
	}
	stubsGoMod := fmt.Sprintf("module stubs\n\ngo %s\n", goVersion)
	files := map[string]string{
		filepath.Join(projectDir, "go.mod"):         projectGoMod.String(),
		filepath.Join(stubsDir, "go.mod"):           stubsGoMod,
		filepath.Join(projectDir, ".golangci.yaml"): golangCIConfig,
	}
	for path, contents := range files {
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			return "", fmt.Errorf("write %q: %w", path, err)
		}
	}

	return projectDir, nil
}

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

// RepositoryGoVersion returns the Go version declared by the repository's go.mod.
func RepositoryGoVersion(repoRoot string) (string, error) {
	path := filepath.Join(repoRoot, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read repository go.mod: %w", err)
	}
	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return "", fmt.Errorf("parse repository go.mod: %w", err)
	}
	if file.Go == nil {
		return "", errors.New("repository go.mod has no go directive")
	}
	return file.Go.Version, nil
}
