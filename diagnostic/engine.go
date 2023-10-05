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
	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/inference"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// Engine is the main engine for generating diagnostics from conflicts.
type Engine struct {
	pass      *analysis.Pass
	conflicts []conflict
}

// NewEngine creates a new diagnostic engine.
func NewEngine(pass *analysis.Pass) *Engine {
	return &Engine{pass: pass}
}

// Diagnostics generates diagnostics from the internally-stored conflicts. The grouping parameter
// controls whether the conflicts with the same nil flow -- the part in the complete nil flow going
// from a nilable source point to the conflict point -- are grouped together for concise reporting.
func (e *Engine) Diagnostics(grouping bool) []analysis.Diagnostic {
	conflicts := e.conflicts
	if grouping {
		// group conflicts with the same nil path together for concise reporting
		conflicts = groupConflicts(e.conflicts)
	}

	// build diagnostics from conflicts
	diagnostics := make([]analysis.Diagnostic, 0, len(conflicts))
	for _, c := range conflicts {
		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:     c.pos,
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

	e.conflicts = append(e.conflicts, conflict{
		pos:  trigger.Consumer.Expr.Pos(),
		flow: flow,
	})
}

// AddOverconstraintConflict adds a new overconstraint conflict to the engine.
func (e *Engine) AddOverconstraintConflict(nilReason, nonnilReason inference.ExplainedBool) {
	c := conflict{}

	// Build nil path by traversing the inference graph from `nilReason` part of the overconstraint failure.
	// (Note that this traversal gives us a backward path from point of conflict to the source of nilability. Hence, we
	// must take this into consideration while printing the flow, which is currently being handled in `addNilPathNode()`.)
	for r := nilReason; r != nil; r = r.DeeperReason() {
		producer, consumer := r.TriggerReprs()
		// We have two cases here:
		// 1. No annotation present (i.e., full inference): we have producer and consumer explanations available; use them directly
		// 2: Annotation present (i.e., no inference): we construct the reason from the annotation string
		if producer != nil && consumer != nil {
			c.flow.addNilPathNode(producer, consumer)
		} else {
			c.flow.addNilPathNode(annotation.LocatedPrestring{
				Contained: r,
				Location:  util.TruncatePosition(e.pass.Fset.Position(r.Pos())),
			}, nil)
		}
	}

	// Build nonnil path by traversing the inference graph from `nonnilReason` part of the overconstraint failure.
	// (Note that this traversal is forward from the point of conflict to dereference. Hence, we don't need to make
	// any special considerations while printing the flow.)
	// Different from building the nil path above, here we also want to deduce the position where the error should be reported,
	// i.e., the point of dereference where the nil panic would occur. In NilAway's context this is the last node
	// in the non-nil path. Therefore, we keep updating `c.pos` until we reach the end of the non-nil path.
	for r := nonnilReason; r != nil; r = r.DeeperReason() {
		producer, consumer := r.TriggerReprs()
		// Similar to above, we have two cases here:
		// 1. No annotation present (i.e., full inference): we have producer and consumer explanations available; use them directly
		// 2: Annotation present (i.e., no inference): we construct the reason from the annotation string
		if producer != nil && consumer != nil {
			c.flow.addNonNilPathNode(producer, consumer)
			c.pos = r.Pos()
		} else {
			c.flow.addNonNilPathNode(annotation.LocatedPrestring{
				Contained: r,
				Location:  util.TruncatePosition(e.pass.Fset.Position(r.Pos())),
			}, nil)
			c.pos = r.Pos()
		}
	}

	e.conflicts = append(e.conflicts, c)
}
