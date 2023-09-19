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

package annotation

import (
	"fmt"
	"go/token"

	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// A FullTrigger is a completed assertion. It contains both a ProduceTrigger Producer and a
// ConsumeTrigger Consumer, representing a path along which a nil value can be produced and
// consumed respectively. All produce and consume triggers are functions of the read set of
// annotations, so a FullTrigger represents only a possibility of a nil flow error depending
// on the set of annotations. A FullTrigger can be compared to an Annotation set to see if
// such a nil flow error actually arises by the Check method.
type FullTrigger struct {
	Producer *ProduceTrigger
	Consumer *ConsumeTrigger
	// Controller is the site that controls if this trigger will be activated or not.
	// If the controller site is assigned to nilable, then this full trigger is activated;
	// otherwise the full trigger is deactivated in the inference engine.
	// If this field is nil, it means the trigger is not a controlled trigger and the trigger will
	// be activated all the time.
	Controller *CallSiteParamAnnotationKey
	// CreatedFromDuplication is true if the full trigger is created from duplicating another full
	// trigger; otherwise false, which is also the default value for any normal full trigger.
	CreatedFromDuplication bool
}

// Controlled returns true if this full trigger is controlled by a controller site; otherwise
// returns false.
func (t *FullTrigger) Controlled() bool {
	return t.Controller != nil
}

// Pos returns the position for logging the error specified by the ConsumeTrigger
func (t *FullTrigger) Pos() token.Pos {
	return t.Consumer.Pos()
}

// Check is a boolean test that determines whether this FullTrigger should be triggered against the Annotation map `annMap`
func (t *FullTrigger) Check(annMap Map) bool {
	return t.Producer.Annotation.CheckProduce(annMap) &&
		t.Consumer.Annotation.CheckConsume(annMap)
}

func (t *FullTrigger) truncatedConsumerPos(pass *analysis.Pass) token.Position {
	return util.PosToLocation(t.Consumer.Pos(), pass)
}

func (t *FullTrigger) truncatedProducerPos(pass *analysis.Pass) token.Position {
	// Our struct init analysis only tracks fields for depth 1 and relies on escape analysis for
	// escaped fields (t.Producer.Expr here). Since there are functions that return nil producers
	// (although they were never assigned to [FullTrigger.Producer]), NilAway concluded that
	// [ProduceTrigger.Expr] must be nilable. Therefore, we add a redundant check here to guard
	// against such cases and make NilAway happy.
	// TODO: remove this redundant check .
	if t.Producer.Expr == nil {
		panic(fmt.Sprintf("nil Expr for producer %q", t.Producer))
	}
	return util.PosToLocation(t.Producer.Expr.Pos(), pass)
}

// equals returns true if the two passed FullTriggers are equal, and false otherwise.
func (t *FullTrigger) equals(other FullTrigger) bool {
	return t.Producer.Annotation.equals(other.Producer.Annotation) &&
		t.Consumer.Annotation.equals(other.Consumer.Annotation) &&
		t.Consumer.Expr == other.Consumer.Expr &&
		t.Consumer.GuardMatched == other.Consumer.GuardMatched
}

// equalsModuloGuardMatched returns true if the two passed FullTriggers (modulo the GuardMatched field) are equal, and false otherwise.
func (t *FullTrigger) equalsModuloGuardMatched(other FullTrigger) bool {
	return t.Producer.Annotation.equals(other.Producer.Annotation) &&
		t.Consumer.Annotation.equals(other.Consumer.Annotation) &&
		t.Consumer.Expr == other.Consumer.Expr
}

// A LocatedPrestring wraps another Prestring with a `token.Position` - for formatting with that position
type LocatedPrestring struct {
	Contained Prestring
	Location  token.Position
}

func (l LocatedPrestring) String() string {
	return fmt.Sprintf("%s at \"%s\"", l.Contained.String(), l.Location.String())
}

// Prestrings returns Prestrings for clauses describing the production and consumption indicated by this
// FullTrigger, of the forms: "assigned into a field a bar.go:10" or
// "returned from the function foo at baz.go:25"
//
// If the Producer's expression is an artificial one created by NilAway instead of pulled as an authentic
// AST node from the source, we elide its location as it will be counter-informative.
// Unfortunately - many if not most Produce Triggers expression are artificial. More specifically
// any producers that are matched with consumers that reached entry to a function get matched
// with artifical expression generated from the position of that consumer in the assertion tree,
// and producers that arise from non-trackable expressions correspond to those real non-trackable
// expressions.
func (t *FullTrigger) Prestrings(pass *analysis.Pass) (Prestring, Prestring) {
	producerPrestring := t.Producer.Annotation.Prestring()
	if util.ExprIsAuthentic(pass, t.Producer.Expr) {
		producerPrestring = LocatedPrestring{
			Contained: producerPrestring,
			Location:  t.truncatedProducerPos(pass),
		}
	}
	consumerPrestring := LocatedPrestring{
		Contained: t.Consumer.Annotation.Prestring(),
		Location:  t.truncatedConsumerPos(pass),
	}
	return producerPrestring, consumerPrestring
}

// FullTriggerSlicesEq returns true if the two passed slices of FullTriggers contain the same elements. It determines if
// assertion trees have stabilized during the primary fixpoint loop in `BackpropAcrossFunc`
// (precondition: no duplications)
// The equality of two FullTriggers is determined by four parameters:
// 1) Producer Annotation - this is the first half of the assertion on annotations represented by the trigger
// 2) Consumer Annotation - this is the second half of the assertion on annotations represented
// 3) Consumer Expression - this distinguishes triggers that represent the same assertion but should
// be reported on different lines. If we switch to a purely inference-based approach, this is not
// necessary - it serves only to report errors on every line that the error repeatedly occurs.
// 4) Consumer GuardMatched - this is essential because after stabilization, calls to
// RootAssertionNode.ProcessEntry can use checkGuardOnFullTrigger to rewrite the producer based on
// its value. So if you accept that the producer is needed for equality, you accept that
// Consumer.GuardMatched is needed for equality.
func FullTriggerSlicesEq(left, right []FullTrigger) bool {
	if len(left) != len(right) {
		return false
	}

	// because we have two sets of the same size, without repetition, to test equality it suffices
	// to check that one of them contains the other
	matched := make(map[int]bool)
	for _, l := range left {
		for j, r := range right {
			if l.equals(r) {
				matched[j] = true
				break
			}
		}
	}
	return len(matched) == len(left)
}

// MergeFullTriggers creates a union of the passed left and right triggers eliminating duplicates
// Merging is based on three parameters (out of the four discussed above):
// 1) Producer Annotation
// 2) Consumer Annotation
// 3) Consumer Expression
// The three parameters are chosen based on the fact that we merge two full triggers that disagree only on
// Consumer.GuardMatched into a single trigger with Consume.GuardMatched = false. In all other cases - such as
// checking fixed point in propagation, the function FullTriggersEq
// that does observe GuardMatched should be used instead of this function.
func MergeFullTriggers(left []FullTrigger, right ...FullTrigger) []FullTrigger {
	var out []FullTrigger
	updateLeftGuard := make(map[int]bool)
	skipRight := make(map[int]bool)

	for i, l := range left {
		for j, r := range right {
			if l.equalsModuloGuardMatched(r) {
				if l.Consumer.GuardMatched && !r.Consumer.GuardMatched {
					updateLeftGuard[i] = true
				} else {
					skipRight[j] = true
				}
			}
		}
	}

	for i, l := range left {
		if updateLeftGuard[i] {
			l.Consumer.Guards = util.NoGuards()
			l.Consumer.GuardMatched = false
		}
		out = append(out, l)
	}

	for j, r := range right {
		if !skipRight[j] {
			out = append(out, r)
		}
	}

	return out
}
