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

// Package defaultfield checks the escape policy for nil struct fields:
//
//  1. A nil field that escapes into analyzed code (returned to, passed to, or mutated by a function
//     NilAway can see) and is then dereferenced IS reported, at the dereference.
//  2. A nil field that escapes into unanalyzed code, or a parameter with no in-package caller, gets
//     NilAway's standard optimistic treatment and is NOT reported.
package defaultfield

type A struct {
	ptr  *int
	aptr *A2
}

type A2 struct {
	ptr  *int
	aptr *A3
}

type A3 struct {
	ptr *int
}

// ---------------------------------------------------------------------------
// Escape into analyzed code: tracked precisely and deeply, and REPORTED.
// ---------------------------------------------------------------------------

// A nil field escapes via a return value and is dereferenced deeply by the caller.
func makeNil() *A { return &A{aptr: &A2{}} } // result.aptr.aptr is nil

func derefReturned() {
	b := makeNil()
	print(b.aptr.aptr.ptr) //want "uninitialized field `aptr`"
}

// A nil field escapes via an argument into an analyzed callee that dereferences it deeply.
func sink(c *A) {
	print(c.aptr.aptr.ptr) //want "uninitialized field `aptr`"
}

func escapeIntoSink() {
	sink(&A{aptr: &A2{}}) // supplies aptr.aptr == nil
}

// A nil field escapes via a callee's side effect and is dereferenced by the caller afterwards.
func clobber(c *A) { c.aptr = nil }

func derefAfterSideEffect() {
	b := &A{aptr: &A2{}}
	clobber(b)
	print(b.aptr.ptr) //want "field `aptr`"
}

// ---------------------------------------------------------------------------
// Unknown caller / unanalyzed escape: optimistic, NOT reported (by design).
// ---------------------------------------------------------------------------

// No in-package caller constrains `c`, so this stays optimistic and is not flagged.
func unknownCaller(c *A) {
	print(c.aptr.aptr.ptr)
}

// The struct is allocated nil-fielded and escapes to an unanalyzed sink (an interface), but no
// analyzed dereference observes the nil, so it stays optimistic.
func escapeToUnanalyzed() {
	var s any = &A{} // escapes into an interface; consumer unknown
	_ = s
}

// Properly initialized along every in-package path: no error.
func neverNil(c *A) {
	if c.aptr == nil {
		return
	}
	print(c.aptr.ptr)
}
