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

package diagnostic

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"strings"

	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis"
)

type conflict struct {
	// position is the package-independent position where the conflict should be reported.
	position token.Position
	// flow stores nil flow from source to dereference point
	flow nilFlow
	// similarConflicts stores other conflicts that are similar to this one.
	similarConflicts []*conflict
}

func (c *conflict) String() string {
	// build string for similar conflicts (i.e., conflicts with the same nil path)
	similarConflictsString := ""
	if len(c.similarConflicts) > 0 {
		similarPos := make([]string, len(c.similarConflicts))
		for i, s := range c.similarConflicts {
			similarPos[i] = fmt.Sprintf("\"%s\"", s.flow.nonnilPath[len(s.flow.nonnilPath)-1].consumerPosition.String())
		}

		posString := strings.Join(similarPos[:len(similarPos)-1], ", ")
		if len(similarPos) > 1 {
			posString = posString + ", and "
		}
		posString = posString + similarPos[len(similarPos)-1]

		similarConflictsString = fmt.Sprintf("\n\n(Same nil source could also cause potential nil panic(s) at %d "+
			"other place(s): %s.)", len(c.similarConflicts), posString)
	}

	return fmt.Sprintf("Potential nil panic detected. Observed nil flow from "+
		"source to dereference point: %s%s\n", c.flow.String(), similarConflictsString)
}

func (c *conflict) addSimilarConflict(conflict conflict) {
	c.similarConflicts = append(c.similarConflicts, &conflict)
}

// groupConflicts groups conflicts with the same nil path together and update conflicts list.
func groupConflicts(allConflicts []conflict, pass *analysis.Pass, cwd string) []conflict {
	conflictsMap := make(map[string]int)  // key: nil path string, value: index in `allConflicts`
	indicesToIgnore := make(map[int]bool) // indices of conflicts to be ignored from `allConflicts`, since they are grouped with other conflicts

	for i, c := range allConflicts {
		key := pathString(c.flow.nilPath)

		// Handle the case of single assertion conflict separately
		if len(c.flow.nilPath) == 0 && len(c.flow.nonnilPath) == 1 {
			// This is the case of single assertion conflict. Use producer position and repr from the non-nil path as
			// the key, if present, else use the producer and consumer repr as a heuristic key to group conflicts.
			p := c.flow.nonnilPath[0]
			key = p.producerRepr + ";" + p.consumerRepr
			if p.producerPosition.IsValid() {
				key = p.producerPosition.String() + ": " + p.producerRepr
			} else {
				// The heuristic of using producer and consumer repr as key may not work perfectly, especially when the
				// error messages in two different functions are exactly the same. Consider the following example:
				// ```
				// 	func f1() {
				//		mp := make(map[int]*int)
				//		_ = *mp[0] // error message: "deep read from local variable `mp` lacking guarding; dereferenced"
				// 	}
				//
				// 	func f2() {
				//		mp := make(map[int]*int)
				//		_ = *mp[0] // error message: "deep read from local variable `mp` lacking guarding; dereferenced"
				// 	}
				// ```
				// Here, the two error messages are exactly the same, but they should not be grouped together as they are
				// from different functions. To handle such cases, we prepend the enclosing function name to the key.
				conf := pass.ResultOf[config.Analyzer].(*config.Config)
				for _, file := range pass.Files {
					// `fileName` stores the complete file path relative to the current working directory
					fileName := pass.Fset.Position(file.FileStart).Filename
					if fn, err := filepath.Rel(cwd, fileName); err == nil {
						fileName = fn
					}
					// Check if the file is in scope and the conflict position is in the same file
					if !conf.IsFileInScope(file) || fileName != c.position.Filename {
						continue
					}
					for _, decl := range file.Decls {
						// Check if the conflict position falls within the function's position range. If so, update the key to
						// include the function name, and end the traversal.
						if fd, ok := decl.(*ast.FuncDecl); ok {
							functionStart := pass.Fset.Position(fd.Pos()).Offset
							functionEnd := pass.Fset.Position(fd.End()).Offset
							if c.position.Offset >= functionStart && c.position.Offset <= functionEnd {
								key = fd.Name.Name + ":" + key
								break
							}
						}
					}
				}
			}
		}

		if existingConflictIndex, ok := conflictsMap[key]; ok {
			// Grouping condition satisfied. Add new conflict to `similarConflicts` in `existingConflict`, and update groupedConflicts map
			allConflicts[existingConflictIndex].addSimilarConflict(c)
			indicesToIgnore[i] = true
		} else {
			conflictsMap[key] = i
		}
	}

	// update groupedConflicts list with grouped groupedConflicts
	var groupedConflicts []conflict
	for i, c := range allConflicts {
		if _, ok := indicesToIgnore[i]; !ok {
			groupedConflicts = append(groupedConflicts, c)
		}
	}
	return groupedConflicts
}
