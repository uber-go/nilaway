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

// Tests that a field write whose RHS is an address-of expression (`x.f = &local`) is non-nil at
// the pointer level, no matter how the pointee's value was produced. In particular, storing the
// address of an error-returning call's value result BEFORE the error check (a common
// code-generation pattern: `mr, err := f(); data.resp = &mr; if err != nil { return err }`) must
// not blame the guarded call result for the pointer-level read: `&mr` can never be nil. The
// pointee's field-level nilability must still flow (a nil field of the pointee stays reported).

package paramsideeffect

import "errors"

type payload struct {
	x   int
	ptr *int
}

type box struct {
	resp *payload
}

// makePayload is an error-returning function with a struct value result: on failure the value
// result is meaningless, but its ADDRESS is still a valid non-nil pointer.
func makePayload(fail bool) (payload, error) {
	if fail {
		return payload{}, errors.New("failed")
	}
	return payload{x: 1, ptr: new(int)}, nil
}

// Case 1 — the code-generation shape: store `&mr` before the error check, then a callee derefs
// the written field. The pointer-level deref is safe.
func storeAddrThenCheck(b *box, fail bool) error {
	mr, err := makePayload(fail)
	b.resp = &mr // stored pointer is non-nil even on the error path
	if err != nil {
		return err
	}
	return useBox(b)
}

func useBox(b *box) error {
	b.resp.x = 2 // safe: b.resp was assigned the address of a local
	return nil
}

// Case 2 — post-call read in the caller (param-out summary): the written field is non-nil after
// the call regardless of the returned error.
func readAfterStore() int {
	b := &box{}
	_ = storeAddrThenCheck(b, true)
	return b.resp.x // safe: storeAddrThenCheck unconditionally stores a non-nil pointer
}

// Case 3 — the pointee's FIELD-level nilability still flows: a nil field of the local reaching a
// callee's deref stays reported.
func storeAddrWithNilField(b *box) {
	mr := payload{} // ptr left nil
	b.resp = &mr
	derefPayloadPtr(b)
}

func derefPayloadPtr(b *box) int {
	return *b.resp.ptr //want "dereferenced"
}

// Case 4 — over-discharge guard: a genuinely nil field write must still be reported at the
// pointer level.
func clearResp(b *box) {
	b.resp = nil
}

func readCleared() int {
	b := &box{resp: &payload{}}
	clearResp(b)
	return b.resp.x //want "field `resp` of param 0 of `clearResp`"
}

// Case 5 — local alias of the address: `resp := &mr` before the error check, deref after. The
// deref is of the alias (a non-nil address), not of the call result.
func aliasAddrOf(fail bool) int {
	mr, err := makePayload(fail)
	resp := &mr
	if err != nil {
		return 0
	}
	return resp.x // safe: resp is the address of a local
}

// Case 6 — address of a field chain rooted at a local: still a non-nil pointer.
type holder struct {
	inner payload
}

func storeAddrOfFieldChain(b *box) {
	h := holder{inner: payload{x: 3, ptr: new(int)}}
	b.resp = &h.inner
	_ = useBox(b)
}

// Case 7 — boundary of this fix (pre-existing behavior, unaffected): a POINTER-typed result
// stored before the error check is not an address-of — the stored value really is the call
// result, which is nil on the error path. The store-site read is unguarded, so the flow is
// reported at the callee's deref. (Correlating "the deref only happens on the success path"
// through a field store is a separate, unimplemented refinement.)
func makePayloadPtr(fail bool) (*payload, error) {
	if fail {
		return nil, errors.New("failed")
	}
	return &payload{x: 1}, nil
}

func storePtrThenCheck(b *box, fail bool) error {
	mrp, err := makePayloadPtr(fail)
	b.resp = mrp
	if err != nil {
		return err
	}
	return useBoxPtrResult(b)
}

func useBoxPtrResult(b *box) error {
	b.resp.x = 2 //want "lacking guarding"
	return nil
}
