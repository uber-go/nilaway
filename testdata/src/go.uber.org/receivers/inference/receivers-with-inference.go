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

package inference

import (
	"os"
)

var dummy bool

type A struct {
	f string
}

func (a *A) nilableRecv() string {
	if a == nil {
		return "<nil>"
	}
	return a.f
}

func (a *A) nonnilRecv() string {
	return a.f //want "read by method receiver" "read by method receiver"
}

func newA() *A {
	if dummy {
		return nil
	}
	return &A{}
}

func testRecv() {
	var a *A
	a.nilableRecv() // safe
	a.nonnilRecv()  // error

	a = &A{}
	a.nilableRecv() // safe
	a.nonnilRecv()  // safe

	newA().nilableRecv() // safe
	newA().nonnilRecv()  // error
}

// -----------------------------------
// the below test checks for in-scope analysis of receivers. If a receiver-based call is made to an external method,
// such as `err.Error()`, then it is treated with optimistic default, assuming the external method to be handling
// nil receivers. This can potentially result in false negatives, as shown below in the example of `err.Error()`.
// However, this is a trade-off made to avoid false positives.

func (a *A) retErr() error {
	return nil
}

func testInScope() {
	var file *os.File
	_, _ = file.Stat() // true negative, since `Stat()` is nil-safe

	var a *A
	err := a.retErr()
	print(err.Error()) //want "result 0 of `retErr.*`"
}

// -----------------------------------
// the below test checks affiliation (interface-struct) case. Currently, this is out of scope. We don't analyze affiliations
// for tracking nilable receivers, hence an error should be thrown at the call site itself following the default behavior.
// This may result in false positives, but this decision was made owing to the several challenges encountered in its implementation.

type I interface {
	foo()
}

type S struct {
	f int
}

func (s *S) foo() {
	if s == nil {
		print(-1)
	} else {
		print(s.f)
	}
}

func newI1() I {
	return nil
}

func newI2() I {
	var s *S
	return s
}

func testAffiliation() {
	// TP since it's the case of untyped nil
	newI1().foo() //want "result 0 of `newI1.*`"

	// FP since affiliations are not tracked for nilable receivers
	newI2().foo() //want "result 0 of `newI2.*`"
}

// -----------------------------------
// below tests check for non-pointer receivers. When you call a method on a non-pointer receiver (blank or named),
// Go automatically dereferences the value and passes a copy of the value to the method. This means that such receivers
// needs to be checked for nilness at the call site.

func (a *A) namedPointer() {
	_ = a.f //want "accessed field"
}

func (a A) namedNonpointer() {
	_ = a.f
}

func (*A) nonnamedPointer() {}

func (A) nonNamedNonPointer() {}

func testBlankAndNonPointerReceivers() {
	var s1, s2, s3, s4 *A
	s1.namedPointer()    // safe at call site
	s2.nonnamedPointer() // safe at call site

	// below two non-pointer cases are not safe at call site
	s3.namedNonpointer()    //want "unassigned variable"
	s4.nonNamedNonPointer() //want "unassigned variable"
}

type myErr struct{}

func (myErr) Error() string { return "myErr message" }

type E struct {
	errField error
}

func testBlankAndNonPointerReceiversForLibraryMethods() {
	var err *myErr
	print(err.Error()) //want "unassigned variable"

	var e E
	var err2 error
	e.errField = err2
	print(e.errField.Error()) //want "unassigned variable"

	e.errField = &myErr{}
	print(e.errField.Error()) // safe
}
