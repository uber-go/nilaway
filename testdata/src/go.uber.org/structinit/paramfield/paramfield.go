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
Package paramfield Tests when nilability flows through the field of param of a function or a method
<nilaway struct enable>
*/
package paramfield

type A struct {
	ptr  *int
	aptr *A
}

func callF11() {
	t := &A{aptr: &A{}}
	f11(t)
}

func f11(c *A) {
	print(c.aptr.ptr)
}

// Positive example

func callF12() {
	t := &A{}
	f12(t)
}

func f12(c *A) {
	print(c.aptr.ptr) //want "field `aptr` of argument 0 to call to function `f12`"
}

// Negative test

func callF31() {
	t1 := &A{}
	t2 := &A{aptr: &A{}}
	f31(t1, t2)
}

func f31(c *A, d *A) {
	c.aptr = &A{}
	g31(d, c)
}

func g31(c *A, d *A) {
	// Both (param 0).aptr and (param 1).aptr are initialized in all calls to g31
	print(c.aptr.ptr, d.aptr.ptr)
}

// Another negative test

func m31(c *A) {
	// c.aptr is initialized in all calls to m31
	print(c.aptr.ptr)
}

func m32() {
	d := &A{}
	d.aptr = &A{}
	m31(d)
}

func m33() {
	d := &A{aptr: &A{}}
	m31(d)
}

// Positive example with direct composite as parameter

func callF14() {
	f14(&A{})
}

func f14(c *A) {
	print(c.aptr.ptr) //want "field `aptr` of param 0 of function `f14`"
}

// Positive example with direct composite as parameter
func giveA15() *A {
	return &A{}
}

func callF15() {
	f15(giveA15())
}

func f15(c *A) {
	print(c.aptr.ptr) //want "field `aptr` of argument 0 to call to function `f15`"
}

// Negative example with direct composite as parameter

func callF16() {
	f16(&A{aptr: &A{}})
}

func f16(c *A) {
	print(c.aptr.ptr)
}

// Negative example with direct composite as parameter
func giveA17() *A {
	return &A{aptr: new(A)}
}

func callF17() {
	f17(giveA17())
}

func f17(c *A) {
	print(c.aptr.ptr)
}

// Negative example with multiple return function as a parameter
func giveA18() (*A, *A) {
	return &A{aptr: new(A)}, &A{aptr: new(A)}
}

func callF18() {
	f18(giveA18())
}

func f18(c *A, d *A) {
	print(c.aptr.ptr, d.aptr.ptr)
}

// TODO: Handle this case
// Positive example with multiple return function as a parameter
func giveA19() (*A, *A) {
	return &A{aptr: new(A)}, &A{}
}

func callF19() {
	f19(giveA19())
}

func f19(c *A, d *A) {
	// This should give a Nilaway error
	print(c.aptr.ptr, d.aptr.ptr)
}
