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
	Mid *Node
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
func directRead(o *Outer) { // expect_reads: param_reads:0:Mid param_reads:0:Mid.Child
	_ = o.Mid.Child.Ptr
}

// forwarder passes its parameter straight through, so the closure copies directRead's reads verbatim.
func forwarder(o *Outer) { // expect_reads: param_reads:0:Mid param_reads:0:Mid.Child
	directRead(o)
}

// forwardField forwards a nested field of its parameter, so the inherited reads gain the "Inner" prefix.
func forwardField(w *Wrap) { // expect_reads: param_reads:0:Inner.Mid param_reads:0:Inner.Mid.Child
	directRead(w.Inner)
}

// recvRead exercises the receiver boundary index (ReceiverParamIndex).
func (o *Outer) recvRead() { // expect_reads: param_reads:-1:Mid param_reads:-1:Mid.Child
	_ = o.Mid.Child.Ptr
}

// makeOuter is a struct-returning constructor; a caller's deref of its result becomes a return read.
func makeOuter() *Outer { // expect_reads: return_reads:0:Mid return_reads:0:Mid.Child
	return &Outer{}
}

// consumeReturn dereferences the result of makeOuter, attributing the demand to makeOuter's return.
func consumeReturn() {
	o := makeOuter()
	_ = o.Mid.Child.Ptr
}

// recRead directly dereferences a self-recursive field chain; direct reads keep the exact paths.
func recRead(r *Rec) { // expect_reads: param_reads:0:Self param_reads:0:Self.Self
	_ = r.Self.Self.Ptr
}

// recForward forwards a self-recursive field, so the closure must stop the path from growing.
func recForward(r *Rec) { // expect_reads:
	recRead(r.Self)
}
