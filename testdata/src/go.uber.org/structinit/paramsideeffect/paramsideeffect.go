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
Package paramsideeffect Tests when nilability flows through the field of param on a call to a function or a method
<nilaway struct enable>
*/
package paramsideeffect

type A struct {
	ptr    *int
	aptr   *A
	newPtr *A
}

func populate11(x *A) {
	x.newPtr = &A{}
}

func m1() *int {
	b := &A{}
	b.aptr = &A{}
	populate11(b)
	print(b.newPtr.ptr)
	return b.aptr.ptr
}

// Similar but positive test

func populate12(x *A) {
	x.newPtr = nil
}

func m12() *int {
	b := &A{}
	b.aptr = &A{}
	populate12(b)
	print(b.newPtr.ptr) //want "field `newPtr` of argument 0 to call to function `populate12`"
	return b.aptr.ptr
}

// Negative test

func (*A) populate13(x *A) {
	x.newPtr = &A{}
}

func m3() *int {
	b := &A{}
	b.aptr = &A{aptr: new(A)}
	b.aptr.populate13(b)
	print(b.newPtr.ptr)
	return b.aptr.ptr
}

// Negative test

type B struct {
	ptr1 *A
	ptr2 *A
	ptr3 *A
}

func setPtr1(a *B) {
	a.ptr1 = new(A)
}

func setPtr2(a *B) {
	a.ptr2 = new(A)
}

func setPtr3(a *B) {
	a.ptr3 = new(A)
}

func m() {
	a := &B{}
	setPtr1(a)
	print(a.ptr1.ptr)
	setPtr2(a)
	print(a.ptr1.ptr)
	print(a.ptr2.ptr)
	setPtr3(a)
	print(a.ptr1.ptr)
	print(a.ptr2.ptr)
	print(a.ptr3.ptr)
}

// If identical arguments to function call

func initializeFirst(a1 *A, a2 *A) {
	a1.aptr = a2
}
func caller() {
	a := &A{}
	initializeFirst(a, a)
	print(a.aptr.ptr)
}

// positive case
func populate14(x *A) {
	x.newPtr = nil
}

func m14() *int {
	b := &A{}
	b.newPtr = &A{}
	populate14(b)
	return b.newPtr.ptr //want "field `newPtr` of argument 0 to call to function `populate14`"
}

// negative case
func populate15(x *A) {
	x.newPtr = nil
}

func m15() *int {
	b := &A{}
	populate14(b)
	b.newPtr = &A{}
	return b.newPtr.ptr
}

// Following cases of false positives and false negatives are due to limitations of our current technique under these special cases
// TODO: Find a better to way to handle these.
// Another case of identical arguments to function call: False positive

func initializeSecond(a1 *A, a2 *A) {
	a1.aptr = nil
	a2.aptr = new(A)
}

func caller2() {
	a := &A{}
	initializeSecond(a, a)
	print(a.aptr.ptr) //want "field `aptr` of argument 0 to call to function `initializeSecond`"
}

// Another case of identical arguments to function call: False negative

func initializeSecond2(a1 *A, a2 *A) {
	a1.aptr = new(A)
	a2.aptr = nil
}

func caller3() {
	a := &A{}
	initializeSecond2(a, a)
	print(a.aptr.ptr)
}

// Test case for checking tracking of param side effects with a struct object
func testStructObject() *int {
	var b A
	populate11(&b)
	return b.newPtr.ptr
}
