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
Package local
This test checks if the struct initialization checker catches use of uninitialized fields in the structs.
<nilaway struct enable>
*/
package local

// Tests nil flow within a single function

type A struct {
	ptr  *int
	aptr *A
}

func m() {
	var b = &A{}
	// TODO: all errors because of "aptr" unassigned at struct initialization are grouped and reported on the line below,
	// which is not correct. This should be fixed after https://github.com/uber-go/nilaway/issues/29 is implemented,
	// and struct init producer expressions are updated accordingly with the original AST expressions.
	// ERR_GROUP: represents a group of errors that are reported on the next line
	print(b.aptr.ptr) //want "unassigned at struct initialization (.|\n)* potential nil panic\\(s\\) at 6 other place\\(s\\)"
}

func m2() {
	var b = A{}
	print(b.aptr.ptr) // (error here grouped with ERR_GROUP)
}

func m3() {
	var b = &A{aptr: new(A)}
	print(b.aptr.ptr)
}

func m4() {
	var b = &A{aptr: &A{}}
	print(b.aptr.ptr)
}

func m5() {
	var b = A{aptr: new(A)}
	print(b.aptr.ptr)
}

// Initialization without explicit field key
func m6() {
	var b = A{new(int), new(A)}
	print(b.aptr.ptr)
}

func m7() {
	b := &A{}
	print(b.aptr.ptr) // (error here grouped with ERR_GROUP)
}

func m8() {
	b := &A{aptr: new(A)}
	print(b.aptr.ptr)
}

func m9() {
	var b *A
	b = &A{}
	print(b.aptr.ptr) // (error here grouped with ERR_GROUP)
}

func m10() {
	var b *A
	b = &A{aptr: new(A)}
	print(b.aptr.ptr)
}

func m14() {
	b := new(A)
	print(b.aptr.ptr) // (error here grouped with ERR_GROUP)
}

func m15() {
	var b A
	print(b.aptr.ptr) // (error here grouped with ERR_GROUP)
}

// this test checks that we only get error for `b` being nil, and not for its uninitialized fields
func m16() {
	var b *A
	print(b.aptr.ptr) //want "read from a variable that was never assigned to"
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
	*A11
}

func m11() {
	var b = &A11{}
	print(b.A11.ptr) // (error here grouped with ERR_GROUP)
}

// Tests use of promoted fields
// similar to the previous test

// TODO: Add support for promoted fields
// This test should give an error

type B13 struct {
	A13
}

type A13 struct {
	aptr *A13
	ptr  *int
}

func m13() {
	var b = &B13{}
	// This should actually give an error
	print(b.aptr.ptr)
}
