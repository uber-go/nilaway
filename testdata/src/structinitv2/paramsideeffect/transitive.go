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

// Tests transitive (forwarding) param side effects: a function that does not directly write a
// parameter's field but forwards the parameter to another function/method that does. The write must
// be attributed to the forwarder so callers re-produce the mutated field.

package paramsideeffect

// 1. Two-level forward, positive: writeNil nils x.f, fwdNil forwards x to it.

func writeNil(x *A) {
	x.newPtr = nil
}

func fwdNil(x *A) {
	writeNil(x)
}

func tForwardPositive() *int {
	b := &A{}
	b.newPtr = &A{}
	fwdNil(b)
	return b.newPtr.ptr //want "field `newPtr` of param 0 of `fwdNil`"
}

// 2. Two-level forward, negative: writeNonNil sets x.f non-nil, forwarded through fwdNonNil, so no
// diagnostic.

func writeNonNil(x *A) {
	x.newPtr = &A{}
}

func fwdNonNil(x *A) {
	writeNonNil(x)
}

func tForwardNegative() *int {
	b := &A{}
	fwdNonNil(b)
	return b.newPtr.ptr
}

// 3. Three-level chain to exercise the fixpoint past depth 2.

func writeNil3(x *A) {
	x.newPtr = nil
}

func fwd3a(x *A) {
	writeNil3(x)
}

func fwd3b(x *A) {
	fwd3a(x)
}

func tThreeLevelChain() *int {
	b := &A{}
	b.newPtr = &A{}
	fwd3b(b)
	return b.newPtr.ptr //want "field `newPtr` of param 0 of `fwd3b`"
}

// 4. Field-chain forward through a nested struct: fwdChain forwards x.mid to a writer of its leaf
// field. The caller observes the write on the nested path mid.leaf.

type inner struct {
	leaf *A
}

type outer struct {
	mid *inner
}

func writeNilLeaf(i *inner) {
	i.leaf = nil
}

func fwdChain(o *outer) {
	writeNilLeaf(o.mid)
}

func tFieldChainForward() *int {
	b := &outer{mid: &inner{leaf: &A{}}}
	fwdChain(b)
	return b.mid.leaf.ptr //want "field `mid.leaf` of param 0 of `fwdChain`"
}

// 4b. Recursive-field forward: forwarding through a self-referential field (`A.aptr *A`) is not
// supported and must be skipped — the analysis terminates and emits no diagnostic.

func writeNilRecField(x *A) {
	x.newPtr = nil
}

func fwdRecField(x *A) {
	writeNilRecField(x.aptr)
}

func tRecursiveFieldForward() *int {
	b := &A{aptr: &A{newPtr: &A{}}}
	fwdRecField(b)
	return b.aptr.newPtr.ptr // recursive field path not tracked: under-report, no want
}

// 5. Receiver forward: a method forwards its receiver to a writer method.

func (x *A) populateRecv() {
	x.newPtr = nil
}

func (x *A) fwdRecv() {
	x.populateRecv()
}

func tReceiverForward() *int {
	b := &A{}
	b.newPtr = &A{}
	b.fwdRecv()
	return b.newPtr.ptr //want "field `newPtr` of method receiver of `fwdRecv`"
}

// 5b. Symmetric: the receiver itself is forwarded as a plain argument to a writer function.

func writeNilArg(x *A) {
	x.newPtr = nil
}

func (x *A) fwdRecvAsArg() {
	writeNilArg(x)
}

func tReceiverForwardedAsArg() *int {
	b := &A{}
	b.newPtr = &A{}
	b.fwdRecvAsArg()
	return b.newPtr.ptr //want "field `newPtr` of method receiver of `fwdRecvAsArg`"
}

// 6. Recursive / cyclic forwarder: must terminate and produce the write from the base-case writer.

func writeNilRec(x *A) {
	x.newPtr = nil
}

func fwdRec(x *A, n int) {
	if n <= 0 {
		writeNilRec(x)
		return
	}
	fwdRec(x, n-1)
}

func tRecursiveForward() *int {
	b := &A{}
	b.newPtr = &A{}
	fwdRec(b, 3)
	return b.newPtr.ptr //want "field `newPtr` of param 0 of `fwdRec`"
}

// 6b. Mutual recursion (g <-> h) with a writer at the base case.

func writeNilMut(x *A) {
	x.newPtr = nil
}

func fwdMutA(x *A, n int) {
	if n <= 0 {
		writeNilMut(x)
		return
	}
	fwdMutB(x, n-1)
}

func fwdMutB(x *A, n int) {
	fwdMutA(x, n-1)
}

func tMutualRecursion() *int {
	b := &A{}
	b.newPtr = &A{}
	fwdMutA(b, 4)
	return b.newPtr.ptr //want "field `newPtr` of param 0 of `fwdMutA`"
}

// 7. Negative non-forward (regression): an opaque/no-op call between two writes must NOT mark
// unrelated fields. noop forwards nothing, so the caller's own non-nil value survives.

func noop(x *A) {
	_ = x
}

func tNonForwardNoFalsePositive() *int {
	b := &A{}
	b.newPtr = &A{}
	noop(b)
	return b.newPtr.ptr
}

// 8. Forward only re-produces the forwarded field; an unrelated field the caller initialized
// non-nil must survive the forwarding call.

func writeOneField(x *A) {
	x.aptr = nil
}

func fwdOneField(x *A) {
	writeOneField(x)
}

func tForwardOnlyAffectsWrittenField() *int {
	b := &A{}
	b.aptr = &A{}
	b.newPtr = &A{}
	fwdOneField(b)
	print(b.aptr.ptr) //want "field `aptr` of param 0 of `fwdOneField`"
	return b.newPtr.ptr
}
