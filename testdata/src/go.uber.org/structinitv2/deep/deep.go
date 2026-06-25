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

// Package deep checks that field nilability is tracked at depth >= 2, both within a function and
// across return and parameter boundaries. Depth is modeled with a chain of distinct types
// (A -> A2 -> A3 -> A4).
package deep

type A struct {
	ptr  *int
	aptr *A2
}

type A2 struct {
	ptr  *int
	aptr *A3
}

type A3 struct {
	ptr  *int
	aptr *A4
}

type A4 struct {
	ptr *int
}

// --- intra-procedural depth ---

func depth2Nil() {
	a := &A{aptr: &A2{}}   // a.aptr.aptr is nil
	print(a.aptr.aptr.ptr) //want "uninitialized field `aptr`"
}

func depth2Ok() {
	a := &A{aptr: &A2{aptr: &A3{}}}
	print(a.aptr.aptr.ptr)
}

func depth3Nil() {
	a := &A{aptr: &A2{aptr: &A3{}}} // a.aptr.aptr.aptr is nil
	print(a.aptr.aptr.aptr.ptr)     //want "uninitialized field `aptr`"
}

func depth3Ok() {
	a := &A{aptr: &A2{aptr: &A3{aptr: &A4{}}}}
	print(a.aptr.aptr.aptr.ptr)
}

// --- across the return boundary ---

func giveNil() *A { return &A{aptr: &A2{}} } // result.aptr.aptr is nil

func useReturnNil() {
	b := giveNil()
	print(b.aptr.aptr.ptr) //want "uninitialized field `aptr`"
}

func giveOk() *A { return &A{aptr: &A2{aptr: &A3{}}} }

func useReturnOk() {
	b := giveOk()
	print(b.aptr.aptr.ptr)
}

// --- across the parameter boundary ---

func takesDeep(c *A) {
	print(c.aptr.aptr.ptr) //want "uninitialized field `aptr`"
}

func callDeepNil() {
	takesDeep(&A{aptr: &A2{}}) // supplies aptr.aptr == nil
}

func takesDeepOk(c *A) {
	print(c.aptr.aptr.ptr)
}

func callDeepOk() {
	takesDeepOk(&A{aptr: &A2{aptr: &A3{}}})
}

// Return summaries are forwarded through simple return chains.
func giveTopNil() *A { return &A{} }

func forwardTopNil() *A { return giveTopNil() }

func useForwardedTopReturnNil() {
	b := forwardTopNil()
	print(b.aptr.ptr) //want "field `aptr` of result 0 of `giveTopNil`"
}

// Return-read demand also applies to var declarations.
func useReturnVarDeclNil() {
	var b = giveNil()
	print(b.aptr.aptr.ptr) //want "uninitialized field `aptr`"
}

// Multi-return assignments bind the struct result by result index.
func giveNilWithFlag() (*A, bool) { return &A{aptr: &A2{}}, false }

func useMultiReturnNil() {
	b, _ := giveNilWithFlag()
	print(b.aptr.aptr.ptr) //want "uninitialized field `aptr`"
}

// Returns with a nonnil error do not bind struct fields.
type deepErr struct{}

func (deepErr) Error() string { return "deep" }

func giveErrorPathNil() (*A, error) {
	return &A{}, deepErr{}
}

func useErrorPathNil() {
	b, err := giveErrorPathNil()
	if err != nil {
		return
	}
	print(b.aptr.ptr)
}

// Returned locals bind their field shape to the return context.
func giveLocalNil() *A {
	b := &A{}
	return b
}

func useLocalReturnNil() {
	b := giveLocalNil()
	print(b.aptr.ptr) //want "uninitialized field `aptr`"
}

// Blank-assigned struct results are skipped.
func dropStructResult() {
	_, _ = giveNilWithFlag()
}

// Parameters with no field reads do not bind argument fields.
func takesNoRead(c *A) {
	_ = c
}

func callNilStructArg() {
	takesNoRead(nil)
}

// Forwarded returns skip nonnil fields in their site-to-site links.
type valueReturn struct {
	n   int
	ptr *int
}

func giveValueReturn() *valueReturn {
	return &valueReturn{}
}

func forwardValueReturn() *valueReturn {
	return giveValueReturn()
}
