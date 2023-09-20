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

// Package defaultfield tests default nilability of the fields. Since struct initialization checking currently tracks
// nilability of only depth one fields, we resort to default nilability of the field when the field read is not tracked.
// For instance, a call to x.y when x is a parameter is tracked but x.y.z is not tracked. In these untracked cases,
// the default nilability of the field is chosen based on escaping initialization analysis. The exact assumptions in the
// analysis are documented at the FldEscape consume trigger. In short, it tracks the uninitialized fields that escape the
// function.
// <nilaway struct enable>
package defaultfield

// This gives an error since field aptr escapes

type A10 struct {
	aptr *A10
	ptr  *int
}

func m() *A10 {
	// field aptr escapes uninitialized here
	return &A10{}
}

func m3(a *A10) {
	// relies on default annotation of field aptr since we don't track field at depth >=2
	print(a.aptr.aptr.ptr) //want "accessed field `ptr`"
}

// This should give an error since aptr escapes

type A11 struct {
	ptr  *int
	aptr *A11
}

func m11(c *A11) {
	print(c.aptr.aptr.ptr) //want "field `aptr` escaped"
}

func callEscape() {
	// field aptr escapes uninitialized here
	escape11(&A11{})
}

// TODO: We should only call param fields escaping if the callee function is not analyzed by NilAway
func escape11(a *A11) {
	// no-op
}

// This does not give an error since A12 never escapes

type A12 struct {
	ptr  *int
	aptr *A12
}

func m12(c *A12) {
	print(c.aptr.aptr.ptr)
}

// This gives an error since A13 can escape

type A13 struct {
	ptr  *int
	aptr *A13
}

func m13(c *A13) {
	print(c.aptr.aptr.ptr) //want "field `aptr` escaped"
}

func escape13() *A13 {
	var a = &A13{}
	return a
}

// This is another example with accessed field at depth 2, it should error as well

type A14 struct {
	ptr  *int
	bptr *B14
}

type B14 struct {
	cptr *C14
}

type C14 struct {
	ptr *int
}

func m21(c *A14) {
	// relies on default annotation of field cptr
	print(c.bptr.cptr.ptr) //want "field `cptr` escaped"
}

func escape14(a *B14) {
	// no-op
}

func createC14() {
	t := &B14{}
	escape14(t)
}
