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
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

type conflict struct {
	position token.Pos // stores position where the error should be reported
	expr     string    // stores expression that is overcontrained (i.e., expected to be nonnil, but found nilable)
	nilFlow  nilFlow   // stores nil flow from source to dereference point
}

type nilFlow struct {
	nilPath    []node // stores nil path of the flow from nilable source to conflict point
	nonnilPath []node // stores non-nil path of the flow from conflict point to dereference point
}

type node struct {
	position token.Position
	reason   string
}

// newNode creates a new node object from the given Prestring.
// LocatedPrestring contains accurate information about the position and the reason why NilAway deemed that position
// to be nilable. We use it if available, else we use the raw string representation available from the Prestring `p`.
func newNode(p annotation.Prestring) node {
	nodeObj := node{}
	if l, ok := p.(annotation.LocatedPrestring); ok {
		nodeObj.position = l.Location
		nodeObj.reason = l.Contained.String()
	} else if p != nil {
		nodeObj.reason = p.String()
	}
	return nodeObj
}

func (n *node) String() string {
	posStr := "<no pos info>"
	reasonStr := "<no reason info>"
	if n.position.IsValid() {
		posStr = n.position.String()
	}
	if len(n.reason) > 0 {
		reasonStr = n.reason
	}
	return fmt.Sprintf("\n\t-> %s: %s", posStr, reasonStr)
}

// addNilPathNode adds a new node to the nil path.
func (n *nilFlow) addNilPathNode(p annotation.Prestring) {
	nodeObj := newNode(p)

	// Note that in the implication graph, we traverse backwards from the point of conflict to the source of nilability.
	// Therefore, they are added in reverse order from what the program flow would look like. To account for this we
	// prepend the new node to nilPath because we want to print the program flow in its correct (forward) order.
	n.nilPath = append([]node{nodeObj}, n.nilPath...)
}

// addNonNilPathNode adds a new node to the non-nil path
func (n *nilFlow) addNonNilPathNode(p annotation.Prestring) {
	nodeObj := newNode(p)
	n.nonnilPath = append(n.nonnilPath, nodeObj)
}

// String converts a nilFlow to a string representation, where each entry is the flow of the form: `<pos>: <reason>`
func (n *nilFlow) String() string {
	// Augment reason for the first and last node in the flow with nilable and nonnil information, respectively.
	firstNilNodeIndex := 0
	n.nilPath[firstNilNodeIndex].reason = fmt.Sprintf("%s (found NILABLE)", n.nilPath[firstNilNodeIndex].reason)

	lastNonnilNodeIndex := len(n.nonnilPath) - 1
	n.nonnilPath[lastNonnilNodeIndex].reason = fmt.Sprintf("%s (must be NONNIL)", n.nonnilPath[lastNonnilNodeIndex].reason)

	flow := ""
	for _, nodes := range [...][]node{n.nilPath, n.nonnilPath} {
		for _, nodeObj := range nodes {
			flow += nodeObj.String()
		}
	}
	return flow
}

func (c *conflict) String() string {
	consumerPos := c.nilFlow.nonnilPath[len(c.nilFlow.nonnilPath)-1].position
	producerPos := c.nilFlow.nilPath[0].position
	return fmt.Sprintf(" Nonnil `%s` expected at \"%s\", but produced as nilable at \"%s\". Observed nil flow from "+
		"source to dereference: %s", c.expr, consumerPos.String(), producerPos.String(), c.nilFlow.String())
}

type conflictList struct {
	conflicts []conflict
}

func (l *conflictList) addSingleAssertionConflict(pass *analysis.Pass, trigger annotation.FullTrigger) {
	t := fullTriggerAsPrimitive(pass, trigger)
	c := conflict{
		position: t.Pos,
		expr:     util.ExprToString(trigger.Consumer.Expr, pass),
		nilFlow:  nilFlow{},
	}

	c.nilFlow.addNilPathNode(t.ProducerRepr)
	c.nilFlow.addNonNilPathNode(t.ConsumerRepr)

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
			c.nilFlow.addNilPathNode(t.ConsumerRepr)
			c.nilFlow.addNilPathNode(t.ProducerRepr)
		} else {
			c.nilFlow.addNilPathNode(annotation.LocatedPrestring{
				Contained: e,
				Location:  util.TruncatePosition(pass.Fset.Position(t.Pos)),
			})
		}

		if e.getExplainedBool() != nil {
			queue = append(queue, e.getExplainedBool())
		}
	}

	// Build nonnil path by traversing the inference graph from `nonnilExplanation` part of the overconstraint failure.
	// (Note that this traversal is forward from the point of conflict to dereference. Hence, we don't need to make
	// any special considerations while printing the flow.)
	// Different from building the nil path above, here we also want to deduce the position where the error should be reported,
	// i.e., the point of dereference where the nil panic would occur. In NilAway's context this is the last node
	// in the non-nil path. Therefore, we keep updating `c.position` until we reach the end of the non-nil path.
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
			c.nilFlow.addNonNilPathNode(t.ProducerRepr)
			c.nilFlow.addNonNilPathNode(t.ConsumerRepr)
			c.position = t.Pos
		} else {
			c.nilFlow.addNonNilPathNode(annotation.LocatedPrestring{
				Contained: e,
				Location:  util.TruncatePosition(pass.Fset.Position(t.Pos)),
			})
			c.position = t.Pos
		}

		if e.getExplainedBool() != nil {
			queue = append(queue, e.getExplainedBool())
		}
	}

	// add the expression that was overconstrained
	c.expr = nonnilExplanation.getPrimitiveFullTrigger().ExprRepr

	l.conflicts = append(l.conflicts, c)
}

func (l *conflictList) diagnostics() []analysis.Diagnostic {
	var diagnostics []analysis.Diagnostic
	for _, c := range l.conflicts {
		diagnostics = append(diagnostics, analysis.Diagnostic{
			Pos:     c.position,
			Message: c.String(),
		})
	}
	return diagnostics
}
