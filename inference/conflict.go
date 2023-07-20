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

package inference

import (
	"fmt"
	"go/token"

	"go.uber.org/nilaway/annotation"
	"golang.org/x/tools/go/analysis"
)

// For concise error reporting, conflicts are grouped by their nilness root cause, i.e., by their producers. Thus, the producer
// forms the 'key' to identify a conflict group.

// groupKey stores the type of the key representing a conflict group.
// (Note that the type is currently set to any since the producer types of singleAssertionConflict and overconstrainedConflict
// are not the same. Consolidating the two types of conflicts will help solve this problem (tracked in .)
type groupKey any

// conflictGrouping stores groups of conflicts represented by their keys in a map. It also provides methods to add conflicts
// to the groups and to generate diagnostics from the groups.
type conflictGrouping struct {
	groupMap map[groupKey][]conflict
}

// newConflictGrouping constructs and returns a pointer to conflictGrouping.
func newConflictGrouping() *conflictGrouping {
	return &conflictGrouping{
		groupMap: make(map[groupKey][]conflict),
	}
}

// addConflict adds a conflict to a conflict group identified by the conflict's key.
func (t *conflictGrouping) addConflict(conflict conflict) {
	key := conflict.key()
	t.groupMap[key] = append(t.groupMap[key], conflict)
}

// diagnostics converts conflict groups to a list of analysis.Diagnostic for reporting.
func (t *conflictGrouping) diagnostics() []analysis.Diagnostic {
	var diagnostics []analysis.Diagnostic
	for _, conflicts := range t.groupMap {
		site := conflicts[0].getSiteMessage()
		producer := conflicts[0].getProducerMessage()
		consumer := ""
		for _, c := range conflicts {
			consumer += c.getConsumerMessage()
		}

		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:     conflicts[0].getPosition(),
			Message: fmt.Sprintf("%s%s%s", site, producer, consumer),
		})
	}
	return diagnostics
}

// A conflict indicates a conflict in inferred value for an annotation site found by the inference engine.
type conflict interface {
	getSiteMessage() string
	getPosition() token.Pos
	getProducerMessage() string
	getConsumerMessage() string
	key() groupKey
}

// A singleAssertionConflict represents a conflict of a single assertion because its producer was
// already fixed to produce nilable, and its consumer was already fixed to consume nonnil.
// This could happen for some silly code like `var x X = nil; x.f` that does not depend at all on
// annotations. A more complicated example of such conflict involves flows across different source
// files in a package: the field `f` of some value of type `T` in one source file written with
// `nil` and the same field `f` of `T` passed to a deeper field access without nil check in a
// different source file.
type singleAssertionConflict struct {
	trigger         primitiveFullTrigger
	originalTrigger annotation.FullTrigger
}

// newSingleAssertionConflict constructs and returns a singleAssertionConflict.
func newSingleAssertionConflict(pass *analysis.Pass, trigger annotation.FullTrigger) *singleAssertionConflict {
	return &singleAssertionConflict{
		trigger:         fullTriggerAsPrimitive(pass, trigger),
		originalTrigger: trigger,
	}
}

// getSiteMessage returns the message for the site of the conflict. For a singleAssertionConflict, this is an empty string.
func (t *singleAssertionConflict) getSiteMessage() string {
	return ""
}

// getPosition returns the source code position of the conflict.
func (t *singleAssertionConflict) getPosition() token.Pos {
	return t.trigger.Pos
}

// getProducerMessage returns the error message to be reported for the producer part of the conflict.
func (t *singleAssertionConflict) getProducerMessage() string {
	return fmt.Sprintf(" Value %s (definitely nilable)", t.trigger.ProducerRepr)
}

// getConsumerMessage returns the error message to be reported for the consumer part of the conflict.
func (t *singleAssertionConflict) getConsumerMessage() string {
	return fmt.Sprintf(" and is %s (must be nonnil)", t.trigger.ConsumerRepr)
}

// key returns the key of the conflict group that this conflict belongs to.
func (t *singleAssertionConflict) key() groupKey {
	return t.originalTrigger.Producer
}

// An overconstrainedConflict represents a local annotation site that was constrained by two different
// chains of assertions to be both nilable (true) and nonnil (false). It encodes the overconstrained
// site, and the reason that the site had to be true and had to be false, as ExplainedBools.
type overconstrainedConflict struct {
	site             primitiveSite
	trueExplanation  ExplainedBool
	falseExplanation ExplainedBool
}

// newOverconstrainedConflict  constructs and returns an overconstrainedConflict.
func newOverconstrainedConflict(site primitiveSite, trueExplanation, falseExplanation ExplainedBool) *overconstrainedConflict {
	return &overconstrainedConflict{
		site:             site,
		trueExplanation:  trueExplanation,
		falseExplanation: falseExplanation,
	}
}

// getSiteMessage returns the message for the site of the conflict.
func (t *overconstrainedConflict) getSiteMessage() string {
	return fmt.Sprintf(" Annotation on %s overconstrained:", t.site.String())
}

// getPosition returns the source code position of the conflict.
func (t *overconstrainedConflict) getPosition() token.Pos {
	return t.site.Pos
}

// getProducerMessage returns the error message to be reported for the producer part of the conflict.
func (t *overconstrainedConflict) getProducerMessage() string {
	return fmt.Sprintf("\n\tMust be %s", t.trueExplanation.String())
}

// getConsumerMessage returns the error message to be reported for the consumer part of the conflict.
func (t *overconstrainedConflict) getConsumerMessage() string {
	return fmt.Sprintf("\n\t\tAND\n\tMust be %s", t.falseExplanation.String())
}

// key returns the key of the conflict group that this conflict belongs to.
func (t *overconstrainedConflict) key() groupKey {
	return t.trueExplanation
}
