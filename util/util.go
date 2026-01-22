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

// Package util implements utility functions for AST and types.
package util

import (
	"fmt"
	"regexp"
)

var codeReferencePattern = regexp.MustCompile("\\`(.*?)\\`")
var pathPattern = regexp.MustCompile(`"(.*?)"`)
var nilabilityPattern = regexp.MustCompile(`([\(|^\t](?i)(found\s|must\sbe\s)(nilable|nonnil)[\)]?)`)

// PrettyPrintErrorMessage is used in error reporting to post process and pretty print the output with colors
func PrettyPrintErrorMessage(msg string) string {
	// TODO: below string parsing should not be required after  is implemented
	errorStr := fmt.Sprintf("\x1b[%dm%s\x1b[0m", 31, "error: ")      // red
	codeStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 95, "`${1}`")    // magenta
	pathStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 36, "${1}")      // cyan
	nilabilityStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 1, "${1}") // bold

	msg = nilabilityPattern.ReplaceAllString(msg, nilabilityStr)
	msg = codeReferencePattern.ReplaceAllString(msg, codeStr)
	msg = pathPattern.ReplaceAllString(msg, pathStr)
	msg = errorStr + msg
	return msg
}
