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

// Package nilawaytest implements utility functions for tests.
package nilawaytest

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// FindExpectedValues inspects test files and gathers expected values' comment strings
func FindExpectedValues(pass *analysis.Pass, expectedPrefix string) map[ast.Node][]string {
	results := make(map[ast.Node][]string)

	for _, file := range pass.Files {

		// Store a mapping between single comment's line number to its text.
		comments := make(map[int]string)
		for _, group := range file.Comments {
			if len(group.List) != 1 {
				continue
			}
			comment := group.List[0]
			comments[pass.Fset.Position(comment.Pos()).Line] = comment.Text
		}

		// Now, find all nodes of interest, such as *ast.FuncLit and *ast.FuncDecl, and find their comment.
		ast.Inspect(file, func(node ast.Node) bool {
			isExpectedNode := false
			switch node.(type) {
			case *ast.FuncLit, *ast.FuncDecl:
				isExpectedNode = true
			}
			if !isExpectedNode {
				return true
			}

			text, ok := comments[pass.Fset.Position(node.Pos()).Line]
			if !ok {
				// It is ok to not leave annotations for a node - it simply does not use
				// any closure variables. We still need to traverse further since there could be
				// comments for nested func lit nodes.
				return true
			}

			// Trim the trailing slashes and extra spaces and extract the set of expected values.
			text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
			text = strings.TrimSpace(strings.TrimPrefix(text, expectedPrefix))
			// If no expected values are written after the `expectedPrefix`, we simply ignore it.
			results[node] = nil
			if len(text) != 0 {
				results[node] = strings.Split(text, " ")
			}
			return true
		})
	}

	return results
}
