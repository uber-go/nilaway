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

// Package diagnostic hosts the diagnostic engine, which is responsible for collecting the
// conflicts from annotation-based checks (no-infer mode) and/or inference (full-infer mode) and
// generating user-friendly diagnostics from those conflicts.
package diagnostic

import (
	"cmp"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"slices"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/inference"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// fileInfo bundles the token.File object and auxiliary information about it, e.g., whether it is
// a fake file (i.e., imported from archive), for uses in primitivizer.
type fileInfo struct {
	file   *token.File
	isFake bool
}

// Engine is the main engine for generating diagnostics from conflicts.
type Engine struct {
	pass      *analysis.Pass
	conflicts []conflict
	// files maps the file name (modulo the possible build-system prefix) to the token.File object
	// for faster lookup when converting correct upstream position back to local token.Pos for
	// reporting purposes.
	files map[string]fileInfo
	// cwd is the current working directory for trimming the file names to get truly package- and
	// build-system- (bazel for example adds a random sandbox prefix) independent positions.
	cwd string
}

// NewEngine creates a new diagnostic engine.
func NewEngine(pass *analysis.Pass) *Engine {
	// Find the current working directory (e.g., random sandbox prefix if using bazel) for trimming
	// the file names.
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("cannot get current working directory: %v", err))
	}

	// Iterate all files within the Fset (which includes upstream and current-package files), and
	// store the mapping between its file name (modulo the possible build-system prefix) and the
	// token.File object. This is needed for converting correct upstream position back to local
	// incorrect token.Pos for error reporting purposes. Also see
	// [inference.primitivizer.toPosition] for more detailed explanations.
	files := make(map[string]fileInfo)
	pass.Fset.Iterate(func(file *token.File) bool {
		name, err := filepath.Rel(cwd, file.Name())
		if err != nil {
			// For files that are not in the execroot (e.g., stdlib files start with "$GOROOT", and
			// upstream files that do not have the build-system prefix), we can simply use the
			// original file name.
			name = file.Name()
		}

		// The file will be fake (conceptually "\n" * 65535) if it is imported from archive. So we
		// check if there are any gaps between the line starts to determine if the file is fake.
		isFake := true
		prev := -1
		for _, pos := range file.Lines() {
			if prev != -1 && pos-prev > 1 {
				isFake = false
				break
			}
			prev = pos
		}
		files[name] = fileInfo{
			file:   file,
			isFake: isFake,
		}
		return true
	})

	return &Engine{pass: pass, files: files, cwd: cwd}
}

// Diagnostics generates diagnostics from the internally-stored conflicts. The grouping parameter
// controls whether the conflicts with the same nil flow -- the part in the complete nil flow going
// from a nilable source point to the conflict point -- are grouped together (under the first
// diagnostic) for concise reporting. The returned slice of diagnostics are sorted by file names
// and then offsets in the file.
func (e *Engine) Diagnostics(grouping bool) []analysis.Diagnostic {
	// First sort the conflicts by position such that similar conflicts are grouped under the
	// first diagnostic.
	slices.SortFunc(e.conflicts, func(a, b conflict) int {
		if n := cmp.Compare(a.position.Filename, b.position.Filename); n != 0 {
			return n
		}
		return cmp.Compare(a.position.Offset, b.position.Offset)
	})

	conflicts := e.conflicts
	if grouping {
		// Group conflicts with the same nil path together for concise reporting.
		conflicts = groupConflicts(e.conflicts, e.pass, e.cwd)
	}

	// Build diagnostics from conflicts.
	diagnostics := make([]analysis.Diagnostic, 0, len(conflicts))
	for _, c := range conflicts {
		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:     e.toPos(c.position),
			Message: c.String(),
		})
	}
	return diagnostics
}

// AddSingleAssertionConflict adds a new single assertion conflict to the engine.
func (e *Engine) AddSingleAssertionConflict(trigger annotation.FullTrigger) {
	producer, consumer := trigger.Prestrings(e.pass)
	flow := nilFlow{}
	flow.addNonNilPathNode(producer, consumer)

	position := e.pass.Fset.Position(trigger.Consumer.Expr.Pos())
	// Try to trim the build system prefix (i.e., the current working directory) from the position.
	// If NilAway is running in a driver that does not add such prefix, we will hit an error here,
	// but that is fine, and we just do not need to do anything.
	if filename, err := filepath.Rel(e.cwd, position.Filename); err == nil {
		position.Filename = filename
	}
	e.conflicts = append(e.conflicts, conflict{
		position: position,
		flow:     flow,
	})
}

// AddOverconstraintConflict adds a new overconstraint conflict to the engine.
func (e *Engine) AddOverconstraintConflict(nilReason, nonnilReason inference.ExplainedBool) {
	flow := nilFlow{}

	// Build nil path by traversing the inference graph from `nilReason` part of the overconstraint failure.
	// (Note that this traversal gives us a backward path from point of conflict to the source of nilability. Hence, we
	// must take this into consideration while printing the flow, which is currently being handled in `addNilPathNode()`.)
	for r := nilReason; r != nil; r = r.DeeperReason() {
		producer, consumer := r.TriggerReprs()
		// We have two cases here:
		// 1. No annotation present (i.e., full inference): we have producer and consumer explanations available; use them directly
		// 2: Annotation present (i.e., no inference): we construct the reason from the annotation string
		if producer != nil && consumer != nil {
			flow.addNilPathNode(producer, consumer)
		} else {
			flow.addNilPathNode(annotation.LocatedPrestring{
				Contained: r,
				Location:  util.TruncatePosition(r.Position()),
			}, nil)
		}
	}

	// Build nonnil path by traversing the inference graph from `nonnilReason` part of the overconstraint failure.
	// (Note that this traversal is forward from the point of conflict to dereference. Hence, we don't need to make
	// any special considerations while printing the flow.)
	// Different from building the nil path above, here we also want to deduce the position where the error should be reported,
	// i.e., the point of dereference where the nil panic would occur. In NilAway's context this is the last node
	// in the non-nil path. Therefore, we keep updating `c.pos` until we reach the end of the non-nil path.
	var reportPosition token.Position
	for r := nonnilReason; r != nil; r = r.DeeperReason() {
		producer, consumer := r.TriggerReprs()
		position := r.Position()
		// Similar to above, we have two cases here:
		// 1. No annotation present (i.e., full inference): we have producer and consumer explanations available; use them directly
		// 2: Annotation present (i.e., no inference): we construct the reason from the annotation string
		if producer != nil && consumer != nil {
			flow.addNonNilPathNode(producer, consumer)
			reportPosition = position
		} else {
			flow.addNonNilPathNode(annotation.LocatedPrestring{
				Contained: r,
				Location:  util.TruncatePosition(r.Position()),
			}, nil)
			reportPosition = position
		}
	}

	e.conflicts = append(e.conflicts, conflict{
		position: reportPosition,
		flow:     flow,
	})
}

// _fakeFileMaxLines is the maximum number of lines that the archive importer will add to a (fake)
// file when it imports a package. See [the importer code] for more details. We use this to create
// more fake files when necessary (see [primitivizer.sitePos]).
// [the importer code]: https://cs.opensource.google/go/x/tools/+/master:internal/gcimporter/bimport.go;l=34;bpv=0;bpt=1
const _fakeFileMaxLines = 64 * 1024

// toPos converts the token.Position back to a token.Pos that is relative to local Fset for
// reporting purposes _only_. Note that the input position could be obtained from facts or
// inference, so the position might not exist in the local Fset. In such cases, we pad the local
// Fset for correct reporting.
func (e *Engine) toPos(position token.Position) token.Pos {
	info, ok := e.files[position.Filename]
	if !ok {
		// For incremental build systems like bazel, the pass.Fset contains only the files in
		// current and _directly_ imported packages (see [gcexportdata] for more details). However,
		// analyzer facts are imported transitively from all imported packages, and NilAway is able
		// to operate across all those packages. As a result, if NilAway ever needs to report an
		// error on a file from a transitively imported package, we need to create a fake file in
		// the file set.
		// [gcexportdata]: https://pkg.go.dev/golang.org/x/tools/go/gcexportdata
		file := e.pass.Fset.AddFile(position.Filename, e.pass.Fset.Base(), _fakeFileMaxLines)
		// Set up fake lines for the fake file.
		fakeLines := make([]int, position.Line)
		for i := range fakeLines {
			fakeLines[i] = i
		}
		file.SetLines(fakeLines)
		info = fileInfo{file: file, isFake: true}
		e.files[position.Filename] = info
	}

	if info.isFake {
		// If the file is fake (imported from archive), it may not contain fake lines for unexported
		// objects (as an "optimization", see [importer code]). However, NilAway may report errors
		// on unexported objects due to multi-package inference. In such cases, we pad the file with
		// more fake lines.
		// [importer code]: https://cs.opensource.google/go/x/tools/+/refs/tags/v0.12.0:internal/gcimporter/bimport.go;l=36-69;drc=ad74ff6345e3663a8f1a4ba5c6e85d54a6fd5615
		if position.Line > info.file.LineCount() {
			// We are adding offsets to fake lines here, and offset == fake line number - 1. So we
			// can start from the current max line number to the desired line number - 1.
			for i := info.file.LineCount(); i < position.Line; i++ {
				info.file.AddLine(i)
			}
		}

		// For fake files, we can only report accurate line number but not column number.
		return info.file.LineStart(position.Line)
	}

	// For non-fake files, the position is accurate.
	return info.file.Pos(position.Offset)
}
