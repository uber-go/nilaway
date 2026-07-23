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

// Tests param-out writes whose RHS is a field chain rooted at a local or the address of a local
// value. Such values must be resolved through their flow-sensitive producers rather than a static
// snapshot.

package paramsideeffect

type wrapper struct {
	inner *A
}

func nilA() *A { return nil }

// Case 1: field chain rooted at a function-local holding a provably nil field fires.
func setFromFieldChainNil(x *A) {
	inner := nilA()
	local := wrapper{inner: inner}
	x.aptr = local.inner
}

func readFieldChainNil() *int {
	b := &A{}
	setFromFieldChainNil(b)
	return b.aptr.ptr //want "field `aptr` of param 0 of `setFromFieldChainNil`"
}

// Case 2: address of a function-local value with the dereferenced field initialized does not
// fire. `&value` is a non-nil pointer collapsed to the pointee's trackable path, so the post-call
// deref resolves to the initialized (non-nil) producer of value.ptr.
func setFromAddressOfLocal(x *A) {
	value := A{ptr: new(int)}
	x.aptr = &value
}

func readAddressOfLocal() *int {
	b := &A{}
	setFromAddressOfLocal(b)
	return b.aptr.ptr // safe: value.ptr is initialized
}
