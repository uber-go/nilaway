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

// Package local checks that uninitialized struct fields are caught within a single function.
package local

type A struct {
	ptr  *int
	aptr *leaf
}

type leaf struct {
	ptr *int
}

func m() {
	var b = &A{}
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

func m2() {
	var b = A{}
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

func m3() {
	var b = &A{aptr: new(leaf)}
	print(b.aptr.ptr)
}

func m4() {
	var b = &A{aptr: &leaf{}}
	print(b.aptr.ptr)
}

func m5() {
	var b = A{aptr: new(leaf)}
	print(b.aptr.ptr)
}

// Initialization without explicit field key
func m6() {
	var b = A{new(int), new(leaf)}
	print(b.aptr.ptr)
}

func m7() {
	b := &A{}
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

func m8() {
	b := &A{aptr: new(leaf)}
	print(b.aptr.ptr)
}

func m9() {
	var b *A
	b = &A{}
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

func m10() {
	var b *A
	b = &A{aptr: new(leaf)}
	print(b.aptr.ptr)
}

func m14() {
	b := new(A)
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

func m15() {
	var b A
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

// this test checks that we only get error for `b` being nil, and not for its uninitialized fields
func m16() {
	var b *A
	print(b.aptr.ptr) //want "unassigned variable `b`"
}

// Testing unnamed struct
func m12() {
	var x = &struct{ a *A }{a: new(A)}
	print(x.a.ptr)

	y := new(struct{ a *A })
	// TODO: unnamed struct initialization is not supported. Following line should give a warning
	print(y.a.aptr)
}

// Tests use of anonymous fields
// otherwise similar to the previous test

type A11 struct {
	ptr *int
	*leaf
}

func m11() {
	var b = &A11{}
	print(b.leaf.ptr) //want "uninitialized field `leaf`"
}

// Tests use of promoted fields
// similar to the previous test

// TODO: Add support for promoted fields
// This test should give an error

type B13 struct {
	A13
}

type A13 struct {
	aptr *leaf
	ptr  *int
}

func m13() {
	var b = &B13{}
	// This should actually give an error
	print(b.aptr.ptr)
}

// Explicit nil field initializers are treated as nil producers.
func m17() {
	b := &A{aptr: nil}
	print(b.aptr.ptr) //want "accessed field `ptr`"
}

// Parenthesized allocations are still recognized as struct allocations.
func m18() {
	b := (&A{})
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

// Non-struct parameters are ignored by the v2 field binding.
func m19(n int) {
	print(n)
}

// Explicit field reassignment to nil overrides the allocation shape.
func m22() {
	b := &A{aptr: &leaf{}}
	b.aptr = nil
	print(b.aptr.ptr) //want "accessed field `ptr`"
}

// Non-struct pointer parameters do not trigger v2 field binding.
func m23(p *int) {
	print(*p)
}

// Selector-returning calls are treated like function calls.
type localFactory struct{}

func (localFactory) newA() *A { return &A{} }

func m24() {
	b := localFactory{}.newA()
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

// Field initializers from parameters use the parameter's nilability.
func m25(l *leaf) {
	b := &A{aptr: l}
	print(b.aptr.ptr)
}
