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

// Edge-case guards for the deep / explicit-deref parameter-field write detectors (see the deep-write
// cases in limitations.go): a top-level by-value copy must not alias its source, a double-pointer
// write must be rejected, and a recursive field path must be skipped while the analysis still
// terminates. Each is a deliberate no-report case.
//
// This file defines one self-recursive type (recNode) solely to exercise the recursion guard; the
// other tests use the non-recursive Outer/Node/Leaf types from limitations.go.
package limitations

// ---------------------------------------------------------------------------------------------
// GUARD: a top-level by-value struct copy does not alias its source
// ---------------------------------------------------------------------------------------------
//
// `g(*x)` passes a copy of the struct, so a field the callee mutates is not visible to the caller's
// `x`. A top-level `*x` argument must therefore not record a forwarding edge.

// byValWriteNil takes a Node by value and nils its field — invisible to any caller's pointer.
func byValWriteNil(n Node) {
	n.child = nil
}

// passByValue passes a by-value copy (`*x`), so no forwarding edge is recorded.
func passByValue(x *Node) {
	byValWriteNil(*x)
}

func tByValueCopyNoForward() *int {
	b := &Node{child: &Leaf{}} // child non-nil
	passByValue(b)             // the copy is mutated; b.child is unchanged
	return b.child.ptr         // safe — must NOT be reported (no //want)
}

// ---------------------------------------------------------------------------------------------
// GUARD: a double-pointer write is rejected
// ---------------------------------------------------------------------------------------------
//
// `(*x).child` with x of type **Node addresses a field two pointer levels deep. The path resolves to
// no struct type and is skipped, so the mutation is an under-report and the analysis must not crash.

func writeViaDoublePtr(x **Node) {
	(*x).child = nil
}

func tWriteViaDoublePtr() *int {
	inner := &Node{child: &Leaf{}} // inner.child non-nil
	pp := &inner                   // pp is **Node
	writeViaDoublePtr(pp)          // double-pointer write is not tracked
	return (*pp).child.ptr         // under-report — not reported (no //want)
}

// ---------------------------------------------------------------------------------------------
// GUARD: a recursive field path is skipped (and the analysis terminates)
// ---------------------------------------------------------------------------------------------
//
// A deep write whose path re-enters a struct type already on the chain (`r.next.leaf`) would let the
// path grow without bound. Such paths are skipped, so the write is an under-report but collection and
// inference still terminate.

// recNode is self-recursive via next; it exists solely to drive the recursion-guard test.
type recNode struct {
	next *recNode
	leaf *Leaf
}

func writeRecDeep(r *recNode) {
	r.next.leaf = nil // path "next.leaf" re-enters recNode at "next" -> skipped
}

func tRecDeepWrite() *int {
	b := &recNode{next: &recNode{leaf: &Leaf{}}}
	writeRecDeep(b)        // recursive-path write is skipped (under-report)
	return b.next.leaf.ptr // not reported (no //want); analysis still terminates
}
