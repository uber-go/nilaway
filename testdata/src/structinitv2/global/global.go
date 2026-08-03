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

// Package global checks allocation-site-sensitive tracking of package-level global struct fields. A
// field is flagged when a write provably leaves it nil, or when it is nil at the declaration and is
// never written; a field that is later written non-nil is safe.
package global

type A struct {
	ptr  *int
	aptr *leaf
}

type leaf struct {
	ptr *int
}

// Positive: one function leaves a global's field nil, another reads it.

var gWrite = &A{aptr: new(leaf)} // aptr non-nil at the declaration ...

func nilTheField() { gWrite.aptr = nil } // ... but a function leaves it nil

func readWrite() {
	print(gWrite.aptr.ptr) //want "field `aptr` of global `gWrite`"
}

// Positive: a field nil at the declaration and never written is flagged.

var gDecl = &A{} // aptr omitted -> nil at the declaration, and never written

func readDecl() {
	print(gDecl.aptr.ptr) //want "field `aptr` of global `gDecl`"
}

// Under-report (documented): a lazily-initialized global guarded by `if g == nil { return }` is
// not summarized, even though initGuarded provably leaves the field nil.

var gGuarded *A = nil

func initGuarded() {
	gGuarded = &A{}
	gGuarded.aptr = nil
}

func readGuarded() {
	if gGuarded == nil {
		return
	}
	print(gGuarded.aptr.ptr) // under-report: guarded lazy global
}

// Negative: last write wins. The trailing non-nil write determines the field's value.

var gLast *A = nil

func initLast() {
	gLast = &A{}           // aptr nil here ...
	gLast.aptr = new(leaf) // ... but overwritten non-nil (last write wins)
}

func readLast() {
	if gLast == nil {
		return
	}
	print(gLast.aptr.ptr) // safe: net write leaves gLast.aptr non-nil
}

// Positive (accepted false positive): the global is mutated only through an escaped pointer, so no
// write is captured and the stale declaration shape (aptr nil) is flagged.

var gEsc = &A{} // aptr nil at the declaration, mutated only via escape below

func setup(pp **A) { *pp = &A{aptr: new(leaf)} }

func escapeIt() { setup(&gEsc) } // mutates gEsc.aptr through a pointer (non-nil)

func readEsc() {
	print(gEsc.aptr.ptr) //want "field `aptr` of global `gEsc`"
}

// Negative: fully initialized at the declaration, never written.

var gOK = &A{aptr: &leaf{ptr: new(int)}}

func readOK() {
	print(*gOK.aptr.ptr) // safe
}

// Negative: nil at the declaration but written non-nil by a function, so the read is safe.

var gInit = &A{} // aptr nil at the declaration ...

func setupInit() { gInit.aptr = new(leaf) } // ... but a function writes it non-nil

func readInit() {
	print(gInit.aptr.ptr) // safe: aptr is written non-nil
}

// Negative: a function replaces the whole sub-struct with an initialized one, so the deep read is
// safe.

var gSub = &A{aptr: new(leaf)} // aptr non-nil, aptr.ptr nil at the declaration ...

func setupSub() { gSub.aptr = &leaf{ptr: new(int)} } // ... aptr replaced, now with aptr.ptr non-nil

func readSub() {
	print(*gSub.aptr.ptr) // safe: aptr replaced with an initialized sub-struct
}
