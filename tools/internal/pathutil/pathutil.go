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

// Package pathutil provides helpers for resolving filesystem paths.
package pathutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRoot returns the absolute path of the root of the git repository that
// contains the current working directory. The path is normalized so that the
// result is consistent across platforms (e.g. Windows, where git reports
// forward-slash paths while os.Getwd reports backslash paths).
func GitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get root of git repository: %w", err)
	}

	return filepath.Abs(filepath.FromSlash(strings.TrimSpace(string(out))))
}

// WorkingDirectory returns the normalized absolute path of the current working directory.
func WorkingDirectory() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	return filepath.Abs(wd)
}
