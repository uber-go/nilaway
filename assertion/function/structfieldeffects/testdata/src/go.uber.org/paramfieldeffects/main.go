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

// Package paramfieldeffects is the fixture for the ComputeParamFieldEffects boundary-summary test.
package paramfieldeffects

type Leaf struct {
	Ptr *int
}

type Node struct {
	Child *Leaf
}

type Outer struct {
	Mid   *Node
	Count int
}

type Wrap struct {
	Inner *Outer
}

type Rec struct {
	Self *Rec
	Ptr  *int
}

// directRead dereferences two nested field prefixes of its parameter: reaching o.Mid.Child.Ptr
// requires o.Mid and o.Mid.Child to be non-nil, so both paths are param reads.
func directRead(o *Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	_ = o.Mid.Child.Ptr
}

func explicitDerefRead(o *Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	_ = (*o).Mid.Child.Ptr
}

func intermediateDerefRead(o *Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	_ = (*o.Mid).Child.Ptr
}

func doublePointerRead(o **Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	_ = (*o).Mid.Child.Ptr
}

// forwarder passes its parameter straight through, so the closure copies directRead's reads verbatim.
func forwarder(o *Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	directRead(o)
}

func forwardDerefPointerRead(o **Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	directRead(*o)
}

// forwardField forwards a nested field of its parameter, so the inherited reads gain the "Inner" prefix.
func forwardField(w *Wrap) { // expect_effects: param_reads:0:Inner.Mid param_reads:0:Inner.Mid.Child
	directRead(w.Inner)
}

// recvRead exercises the receiver boundary index (ReceiverParamIndex).
func (o *Outer) recvRead() { // expect_effects: param_reads:-1:Mid param_reads:-1:Mid.Child
	_ = o.Mid.Child.Ptr
}

// makeOuter is a struct-returning constructor; a caller's deref of its result becomes a return read.
func makeOuter() *Outer { // expect_effects: return_reads:0:Mid return_reads:0:Mid.Child
	return &Outer{}
}

// consumeReturn dereferences the result of makeOuter, attributing the demand to makeOuter's return.
func consumeReturn() {
	o := makeOuter()
	_ = o.Mid.Child.Ptr
}

// recRead directly dereferences a self-recursive field chain; direct reads keep the exact paths.
func recRead(r *Rec) { // expect_effects: param_reads:0:Self param_reads:0:Self.Self
	_ = r.Self.Self.Ptr
}

// recForward forwards a self-recursive field, so the closure must stop the path from growing.
func recForward(r *Rec) { // expect_effects:
	recRead(r.Self)
}

func directWrite(o *Outer) { // expect_effects: writes:0:Mid
	o.Mid = &Node{}
}

func deepWrite(o *Outer) { // expect_effects: writes:0:Mid.Child param_reads:0:Mid
	o.Mid.Child = nil
}

func (o *Outer) receiverWrite() { // expect_effects: writes:-1:Mid
	o.Mid = &Node{}
}

func forwardWrite(o *Outer) { // expect_effects: writes:0:Mid.Child
	writeChild(o.Mid)
}

func forwardDerefPointerWrite(o **Outer) { // expect_effects: writes:0:Mid
	directWrite(*o)
}

func forwardWriteAgain(w *Wrap) { // expect_effects: writes:0:Inner.Mid.Child
	forwardWrite(w.Inner)
}

func writeChild(n *Node) { // expect_effects: writes:0:Child
	n.Child = nil
}

func ignoredNonNilableWrite(o *Outer) { // expect_effects:
	o.Count = 1
}

func ignoredLocalWrite() { // expect_effects:
	o := &Outer{}
	o.Mid = nil
}

func ignoredValueParamWrite(o Outer) { // expect_effects:
	o.Mid = nil
}

func ignoredRecursiveWrite(r *Rec) { // expect_effects: param_reads:0:Self
	r.Self.Ptr = nil
}

func explicitDerefWrite(o *Outer) { // expect_effects: writes:0:Mid
	(*o).Mid = nil
}

func explicitDoublePointerWrite(o **Outer) { // expect_effects: writes:0:Mid
	(*o).Mid = nil
}
