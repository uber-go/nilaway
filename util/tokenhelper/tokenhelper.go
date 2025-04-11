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

// Package tokenhelper hosts helper functions that enhance the `token` package (e.g., around
// position and file path formatting etc.).
package tokenhelper

import (
	"os"
	"path/filepath"
)

var _cwd = func() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic("failed to get current working directory: " + err.Error())
	}
	return cwd
}()

// RelToCwd returns the relative path of the given filename with respect to the current
// working directory (retrieved during initialization). If the filename is not a child of
// the current working directory, it returns the filename itself.
func RelToCwd(filename string) string {
	rel, err := filepath.Rel(_cwd, filename)
	if err != nil {
		return rel
	}
	return filename
}
