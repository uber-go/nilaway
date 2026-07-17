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

// Package limitations collects the known false negatives and false positives of the struct-init
// analysis, plus regression guards for behavior that is now correct. Each case is a deliberate
// consequence of a precision/soundness boundary, not an accidental bug.
//
// Conventions (matching the analysistest framework):
//   - A false negative is a real nil panic the analysis fails to report: the line carries no //want,
//     and a comment notes the missed report.
//   - A false positive is a safe deref the analysis wrongly reports: the line carries a //want
//     matching the spurious diagnostic, and a comment marks it as spurious.
//
// A structural cause shared by several cases: the interprocedural boundary summary is
// context-insensitive — there is one inference site per (function, kind, param/return index, field
// path), shared by every caller. Allocation-site sensitivity holds within a function, but once a
// value crosses a boundary all callers merge into that single site.
package limitations

// Leaf is the bottom of the type chain; ptr is the dereference target.
type Leaf struct {
	ptr *int
}

// Node holds a single nilable field (a depth-1 boundary field).
type Node struct {
	child *Leaf
}

// Outer nests a Node, giving a depth-2 path (mid.child).
type Outer struct {
	mid *Node
}

// ---------------------------------------------------------------------------------------------
// Regression guards: deep binding through a trackable value (variable / field chain)
// ---------------------------------------------------------------------------------------------
//
// A deep (depth >= 2) nil carried into a boundary through a local variable is tracked, reaching
// parity with the inline-allocation case.

// Deep nil supplied to a parameter through a local variable.
func derefDeepParam(x *Outer) *int {
	return x.mid.child.ptr //want "field `mid.child` of param 0 of `derefDeepParam`"
}

func tDeepParamViaVar() *int {
	t := &Outer{mid: &Node{}} // t.mid.child is nil at depth 2
	return derefDeepParam(t)
}

// Deep nil returned through a local variable.
func mkDeepViaVar() *Outer {
	t := &Outer{mid: &Node{}} // t.mid.child is nil at depth 2
	return t
}

func tReturnDeepViaVar() *int {
	b := mkDeepViaVar()
	return b.mid.child.ptr //want "field `mid.child` of result 0 of `mkDeepViaVar`"
}

// ---------------------------------------------------------------------------------------------
// Regression guards: deep and explicit-deref direct parameter-field writes
// ---------------------------------------------------------------------------------------------
//
// A deep write (`o.mid.child = ...`) or an explicit-deref write (`(*x).child = ...`) inside a callee
// is captured and re-produced by the caller, keyed by the full field path.

// Direct multi-level field write inside the callee.
func writeDeepField(o *Outer) {
	o.mid.child = nil
}

func tDeepDirectWrite() *int {
	b := &Outer{mid: &Node{child: &Leaf{}}}
	writeDeepField(b)
	return b.mid.child.ptr //want "field `mid.child` of param 0 of `writeDeepField`"
}

// Field write through an explicit (parenthesized) dereference; `(*x).child` collapses to `x.child`.
func writeViaDeref(x *Node) {
	(*x).child = nil
}

func tWriteViaDeref() *int {
	b := &Node{}
	b.child = &Leaf{}
	writeViaDeref(b)
	return b.child.ptr //want "field `child` of param 0 of `writeViaDeref`"
}

// Negative control: a deep write that sets the field non-nil must not be reported, confirming the
// write-set carries the written value's nilability rather than merely "this path was touched".
func writeDeepFieldNonNil(o *Outer) {
	o.mid.child = &Leaf{}
}

func tDeepDirectWriteNonNil() *int {
	b := &Outer{mid: &Node{}} // b.mid.child is nil at allocation...
	writeDeepFieldNonNil(b)   // ...but the callee writes it non-nil
	return b.mid.child.ptr    // safe — no //want
}

// Deep write through a method receiver.
func (o *Outer) writeDeepRecv() {
	o.mid.child = nil
}

func tDeepRecvWrite() *int {
	b := &Outer{mid: &Node{child: &Leaf{}}}
	b.writeDeepRecv()
	return b.mid.child.ptr //want "field `mid.child` of method receiver of `writeDeepRecv`"
}

// ---------------------------------------------------------------------------------------------
// FALSE NEGATIVES (under-reports — no //want)
// ---------------------------------------------------------------------------------------------

// False negative: a naked named return has no result expression, so the constructor's shape is never
// summarized.
func nakedReturn() (n *Node) {
	n = &Node{} // n.child is nil
	return
}

func tNakedReturn() *int {
	b := nakedReturn()
	return b.child.ptr
}

// False negative: forwarding a parameter through a non-parameter local. Re-binding the parameter to a
// local first (`y := x`) breaks the forwarding link, so the forwardee's write is not attributed to
// the forwarder.
func writeNilField(x *Node) {
	x.child = nil
}

func fwdViaLocal(x *Node) {
	y := x // y is a local, not a parameter — no forwarding edge is recorded
	writeNilField(y)
}

func tForwardViaLocal() *int {
	b := &Node{}
	b.child = &Leaf{}
	fwdViaLocal(b)
	return b.child.ptr
}

// False negative: forwarding a parameter through a func-value call. A dynamic callee cannot be
// resolved, so no forwarding edge is recorded and the call is treated as transparent.
func fwdViaFuncValue(x *Node, fn func(*Node)) {
	fn(x)
}

func tForwardViaFuncValue() *int {
	b := &Node{}
	b.child = &Leaf{}
	fwdViaFuncValue(b, writeNilField)
	return b.child.ptr
}

// False negative: a struct-returning call forwarded through a return (`return f()`) is unsupported.
// The callee's per-field return summary is not composed across the forward, so the nil child from
// makeNilNode is not tracked at the caller. Supporting it would need the return-read demand closed
// over forwarding edges (as closeParamFieldSets does for params), which is not computed for returns.
func makeNilNode() *Node { return &Node{} }

func forwardNilNode() *Node { return makeNilNode() }

func tForwardedReturnNotComposed() *int {
	b := forwardNilNode()
	return b.child.ptr
}

// False negative: a struct-returning call used directly as an argument is not composed into the
// parameter boundary. Return-effects support must bind makeNilNodeForParam's field summary to n.
func makeNilNodeForParam() *Node { return &Node{} }

func readReturnedParam(n *Node) *int { return n.child.ptr }

func tReturnedCallAsParam() *int {
	return readReturnedParam(makeNilNodeForParam())
}

// False negative: the same missing return-to-boundary composition applies to a method receiver.
func makeNilNodeForReceiver() *Node { return &Node{} }

func (n *Node) readReturnedReceiver() *int { return n.child.ptr }

func tReturnedCallAsReceiver() *int {
	return makeNilNodeForReceiver().readReturnedReceiver()
}

// ---------------------------------------------------------------------------------------------
// FALSE POSITIVES (over-reports — //want required)
// ---------------------------------------------------------------------------------------------

// False positive: the same variable passed to several parameters. The caller re-produces the field
// from the first parameter's site, so the first parameter's nil write is reported even though the
// last write through the alias is non-nil. The field is non-nil at allocation and the alias's last
// write is non-nil, so the first parameter's write is the only possible nil source.
func initFirstNilSecondNonNil(a1 *Node, a2 *Node) {
	a1.child = nil     // first param's write (the spurious winner)
	a2.child = &Leaf{} // a1 and a2 alias the same object, so this is the real final value (non-nil)
}

func tSameVarMultiParam() *int {
	a := &Node{child: &Leaf{}} // non-nil at allocation: isolates the param write as the only nil source
	initFirstNilSecondNonNil(a, a)
	return a.child.ptr //want "field `child` of param 0 of `initFirstNilSecondNonNil`"
}

// False positive: a field written nil only inside one branch is summarized as unconditionally
// nilable, so a caller on the path where the branch is not taken is still flagged.
func condWriteSingleBranch(x *Node, b bool) {
	if b {
		x.child = nil
	}
}

func tConditionalWrite() *int {
	a := &Node{}
	a.child = &Leaf{}               // caller-initialized: non-nil
	condWriteSingleBranch(a, false) // b == false: child is NOT written, stays non-nil
	return a.child.ptr              //want "field `child` of param 0 of `condWriteSingleBranch`"
}

// False positive: both branches write the field, but the summary merges them and keeps the nilable
// value, so a caller that takes the non-nil branch is wrongly flagged.
func condBranchOneNil(x *Node, b bool) {
	if b {
		x.child = nil // one branch nils the field...
	} else {
		x.child = &Leaf{} // ...the other sets it non-nil; the merge keeps nilable
	}
}

func tBranchConservativeMerge() *int {
	a := &Node{child: &Leaf{}} // non-nil at allocation
	condBranchOneNil(a, false) // b == false: child is really non-nil at runtime
	return a.child.ptr         //want "field `child` of param 0 of `condBranchOneNil`"
}
