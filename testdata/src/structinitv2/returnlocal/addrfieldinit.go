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

// Tests shallow nilability of struct fields initialized to the address of a local. The address of
// any expression is a non-nil pointer at the shallow level, so a field set to `&local` must not be
// reported as nilable merely because the local has no boundary annotation.

package returnlocal

type addrLeaf struct {
	v *int
}

type addrHolder struct {
	p *addrLeaf
}

// giveAddrOfLocalInit initializes p to the address of a local; the field holds a non-nil pointer,
// so the caller's deep deref of p.v must not fire. Without the fix getShallowExprNilabilityProducer
// follows ParseExprAsProducer's `&A{} ≡ A{}` rule into the pointee, whose unannotated local shallow
// defaults to nilable, so the field is reported nilable and the deref fires as a false positive.
func giveAddrOfLocalInit() *addrHolder {
	local := addrLeaf{v: new(int)}
	return &addrHolder{p: &local}
}

func readAddrOfLocalInit() *int {
	b := giveAddrOfLocalInit()
	return b.p.v // safe: &local is a non-nil pointer, so b.p is non-nil
}
