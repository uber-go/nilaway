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

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// GolangCILintDriver implements Driver for running NilAway via golangci-lint.
type GolangCILintDriver struct{}

// Run runs NilAway via golangci-lint on the test project and returns the diagnostics.
func (d *GolangCILintDriver) Run(dir string) (diagnostics map[Position]string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Create a temporary directory to host the custom gcl binary.
	tempDir, err := os.MkdirTemp("", "golangci-lint-integration-test")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { err = errors.Join(err, os.RemoveAll(tempDir)) }()

	// Read GCLVersion from .golangci.version
	versionBytes, err := os.ReadFile(".golangci.version")
	if err != nil {
		return nil, fmt.Errorf("failed to read golangci.version: %w", err)
	}
	gclVersion := "v" + strings.TrimSpace(string(versionBytes))

	// Instantiate the template with the version and write it to the temp dir for building the
	// custom gcl binary.
	templateContent, err := os.ReadFile(filepath.Join(dir, ".custom-gcl.template.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}
	tmpl, err := template.New("custom-gcl").Parse(string(templateContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"GCLVersion": gclVersion, "NilAwayPath": cwd}); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, ".custom-gcl.yaml"), buf.Bytes(), 0644); err != nil {
		return nil, fmt.Errorf("failed to write custom-gcl.yaml: %w", err)
	}

	// Install the bootstrap gcl and build the custom binary.
	cmd := exec.Command("make", "install-golangci-lint")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to install golangci-lint: %w: %s", err, string(out))
	}
	cmd = exec.Command(filepath.Join(cwd, "bin", "golangci-lint"), "custom")
	cmd.Dir = tempDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to build the custom golangci-lint: %w: %s", err, string(out))
	}

	// Run the custom-gcl to collect NilAway diagnostics.
	diagnosticFile := filepath.Join(tempDir, "diagnostics.json")
	cmd = exec.Command(filepath.Join(tempDir, "custom-gcl"), "run", "--output.json.path", diagnosticFile, "./...")
	cmd.Dir = dir
	// golangci-lint exits with status 1 when it finds issues, which is expected.
	var exitErr *exec.ExitError
	if out, err := cmd.CombinedOutput(); err != nil && !(errors.As(err, &exitErr) && exitErr.ExitCode() == 1) {
		return nil, fmt.Errorf("failed to run custom-gcl: %w: %s", err, string(out))
	}

	data, err := os.ReadFile(diagnosticFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read diagnostics file: %w", err)
	}
	return parseGolangCILintOutput(dir, data)
}

func parseGolangCILintOutput(dir string, output []byte) (map[Position]string, error) {
	if len(output) == 0 {
		return map[Position]string{}, nil
	}

	var result struct {
		Issues []struct {
			FromLinter string   `json:"FromLinter"`
			Text       string   `json:"Text"`
			Pos        Position `json:"Pos"`
		} `json:"Issues"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse golangci-lint output: %w", err)
	}

	diagnostics := make(map[Position]string)
	for _, issue := range result.Issues {
		if issue.FromLinter != "nilaway" {
			continue
		}
		// The file names from golangci-lint are relative, so here we attach the dir prefix for comparisons.
		issue.Pos.Filename = filepath.Join(dir, issue.Pos.Filename)
		diagnostics[issue.Pos] = issue.Text
	}

	return diagnostics, nil
}
