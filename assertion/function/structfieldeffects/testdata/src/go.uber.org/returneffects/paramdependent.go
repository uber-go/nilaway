//  Copyright (c) 2026 Uber Technologies, Inc.
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

// This file is the fixture for return param sources (ReturnParamSource): results whose value is the
// caller's own argument. Whole-result sources come from returned parameters and projections;
// field-level sources come from construction fields initialized from parameters. Sources are dropped
// whenever the syntactic relation is not unconditional: mixed construct/forward results,
// reassigned or address-taken parameters, and spread returns.

package returneffects

// Pair copies caller-dependent values into a fresh allocation, so its fields are param-sourced rather
// than summarized as concrete effects.
type Pair struct {
	Existing  *Leaf
	Requested *Leaf
}

func forwardParam(y *Outer) *Outer { // expect_effects: return_param_source:0::0:
	return y
}

func forwardParamProjection(y *Outer) *Node { // expect_effects: return_param_source:0::0:Mid param_reads:0:Mid
	return y.Mid
}

func (o *Outer) receiverProjection() *Node { // expect_effects: return_param_source:0::-1:Mid param_reads:-1:Mid
	return o.Mid
}

func forwardWithErr(y *Outer) (*Outer, error) { // expect_effects: return_param_source:0::0:
	return y, nil
}

// Both branches' sources are kept even though they disagree; consumers refuse to answer a path
// with disagreeing sources, so keeping both stays deterministic and sound.
func forwardEitherParam(y *Outer, z *Outer, pick bool) *Outer { // expect_effects: return_param_source:0::0: return_param_source:0::1:
	if pick {
		return y
	}
	return z
}

func newPair(existing, requested *Leaf) *Pair { // expect_effects: return_param_source:0:Existing:0: return_param_source:0:Requested:1:
	return &Pair{Existing: existing, Requested: requested}
}

// A nested construction records the source at the composed result path.
func wrapChild(child *Leaf) *Outer { // expect_effects: return_param_source:0:Mid.Child:0: return_effects:0:Value.Child
	return &Outer{Mid: &Node{Child: child}}
}

// A field initialized from a parameter field chain records the source path within the parameter.
func wrapDeep(src *Outer) *Node { // expect_effects: return_param_source:0:Child:0:Mid.Child param_reads:0:Mid param_reads:0:Mid.Child
	return &Node{Child: src.Mid.Child}
}

// A non-nilable value-struct field gets a source too: the source re-roots deeper accesses of the
// result under the parameter (`result.Value.Child` resolves to `src.Value.Child`). Reading
// `src.Value` itself creates no param-read demand (value structs bar nilness).
func copyValueField(src *Outer) *Outer { // expect_effects: return_param_source:0:Value:0:Value return_effects:0:Mid
	return &Outer{Value: src.Value}
}

// A result that sometimes constructs and sometimes forwards its parameter carries no sound source;
// the whole-result source is dropped in close(). (The construct branch's concrete effects remain —
// their interaction with mixed results is owned by a later fix revision.)
func mixedConstructOrForward(y *Outer, pick bool) *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	if pick {
		return &Outer{}
	}
	return y
}

// A reassigned parameter may not hold the caller's argument at the return: no source.
func reassignedParam(y *Outer, z *Outer) *Outer { // expect_effects:
	y = z
	return y
}

// The reassigned parameter loses only its own sources; the stable sibling keeps its source.
func reassignedBeforeConstruct(existing, requested *Leaf) *Pair { // expect_effects: return_param_source:0:Requested:1:
	existing = nil
	return &Pair{Existing: existing, Requested: requested}
}

// An address-taken parameter can be reassigned through the alias: no source.
func addressTakenParam(y *Outer) *Outer { // expect_effects:
	escapeOuter(&y)
	return y
}

func escapeOuter(**Outer) {}

// A closure reassignment destabilizes the captured parameter exactly like a direct one.
func closureReassignedParam(y *Outer, z *Outer) *Outer { // expect_effects:
	swap := func() { y = z }
	swap()
	return y
}

// A spread return supplies every result from one expression; no argument expression identifies a
// single result, so no source is recorded (the forwarding edge machinery is also bypassed).
func spreadReturn(y *Outer) (*Outer, error) { // expect_effects:
	return forwardWithErr(y)
}
