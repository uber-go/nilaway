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

// Package tokenhelper hosts helper functions that enhance the `token` package (e.g., around
// position and file path formatting etc.).
package tokenhelper

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

var _cwd, _cwdErr = os.Getwd()

// RelToCwd returns the relative path of the given filename with respect to the current
// working directory (retrieved during initialization). If the filename is not a child of
// the current working directory, it returns the filename itself.
func RelToCwd(filename string) string {
	if _cwdErr != nil {
		panic("failed to get current working directory: " + _cwdErr.Error())
	}
	rel, err := filepath.Rel(_cwd, filename)
	if err == nil {
		return rel
	}
	return filename
}

// Converse returns the converse of the given token. It panics if the token is not a valid comparison.
func Converse(t token.Token) token.Token {
	switch t {
	case token.EQL:
		return token.EQL
	case token.NEQ:
		return token.NEQ
	case token.LSS:
		return token.GTR
	case token.GTR:
		return token.LSS
	case token.LEQ:
		return token.GEQ
	case token.GEQ:
		return token.LEQ
	default:
		panic(fmt.Sprintf("unrecognized token %q has no known converse", t.String()))
	}
}

// Inverse returns the inverse of the given token. It panics if the token is not a valid comparison.
func Inverse(t token.Token) token.Token {
	switch t {
	case token.EQL:
		return token.NEQ
	case token.NEQ:
		return token.EQL
	case token.LSS:
		return token.GEQ
	case token.GTR:
		return token.LEQ
	case token.LEQ:
		return token.GTR
	case token.GEQ:
		return token.LSS
	default:
		panic(fmt.Sprintf("unrecognized token %q has no known inverse", t.String()))
	}
}

// PortionAfterSep returns the suffix of the passed string `input` containing at most `occ` occurrences
// of the separator `sep`
func PortionAfterSep(input, sep string, occ int) string {
	splits := strings.Split(input, sep)
	n := len(splits)
	if n <= occ+1 {
		return input // input contains at most `occ` occurrences of `sep`
	}
	out := ""
	for i := n - (1 + occ); i < n; i++ {
		if len(out) > 0 {
			out += sep
		}
		out += splits[i]
	}
	return out
}
