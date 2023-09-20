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
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

type conflict struct {
	pos              token.Pos   // stores position where the error should be reported (note that this field is used only within the current, and should NOT be exported)
	flow             nilFlow     // stores nil flow from source to dereference point
	similarConflicts []*conflict // stores other conflicts that are similar to this one
}

type nilFlow struct {
	nilPath    []node // stores nil path of the flow from nilable source to conflict point
	nonnilPath []node // stores non-nil path of the flow from conflict point to dereference point
}

type node struct {
	producerPosition token.Position
	consumerPosition token.Position
	producerRepr     string
	consumerRepr     string
}

// newNode creates a new node object from the given producer and consumer Prestrings.
// LocatedPrestring contains accurate information about the pos and the reason why NilAway deemed that pos
// to be nilable. We use it if available, else we use the raw string representation available from the Prestring.
func newNode(p annotation.Prestring, c annotation.Prestring) node {
	nodeObj := node{}

	// get producer representation string
	if l, ok := p.(annotation.LocatedPrestring); ok {
		nodeObj.producerPosition = l.Location
		nodeObj.producerRepr = l.Contained.String()
	} else if p != nil {
		nodeObj.producerRepr = p.String()
	}

	// get consumer representation string
	if l, ok := c.(annotation.LocatedPrestring); ok {
		nodeObj.consumerPosition = l.Location
		nodeObj.consumerRepr = l.Contained.String()
	} else if c != nil {
		nodeObj.consumerRepr = c.String()
	}

	return nodeObj
}

func (n *node) String() string {
	posStr := "<no pos info>"
	reasonStr := "<no reason info>"
	if n.consumerPosition.IsValid() {
		posStr = n.consumerPosition.String()
	}
	if len(n.producerRepr) > 0 && len(n.consumerRepr) > 0 {
		reasonStr = n.producerRepr + " " + n.consumerRepr
	} else if len(n.producerRepr) > 0 {
		reasonStr = n.producerRepr
	} else if len(n.consumerRepr) > 0 {
		reasonStr = n.consumerRepr
	}

	return fmt.Sprintf("\t-> %s: %s", posStr, reasonStr)
}

// addNilPathNode adds a new node to the nil path.
func (n *nilFlow) addNilPathNode(p annotation.Prestring, c annotation.Prestring) {
	nodeObj := newNode(p, c)

	// Note that in the implication graph, we traverse backwards from the point of conflict to the source of nilability.
	// Therefore, they are added in reverse order from what the program flow would look like. To account for this we
	// prepend the new node to nilPath because we want to print the program flow in its correct (forward) order.
	// TODO: instead of prepending here, we can reverse the nilPath slice while printing.
	n.nilPath = append([]node{nodeObj}, n.nilPath...)
}

// addNonNilPathNode adds a new node to the non-nil path
func (n *nilFlow) addNonNilPathNode(p annotation.Prestring, c annotation.Prestring) {
	nodeObj := newNode(p, c)
	n.nonnilPath = append(n.nonnilPath, nodeObj)
}

// String converts a nilFlow to a string representation, where each entry is the flow of the form: `<pos>: <reason>`
func (n *nilFlow) String() string {
	var flow []string
	for _, nodes := range [...][]node{n.nilPath, n.nonnilPath} {
		for _, nodeObj := range nodes {
			flow = append(flow, nodeObj.String())
		}
	}
	return "\n" + strings.Join(flow, "\n")
}

func (c *conflict) String() string {
	consumerPos := c.flow.nonnilPath[len(c.flow.nonnilPath)-1].consumerPosition

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

	return fmt.Sprintf(" Potential nil panic at \"%s\". Observed nil flow from "+
		"source to dereference: %s%s", consumerPos.String(), c.flow.String(), similarConflictsString)
}

func (c *conflict) addSimilarConflict(conflict conflict) {
	c.similarConflicts = append(c.similarConflicts, &conflict)
}

type conflictList struct {
	conflicts []conflict
}

func (l *conflictList) addSingleAssertionConflict(pass *analysis.Pass, trigger annotation.FullTrigger) {
	t := fullTriggerAsPrimitive(pass, trigger)
	c := conflict{
		pos:  t.Pos,
		flow: nilFlow{},
	}

	c.flow.addNonNilPathNode(t.ProducerRepr, t.ConsumerRepr)

	l.conflicts = append(l.conflicts, c)
}

func (l *conflictList) addOverconstraintConflict(nilExplanation ExplainedBool, nonnilExplanation ExplainedBool, pass *analysis.Pass) {
	c := conflict{}

	// Build nil path by traversing the inference graph from `nilExplanation` part of the overconstraint failure.
	// (Note that this traversal gives us a backward path from point of conflict to the source of nilability. Hence, we
	// must take this into consideration while printing the flow, which is currently being handled in `addNilPathNode()`.)
	var queue []ExplainedBool
	queue = append(queue, nilExplanation)

	for len(queue) > 0 {
		e := queue[0]
		queue = queue[1:]

		t := e.getPrimitiveFullTrigger()
		// We have two cases here:
		// 1. No annotation present (i.e., full inference): we have producer and consumer explanations available; use them directly
		// 2: Annotation present (i.e., no inference): we construct the explanation from the annotation string
		if t.ConsumerRepr != nil && t.ProducerRepr != nil {
			c.flow.addNilPathNode(t.ProducerRepr, t.ConsumerRepr)
		} else {
			c.flow.addNilPathNode(annotation.LocatedPrestring{
				Contained: e,
				Location:  util.TruncatePosition(pass.Fset.Position(t.Pos)),
			}, nil)
		}

		if b := e.deeperReason(); b != nil {
			queue = append(queue, b)
		}
	}

	// Build nonnil path by traversing the inference graph from `nonnilExplanation` part of the overconstraint failure.
	// (Note that this traversal is forward from the point of conflict to dereference. Hence, we don't need to make
	// any special considerations while printing the flow.)
	// Different from building the nil path above, here we also want to deduce the pos where the error should be reported,
	// i.e., the point of dereference where the nil panic would occur. In NilAway's context this is the last node
	// in the non-nil path. Therefore, we keep updating `c.pos` until we reach the end of the non-nil path.
	queue = make([]ExplainedBool, 0)
	queue = append(queue, nonnilExplanation)
	for len(queue) > 0 {
		e := queue[0]
		queue = queue[1:]

		t := e.getPrimitiveFullTrigger()
		// Similar to above, we have two cases here:
		// 1. No annotation present (i.e., full inference): we have producer and consumer explanations available; use them directly
		// 2: Annotation present (i.e., no inference): we construct the explanation from the annotation string
		if t.ConsumerRepr != nil && t.ProducerRepr != nil {
			c.flow.addNonNilPathNode(t.ProducerRepr, t.ConsumerRepr)
			c.pos = t.Pos
		} else {
			c.flow.addNonNilPathNode(annotation.LocatedPrestring{
				Contained: e,
				Location:  util.TruncatePosition(pass.Fset.Position(t.Pos)),
			}, nil)
			c.pos = t.Pos
		}

		if b := e.deeperReason(); b != nil {
			queue = append(queue, b)
		}
	}

	l.conflicts = append(l.conflicts, c)
}

func (l *conflictList) diagnostics() []analysis.Diagnostic {
	var diagnostics []analysis.Diagnostic

	// group conflicts with the same nil path together for concise reporting
	groupedConflicts := groupConflicts(l.conflicts)

	// build diagnostics from grouped conflicts
	for _, c := range groupedConflicts {
		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:     c.pos,
			Message: c.String(),
		})
	}
	return diagnostics
}

func pathString(nodes []node) string {
	path := ""
	for _, n := range nodes {
		path += n.String()
	}
	return path
}

// groupConflicts groups conflicts with the same nil path together and update conflicts list.
func groupConflicts(allConflicts []conflict) []conflict {
	conflictsMap := make(map[string]int)  // key: nil path string, value: index in `allConflicts`
	indicesToIgnore := make(map[int]bool) // indices of conflicts to be ignored from `allConflicts`, since they are grouped with other conflicts

	for i, c := range allConflicts {
		key := pathString(c.flow.nilPath)

		// Handle the case of single assertion conflict separately
		if len(c.flow.nilPath) == 0 && len(c.flow.nonnilPath) == 1 {
			// This is the case of single assertion conflict. Use producer position and repr from the non-nil path as the key.
			if p := c.flow.nonnilPath[0]; p.producerPosition.IsValid() {
				key = p.producerPosition.String() + ": " + p.producerRepr
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

	// update groupedConflicts list with grouped groupedConflicts                                                                                            String         ts
	var groupedConflicts []conflict
	for i, c := range allConflicts {
		if _, ok := indicesToIgnore[i]; !ok {
			groupedConflicts = append(groupedConflicts, c)
		}
	}
	return groupedConflicts
}
