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

// Package returneffects is the fixture for concrete return-effect collection.
package returneffects

type Leaf struct {
	Ptr *int
}

type Node struct {
	Child *Leaf
}

type Outer struct {
	Mid   *Node
	Value Node
}

type Rec struct {
	Self *Rec
	Ptr  *int
}

func safeOuter() *Outer { // expect_effects:
	return &Outer{
		Mid:   &Node{Child: &Leaf{Ptr: new(int)}},
		Value: Node{Child: &Leaf{Ptr: new(int)}},
	}
}

func inlinePointer() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return &Outer{}
}

func inlineValue() Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return Outer{Mid: nil, Value: Node{Child: nil}}
}

func builtinNew() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return new(Outer)
}

func nestedAllocation() *Outer { // expect_effects: return_effects:0:Mid.Child return_effects:0:Value.Child
	return &Outer{Mid: &Node{}}
}

func unknownInitializer(mid *Node) *Outer { // expect_effects: return_effects:0:Value.Child
	return &Outer{
		Mid:   mid,
		Value: Node{},
	}
}

func shadowedNil() *Outer { // expect_effects:
	nil := &Node{Child: &Leaf{Ptr: new(int)}}
	return &Outer{
		Mid:   nil,
		Value: Node{Child: &Leaf{Ptr: new(int)}},
	}
}

func directForward() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return inlinePointer()
}

func genericConcrete[T any]() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return &Outer{}
}

func genericForward() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return genericConcrete[int]()
}

func pair() (*Outer, *Node) { // expect_effects: return_effects:1:Child
	return safeOuter(), &Node{}
}

func cycleA(recurse bool) *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	if recurse {
		return cycleB(recurse)
	}
	return inlinePointer()
}

func cycleB(recurse bool) *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return cycleA(recurse)
}

func nestedFunctionReturn() *Outer { // expect_effects:
	_ = func() *Outer { return &Outer{} }
	return safeOuter()
}

func recursiveAllocation() *Rec { // expect_effects: return_effects:0:Ptr
	return &Rec{Self: &Rec{}}
}

func dynamicCall(f func() *Outer) *Outer { // expect_effects:
	return f()
}

func multiResult() (*Outer, error) { // expect_effects:
	return safeOuter(), nil
}

func multiResultWithNil() (Outer, error) { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return inlineValue(), nil
}

func multiResultSpread() (Outer, error) { // expect_effects:
	return multiResultWithNil()
}

func shortLocal() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	x := &Outer{}
	return x
}

func declaredLocal() *Outer { // expect_effects: return_effects:0:Mid.Child return_effects:0:Value.Child
	var x = &Outer{Mid: &Node{}}
	return x
}

func parenthesizedLocal() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	x := &Outer{}
	return (x)
}

func multipleLocalReturns(cond bool) *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	x := &Outer{}
	if cond {
		return x
	}
	return x
}

func genericLocalForward() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	x := genericConcrete[int]()
	return x
}

func localForward() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	x := directForward()
	return x
}

func forwardSecondResult() *Node { // expect_effects: return_effects:0:Child
	_, n := pair()
	return n
}

// No return effects: mutating the local invalidates its tracked allocation.
func mutatedLocal() *Outer { // expect_effects:
	x := &Outer{}
	x.Mid = &Node{}
	return x
}

// No return effects: reassigning the local invalidates its tracked allocation.
func reassignedLocal() *Outer { // expect_effects:
	x := &Outer{}
	x = safeOuter()
	return x
}

// No return effects: mixed reassignment invalidates the tracked local.
func mixedReassignment() *Outer { // expect_effects:
	x := &Outer{}
	x, used := safeOuter(), true
	_ = used
	return x
}

// No return effects: the range assignment overwrites the tracked local.
func rangeReassignment() *Outer { // expect_effects:
	x := &Outer{}
	for _, x = range []*Outer{safeOuter()} {
	}
	return x
}

func rangeWithoutOperands() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	x := &Outer{}
	for range []int{} {
	}
	return x
}

// No return effects: taking the local's address exposes it to mutation.
func addressTaken() *Outer { // expect_effects:
	x := &Outer{}
	_ = &x
	return x
}

func escape(*Outer) {}

// No return effects: passing the local to a call exposes it to mutation.
func passedToCall() *Outer { // expect_effects:
	x := &Outer{}
	escape(x)
	return x
}

// No return effects: assigning the local to an alias loses exclusive ownership.
func aliasedLocal() *Outer { // expect_effects:
	x := &Outer{}
	y := x
	_ = y
	return x
}

// No return effects: capturing the local allows deferred mutation. This is a false negative
// caused by conservativeness: the closure is never invoked, so x's allocation is in fact safe to
// track, but collectStableStructVars disqualifies any captured candidate without distinguishing
// called from uncalled closures.
func capturedLocal() *Outer { // expect_effects:
	x := &Outer{}
	_ = func() { x.Mid = &Node{} }
	return x
}

// No return effects: capturing the local allows deferred mutation, and here the closure is actually
// invoked, so conservativeness is justified and the loss of effects is a true negative.
func capturedLocalInvoked() *Outer { // expect_effects:
	x := &Outer{}
	f := func() { x.Mid = &Node{} }
	f()
	return x
}

// No return effects: a return from a closure is not an allowed use of the enclosing local.
func returnedFromClosure() *Outer { // expect_effects:
	x := &Outer{}
	_ = func() *Outer { return x }
	return x
}

// No return effects: even a non-mutating observation is outside the stable local pattern.
func observedLocal() *Outer { // expect_effects:
	x := &Outer{}
	if x.Mid == nil {
		return safeOuter()
	}
	return x
}

// No return effects: field projections are not tracked as concrete return sources.
func fieldProjection() *Node { // expect_effects:
	x := &Outer{}
	return x.Mid
}

// No return effects: an uninitialized local is not a tracked allocation.
func zeroValue() Outer { // expect_effects:
	var x Outer
	return x
}

// No return effects: a naked return has no explicit result expression.
func nakedReturn() (out Outer) { // expect_effects:
	return
}
