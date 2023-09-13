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
Package funcreturnfields Tests when nilability flows through the field of return of a function or a method
<nilaway struct enable>
*/
package funcreturnfields

import (
	"go.uber.org/structinit/multipackage/packageone"
)

// In this test, field aptr is initialized in giveA() function and thus no error should be reported

// RHS giveA() in this case is a stable expression that is trackable. Though the high-level logic does not change,
// for stable and not-stable function calls producers are added at distinct places.

type A11 struct {
	ptr  *int
	aptr *A11
}

func giveA() *A11 {
	t := &A11{}
	t.aptr = &A11{}
	return t
}

func m11() *int {
	var b = giveA()
	return b.aptr.ptr
}

// In this test, field aptr is set to nil in giveEmptyA() function and thus error should be reported
// giveEmptyA() is a stable expression

type A12 struct {
	ptr  *int
	aptr *A12
}

func giveEmptyA12() *A12 {
	t := &A12{}
	return t
}

func m12() *int {
	var b = giveEmptyA12()
	return b.aptr.ptr //want "uninitialized"
}

// Testing function with multiple returns
func giveOneEmptyAndOneNonEmptyA12() (*A12, *A12) {
	t1 := &A12{}
	t1.aptr = new(A12)

	t2 := &A12{}

	return t1, t2
}

func m123() {
	var b1, b2 = giveOneEmptyAndOneNonEmptyA12()
	print(b1.aptr.ptr, b2.aptr.ptr) //want "uninitialized"
}

// Testing function with multiple returns
func giveTwoEmptyA12() (*A12, *A12) {
	t1 := &A12{}
	return t1, t1
}

func m124() {
	var b1, b2 = giveTwoEmptyA12()
	print(b1.aptr.ptr, b2.aptr.ptr) //want "uninitialized" "uninitialized"
}

// Testing function with multiple returns
func giveTwoNonEmptyA12() (*A12, *A12) {
	t1 := &A12{aptr: new(A12)}

	return t1, t1
}

func m125() {
	var b1, b2 = giveTwoNonEmptyA12()
	print(b1.aptr.ptr, b2.aptr.ptr)
}

// In this test, rhs giveEmptyA122(someInt) is not a stable expression
func giveEmptyA122(someInt int) *A12 {
	t := &A12{}
	return t
}

func m122(someInt int) *int {
	var b = giveEmptyA122(someInt)
	return b.aptr.ptr //want "accessed field `ptr`"
}

// In this test case, B12 is named type

type B12 A12

func giveEmptyB12() *B12 {
	t := &B12{}
	return t
}

func mb12() *int {
	var b = giveEmptyB12()
	return b.aptr.ptr //want "accessed field `ptr`"
}

// In this test case, B122 is named type

type B122 = A12

func giveEmptyB122() *B122 {
	t := &B122{}
	return t
}

func mb122() *int {
	var b = giveEmptyB122()
	return b.aptr.ptr //want "accessed field `ptr`"
}

// In the following test case we have an anonymous field from different package

type fakeSchema struct {
	packageone.S
}

func slice() packageone.S {
	f := &fakeSchema{}
	return f.S
}

func m3() {
	print(slice()[0]) //want "sliced into"
}
