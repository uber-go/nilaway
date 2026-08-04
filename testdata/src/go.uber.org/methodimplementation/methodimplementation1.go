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

/*
This is a test for checking nilability-type variance. Covariance for return types and contravariance for parameter
types of methods implementing an interface.
interface method returns non-nil (caller derefs the result), implementing method returns nil --> covariance violation
interface method param nil (caller passes nil), implementing method param non-nil (body derefs it) --> contravariance violation
*/
package methodimplementation

type I interface {
	foo(x *A) (*A, string)
}

type J interface {
	bar(x *A, y *B) *string
}

type A struct {
	s string
}

type B struct {
	i int
}

func (A) foo(x *A) (*A, string) {
	var b *A
	return b, x.s //want "passed as param"
}

func (a A) bar(x *A, y *B) *string {
	if x != nil {
		return &x.s
	}
	return &a.s
}

func (b B) foo(x *A) (*A, string) {
	return x, x.s // this is safe because struct of type B is never used as the interface type I
}

func (b *B) bar(x *A, y *B) *string {
	if b.i+y.i > 5 { //want "accessed field `i`"
		return nil
	}
	return &x.s //want "passed as param"
}

func m() {
	// site 1: assignment of a concrete implementation to an interface type
	var v1 I
	v1 = &A{}
	// nil flows into the interface param, making the deref of x inside A.foo unsafe.
	r, _ := v1.foo(nil)
	// A.foo returns a nil result, making this deref of the interface result unsafe.
	_ = r.s //want "returned as result"

	var v2 J
	v2 = &B{}
	// nil flows into the interface param x, making the deref of x inside B.bar unsafe.
	v2.bar(nil, new(B))

	// a direct call passing nil as y makes the deref of y inside B.bar unsafe.
	b := &B{}
	b.bar(new(A), nil)
}
