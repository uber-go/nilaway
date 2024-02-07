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

// Package contracts: This file tests the contract of "ok" form for user defined and standard library functions.
//
// <nilaway no inference>
package contracts

import "runtime/debug"

// below tests check behavior of ok-form for user defined functions

func retPtrAndBool() (*int, bool) {
	if dummy {
		return nil, false
	}
	return new(int), true
}

func testSafeCases(i int) {
	switch i {
	case 0:
		if v, ok := retPtrAndBool(); ok {
			print(*v)
		}
	case 1:
		v, ok := retPtrAndBool()
		if ok {
			print(*v)
		}
	case 2:
		v, ok := retPtrAndBool()
		if ok == true {
			print(*v)
		}
	case 3:
		v, ok := retPtrAndBool()
		if !(!(!(!ok))) {
			print(*v)
		}
	case 4:
		v, ok := retPtrAndBool()
		if !ok {
			return
		}
		print(*v)
	case 5:
		v, ok := retPtrAndBool()
		var otherOk bool
		if ok && !otherOk {
			print(*v)
		}
	}
}

func testUnsafeCases(i int) {
	switch i {
	case 0:
		if v, ok := retPtrAndBool(); !ok {
			print(*v) //want "dereferenced"
		}
	case 1:
		v, ok := retPtrAndBool()
		if !ok {
			print(*v) //want "dereferenced"
		}
	case 2:
		v, ok := retPtrAndBool()
		if ok == false {
			print(*v) //want "dereferenced"
		}
	case 3:
		v, ok := retPtrAndBool()
		if !(!(!(ok))) {
			print(*v) //want "dereferenced"
		}
	case 4:
		v, ok := retPtrAndBool()
		if ok {
			return
		}
		print(*v) //want "dereferenced"
	case 5:
		v, ok := retPtrAndBool()
		var otherOk bool
		if ok || otherOk {
			print(*v) //want "dereferenced"
		}
	}
}

// below tests check behavior of ok-form for user defined functions with named returns

func retPtrAndBoolNamed() (x *int, ok bool) {
	if dummy {
		return nil, false
	}
	return new(int), true
}

func testNamedReturn(i int) {
	if v, ok := retPtrAndBoolNamed(); ok {
		print(*v)
	}

	v, ok := retPtrAndBoolNamed()
	_ = ok
	print(*v) //want "dereferenced"
}

// below test checks behavior of ok-form for user defined methods

type T struct {
	Str *string
}

func (t *T) GetStr() (*string, bool) {
	if t == nil || t.Str == nil {
		return nil, false
	}
	return t.Str, true
}

func testMethod() {
	t := &T{}
	if ptr, ok := t.GetStr(); ok {
		print(*ptr)
	}
}

// below test checks behavior of ok-form for library functions

func testLibraryFunction() {
	info, ok := debug.ReadBuildInfo()
	print(info.Path) //want "accessed field"

	if !ok {
		return
	}
	for _, kv := range info.Settings {
		_ = kv
	}
}

// below tests check behavior of ok-form for user defined functions with non-explicit boolean expression
// TODO: currently these cases result in a false positive. We plan to support them in the future.

func retTrue() bool {
	return true
}

func retPtrAndBoolExpr() (*int, bool) {
	var flag bool
	if dummy {
		// this is a false positive since we don't support non-explicit boolean expressions yet
		return nil, flag //want "literal `nil` returned"
	}
	return new(int), retTrue()
}

func testCasesWithNonExplicitBool() {
	if v, ok := retPtrAndBoolExpr(); ok {
		print(*v)
	}
}

// below tests are relevant excerpts from the `errorreturn` test suite adapted to the "ok" form for user defined functions

// nilable(result 0)
func takesNonnilRetsNilable(x *int) *int {
	return x
}

// nilable(result 0, result 1)
func retsNilableNilableWithBool() (*int, *int, bool) {
	if dummy {
		return nil, nil, true
	}
	return nil, nil, false
}

func retsNonnilNonnilWithBool() (*int, *int, bool) {
	i := 0
	if dummy {
		return &i, &i, true
	}
	return nil, nil, false
}

// nilable(result 0)
func retsNilableNonnilWithBool() (*int, *int, bool) {
	i := 0
	if dummy {
		return nil, &i, true
	}
	return nil, nil, false
}

// nilable(x, result 1)
func retsNonnilNilableWithBool(x *int, y *int, i int) (*int, *int, bool) {
	switch i {
	case 1:
		// this safe case indicates that if we return false as boolean value,
		// we can return nilable values in n-1 results without error
		return nil, nil, false
	case 2:
		// this is the same safe case as above, but involving flow from a nilable param
		return x, nil, false
	case 3:
		// this is safe
		return &i, nil, false
	case 4:
		// this is safe
		return y, nil, false
	case 5:
		// this checks that even if a false aborts the consumption of the other returns,
		// the other returns are still checked for inner illegal consumptions
		return takesNonnilRetsNilable(nil), nil, false //want "passed"
	case 6:
		// this error case indicates that if we return true and nil as a
		// non-nilable result, then that result will be interpreted as an error
		return nil, nil, true //want "returned"
	case 7:
		// this is the same error case as above, but involving flow from a param
		return x, nil, true //want "returned"
	case 8:
		// this is safe
		return &i, nil, true
	case 9:
		// this is safe
		return y, nil, true
	}

	// these cases now test the direct return of other of-form-returning functions
	switch 0 {
	case 1:
		return retsNilableNilableWithBool() //want "returned"
	case 2:
		return retsNilableNonnilWithBool() //want "returned"
	case 3:
		return retsNonnilNonnilWithBool()
	default:
		return retsNonnilNilableWithBool(x, y, i)
	}
}

func takesNonnil(any) {}

// this is mostly here to identify failures of the `ok` checking mechanism in its most basic form
// if this test fails then the mechanism is very broken
func simpleUsesBoolFunc(i int) {
	nonnilPtr, _, ok := retsNonnilNilableWithBool(&i, &i, i)
	if ok {
		takesNonnil(nonnilPtr)
	}
}

func usesBoolFunc() {
	i := 0
	nonnilPtr, nilablePtr, ok := retsNonnilNilableWithBool(&i, &i, i)
	var ok2 bool

	switch 0 {
	case 1:
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 2:
		if ok {
			takesNonnil(nonnilPtr)
			takesNonnil(nilablePtr) //want "passed"
			return
		}
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 3:
		if !ok {
			takesNonnil(nonnilPtr)  //want "passed"
			takesNonnil(nilablePtr) //want "passed"
			return
		}
		takesNonnil(nonnilPtr)
		takesNonnil(nilablePtr) //want "passed"
	case 6:
		if ok2 {
			takesNonnil(nonnilPtr)  //want "passed"
			takesNonnil(nilablePtr) //want "passed"
			return
		}
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 7:
		if dummy {
			if !ok {
				return
			}
		} else {
			if !ok {
				return
			}
		}
		takesNonnil(nonnilPtr)
		takesNonnil(nilablePtr) //want "passed"
	case 8:
		if dummy {
			if ok {
				return
			}
		} else {
			if !ok {
				return
			}
		}
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 9:
		if dummy {
			if !ok {
				return
			}
		} else {
			if ok {
				return
			}
		}
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 10:
		var nilablePtr, nonnilPtr *int
		var ok bool
		if dummy {
			nonnilPtr, nilablePtr, ok = retsNonnilNilableWithBool(&i, &i, i)
		} else {
			nonnilPtr, nilablePtr, ok = retsNonnilNilableWithBool(&i, &i, i)
		}

		if !ok {
			return
		}

		takesNonnil(nonnilPtr)
		takesNonnil(nilablePtr) //want "passed"
	case 11:
		var nonnilPtr *int
		var ok bool
		switch 0 {
		case 1:
			nonnilPtr, _, ok = retsNonnilNilableWithBool(&i, &i, i)
		case 2:
			nonnilPtr, _, ok = retsNonnilNonnilWithBool()
		case 3:
			_, nonnilPtr, ok = retsNonnilNonnilWithBool()
		default:
			_, nonnilPtr, ok = retsNilableNonnilWithBool()
		}

		if !ok {
			return
		}

		takesNonnil(nonnilPtr)
	case 12:
		var nilablePtr, nonnilPtr *int
		var ok bool
		if dummy {
			nonnilPtr, nilablePtr, ok = retsNonnilNilableWithBool(&i, &i, i)
		} else {
			nonnilPtr, nilablePtr = &i, nil
		}

		if !ok {
			return
		}

		takesNonnil(nonnilPtr)
		takesNonnil(nilablePtr) //want "passed" "passed"
	}
}

// nilable(result 0)
func retNilableAnyways() (*int, bool) {
	if dummy {
		return nil, true
	}
	return nil, false
}

func testNilableAnyways() {
	if v, ok := retNilableAnyways(); ok {
		print(*v) //want "dereferenced"
	}
}

func retsAnyBool() (any, bool) {
	return 0, true
}

func noop() {}

// this test checks to make sure that if a FullTrigger is generated as GuardMatched = true, but becomes
// discovered to be GuardMatched = false later (here because the path including the second `noop` and
// `!ok` is longer than the path without it and `ok`) then GuardMatched is correctly
// updated to false in the final FullTriggers - yielding termination (the matched and unmatched
// triggers don't endlessly cycle through the `range x` loop) and exactly one error message
func testStableThroughLoop(x []string) any {

	for range x {
		noop()
	}

	cert, ok := retsAnyBool()

	if !ok {
		noop()
	}

	return cert //want "returned"
}

// nilable(f, g)
type A struct {
	f  *A
	g  *A
	ok bool
}

// nilable(result 1)
func retsNonnilNilableAWithBool() (*A, *A, bool) {
	if dummy {
		return &A{}, nil, true
	}
	return nil, nil, false
}

func testTrackingThroughDeeperExprParallel() {
	a, b := &A{}, &A{}
	a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
	a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
	a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
	b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

	switch getInt() {
	case getInt():
		takesNonnil(a)
		takesNonnil(b)
		takesNonnil(a.f)
		takesNonnil(a.g)
		takesNonnil(b.f)
		takesNonnil(b.g)
		takesNonnil(a.f.g) //want "passed"
		takesNonnil(a.g.f) //want "passed"
		takesNonnil(b.f.g) //want "passed"
		takesNonnil(b.g.f) //want "passed"
	case getInt():
		if b.ok {
			takesNonnil(a)
			takesNonnil(b)
			takesNonnil(a.f)
			takesNonnil(a.g)
			takesNonnil(b.f)
			takesNonnil(b.g)
			takesNonnil(a.f.g)
			takesNonnil(a.g.f) //want "passed"
			takesNonnil(b.f.g) //want "passed"
			takesNonnil(b.g.f) //want "passed"
		}
	case getInt():
		if a.ok {
			takesNonnil(a)
			takesNonnil(b)
			takesNonnil(a.f)
			takesNonnil(a.g)
			takesNonnil(b.f)
			takesNonnil(b.g)
			takesNonnil(a.f.g) //want "passed"
			takesNonnil(a.g.f) //want "passed"
			takesNonnil(b.f.g)
			takesNonnil(b.g.f) //want "passed"
		}
	case getInt():
		if a.ok && b.ok {
			takesNonnil(a)
			takesNonnil(b)
			takesNonnil(a.f)
			takesNonnil(a.g)
			takesNonnil(b.f)
			takesNonnil(b.g)
			takesNonnil(a.f.g)
			takesNonnil(a.g.f) //want "passed"
			takesNonnil(b.f.g)
			takesNonnil(b.g.f) //want "passed"
		}
	case getInt():
		if a.ok || b.ok {
			takesNonnil(a)
			takesNonnil(b)
			takesNonnil(a.f)
			takesNonnil(a.g)
			takesNonnil(b.f)
			takesNonnil(b.g)
			takesNonnil(a.f.g) //want "passed"
			takesNonnil(a.g.f) //want "passed"
			takesNonnil(b.f.g) //want "passed"
			takesNonnil(b.g.f) //want "passed"
		}
	case getInt():
		if b.ok && a.ok {
			takesNonnil(a)
			takesNonnil(b)
			takesNonnil(a.f)
			takesNonnil(a.g)
			takesNonnil(b.f)
			takesNonnil(b.g)
			takesNonnil(a.f.g)
			takesNonnil(a.g.f) //want "passed"
			takesNonnil(b.f.g)
			takesNonnil(b.g.f) //want "passed"
		}
	case getInt():
		if b.ok || a.ok {
			takesNonnil(a)
			takesNonnil(b)
			takesNonnil(a.f)
			takesNonnil(a.g)
			takesNonnil(b.f)
			takesNonnil(b.g)
			takesNonnil(a.f.g) //want "passed"
			takesNonnil(a.g.f) //want "passed"
			takesNonnil(b.f.g) //want "passed"
			takesNonnil(b.g.f) //want "passed"
		}
	}
}

func testTrackingThroughDeeperExprSeries() {
	a, b := &A{}, &A{}
	a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
	a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
	a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
	b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

	takesNonnil(a)
	takesNonnil(b)
	takesNonnil(a.f)
	takesNonnil(a.g)
	takesNonnil(b.f)
	takesNonnil(b.g)
	takesNonnil(a.f.g) //want "passed"
	takesNonnil(a.g.f) //want "passed"
	takesNonnil(b.f.g) //want "passed"
	takesNonnil(b.g.f) //want "passed"

	if b.ok {
		takesNonnil(a)
		takesNonnil(b)
		takesNonnil(a.f)
		takesNonnil(a.g)
		takesNonnil(b.f)
		takesNonnil(b.g)
		takesNonnil(a.f.g)
		takesNonnil(a.g.f) //want "passed"
		takesNonnil(b.f.g) //want "passed"
		takesNonnil(b.g.f) //want "passed"
	}

	if a.ok {
		takesNonnil(a)
		takesNonnil(b)
		takesNonnil(a.f)
		takesNonnil(a.g)
		takesNonnil(b.f)
		takesNonnil(b.g)
		takesNonnil(a.f.g) //want "passed"
		takesNonnil(a.g.f) //want "passed"
		takesNonnil(b.f.g)
		takesNonnil(b.g.f) //want "passed"
	}

	if a.ok && b.ok {
		takesNonnil(a)
		takesNonnil(b)
		takesNonnil(a.f)
		takesNonnil(a.g)
		takesNonnil(b.f)
		takesNonnil(b.g)
		takesNonnil(a.f.g)
		takesNonnil(a.g.f) //want "passed"
		takesNonnil(b.f.g)
		takesNonnil(b.g.f) //want "passed"
	}

	if a.ok || b.ok {
		takesNonnil(a)
		takesNonnil(b)
		takesNonnil(a.f)
		takesNonnil(a.g)
		takesNonnil(b.f)
		takesNonnil(b.g)
		takesNonnil(a.f.g) //want "passed"
		takesNonnil(a.g.f) //want "passed"
		takesNonnil(b.f.g) //want "passed"
		takesNonnil(b.g.f) //want "passed"
	}

	if b.ok && a.ok {
		takesNonnil(a)
		takesNonnil(b)
		takesNonnil(a.f)
		takesNonnil(a.g)
		takesNonnil(b.f)
		takesNonnil(b.g)
		takesNonnil(a.f.g)
		takesNonnil(a.g.f) //want "passed"
		takesNonnil(b.f.g)
		takesNonnil(b.g.f) //want "passed"
	}

	if b.ok || a.ok {
		takesNonnil(a)
		takesNonnil(b)
		takesNonnil(a.f)
		takesNonnil(a.g)
		takesNonnil(b.f)
		takesNonnil(b.g)
		takesNonnil(a.f.g) //want "passed"
		takesNonnil(a.g.f) //want "passed"
		takesNonnil(b.f.g) //want "passed"
		takesNonnil(b.g.f) //want "passed"
	}
}

type I interface{}

func retsI() (I, bool) {
	return &A{}, true
}

// this tests a weird heinous case: type switches don't link their AST node variables to internal
// types.var instances, so we test to make sure that the parsing of ast.AssignStmt's as part of
// contract propagation can handle that
func boolContractPassedThroughTypeSwitch() any {
	i, ok := retsI()

	if !ok {
		return &A{}
	}

	switch j := i.(type) {
	case *A:
		return j
	}
	return i
}

// below test cases are for functions not conforming to NilAway's idea of a "boolean (ok) returning function". In such cases,
// NilAway would treat them as normal returns, with no special handling for boolean returns. This might result in some
// false positives, but such patterns are expected to be rare in practice

// below test case is for a function with error as not the last return
// nilable(result 1)
func testBoolInNonLastPos(i, j int) (bool, *int, *int) {
	switch i {
	case 0:
		return true, nil, nil //want "returned from `testBoolInNonLastPos.*` in position 2"
	case 1:
		return true, &i, &j
	case 2:
		return true, nil, &j
	case 3:
		return true, &i, nil //want "returned from `testBoolInNonLastPos.*` in position 2"
	case 5:
		return false, nil, &j
	case 6:
		// the below error can be considered to be a false positive as per the boolean ok-form contract
		return false, &i, nil //want "returned from `testBoolInNonLastPos.*` in position 2"
	}
	return false, &i, &j
}

// below test case is for a function with multiple boolean returns
func testMultipleBools(i int) (*int, bool, bool) {
	if dummy {
		return &i, true, true
	}
	// the below error can be considered to be a false positive
	return nil, true, false //want "returned from `testMultipleBools.*` in position 0"
}

// below cases test boolean ok-form handling logic for mixed nilable (e.g., pointer) and non-nilable (e.g., string) n-1 returns

// nilable(result 1)
func retStrNilBool() (string, *int, bool) {
	if dummy2 {
		return "abc", nil, true
	}
	return "", nil, false
}

// nilable(result 0)
func retNilStrBool() (*int, string, bool) {
	if dummy2 {
		return nil, "abc", true
	}
	return nil, "", false
}

func testMixedReturns() {
	if _, x, ok := retStrNilBool(); ok {
		print(*x) //want "dereferenced"
	}

	if _, x, _ := retStrNilBool(); x != nil {
		print(*x)
	}

	if x, _, ok := retNilStrBool(); ok {
		print(*x) //want "dereferenced"
	}
}

func testMixedReturnsPassToAnotherFunc() (string, *int, bool) {
	return retStrNilBool() //want "returned"
}

// below tests check for constants

const falseVal = false
const trueVal = true

func retPtrBoolConst() (*int, bool) {
	if dummy {
		return nil, falseVal
	}
	return new(int), trueVal
}

func testConstants() {
	// safe
	if v, ok := retPtrBoolConst(); ok {
		print(*v)
	}

	// unsafe
	if v, ok := retPtrBoolConst(); !ok {
		print(*v) //want "dereferenced"
	}
}
