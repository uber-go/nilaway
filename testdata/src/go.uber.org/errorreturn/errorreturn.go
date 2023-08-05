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
This package aims to test the semantics of functions that return a value of
type "error" as their last result indicating whether they exited abnormally.

These semantics are at least partially a work in progress.

<nilaway no inference>
*/
package errorreturn

import (
	"errors"
	"fmt"
)

// this tests that the default nilability for error returns is `nilable`
func retsJustErr() error {
	return nil
}

// this tests that the default nilability for error params is `nilable`
func takesJustErr(e error) {
	takesJustErr(nil)
}

var dummy bool

type myErr struct{}

func (myErr) Error() string { return "myErr message" }

// nilable(result 0)
func takesNonnilRetsNilable(x *int) *int {
	return x
}

// nilable(result 0, result 1)
func retsNilableNilableWithErr() (*int, *int, error) {
	if dummy {
		return nil, nil, nil
	}
	return nil, nil, &myErr{}
}

func retsNonnilNonnilWithErr() (*int, *int, error) {
	i := 0
	if dummy {
		return &i, &i, nil
	}
	return nil, nil, &myErr{}
}

// nilable(result 0)
func retsNilableNonnilWithErr() (*int, *int, error) {
	i := 0
	if dummy {
		return nil, &i, nil
	}
	return nil, nil, &myErr{}
}

// TODO: : check that this function body actually obeys the error contract
// nilable(x, result 1)
func retsNonnilNilableWithErr(x *int, y *int) (*int, *int, error) {
	var e2 error
	i := 0
	switch 0 {
	case 1:
		// this safe case indicates that if we return non-nil as our error,
		// we can return nilable values in non-nil results without error
		return nil, nil, myErr{}
	case 2:
		// this is the same safe case as above, but involving flow from a nilableparam
		return x, nil, myErr{}
	case 3:
		// this is safe
		return &i, nil, myErr{}
	case 4:
		// this is safe
		return y, nil, myErr{}
	case 5:
		// this checks that even if a non-nil error aborts the consumption of the other returns,
		// the other returns are still checked for inner illegal consumptions
		return takesNonnilRetsNilable(nil), nil, myErr{} //want "passed"
	case 6:
		// this error case indicates that if we return nil as our error and as a
		// non-nilable result, that result will be interpreted as an error
		return nil, nil, nil //want "returned"
	case 7:
		// this is the same error case as above, but involving flow from a param
		return x, nil, nil //want "returned"
	case 8:
		// this is safe
		return &i, nil, nil
	case 9:
		// this is safe
		return y, nil, nil
	case 10:
		// this illustrates that an unassigned local error variable is interpreted as nil based on its zero value
		var e error
		return nil, nil, e //want "returned from the function `retsNonnilNilableWithErr` in position 0"
	case 11:
		return nil, nil, e2 //want "returned from the function `retsNonnilNilableWithErr` in position 0"
	case 12:

		// this is similar to the above case - but makes sure that computations in non-error results
		// are not ignored
		return takesNonnilRetsNilable(nil), nil, e2 //want "returned from the function `retsNonnilNilableWithErr` in position 0" "passed"
	case 13:
		// this illustrates that the checking for nilable results really is flow sensitive
		// here, we determine that `e2` is non-nil making it a valid error that suppresses consumption
		// of the other returns
		if e2 != nil {
			return nil, nil, e2
		}
	case 14:
		// this is similar to the above case - but makes sure that computations in non-error results
		// are not ignored
		if e2 != nil {
			return takesNonnilRetsNilable(nil), nil, e2 //want "passed"
		}
	case 15:
		// this case further tests the flow-sensitivity of the error result
		if e2 != nil {
			if dummy {
				return nil, nil, e2
			}
			if dummy {
				if dummy {
					return nil, nil, e2
				}
				if dummy {
					if dummy {
						return nil, nil, e2
					}
					e2 = nil
					if dummy {
						return nil, nil, e2 //want "returned from the function `retsNonnilNilableWithErr` in position 0"
					}
				}
				if dummy { // here - two different flows result in a nilable (L131) or non-nil (L119) value for e2
					return nil, nil, e2 //want "returned from the function `retsNonnilNilableWithErr` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths"
				}
			} else {
				if dummy {
					return nil, nil, e2
				}
				if dummy {
					e2 = &myErr{}
				}
				if dummy {
					return nil, nil, e2
				}
			}
			if dummy {
				// here - two different flows result in a nilable (L131) or non-nil (L119, L144) value for e2
				return nil, nil, e2 //want "returned from the function `retsNonnilNilableWithErr` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths"
			}
		}
		// here - two different flows result in a nilable (L60, L131) or non-nil (L144) value for e2
		return nil, nil, e2 //want "returned from the function `retsNonnilNilableWithErr` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths"
	}

	// these cases now test the direct return of other error-returning functions
	switch 0 {
	case 1:
		return retsNilableNilableWithErr() //want "returned"
	case 2:
		return retsNilableNonnilWithErr() //want "returned"
	case 3:
		return retsNonnilNonnilWithErr()
	default:
		return retsNonnilNilableWithErr(x, y)
	}
}

func takesNonnil(any) {}

// this is mostly here to identify failures of the error checking mechanism in its most basic form
// if this test fails then the mechanism is very broken
func simpleUsesErrFunc(i int) {
	nonnilPtr, _, err := retsNonnilNilableWithErr(&i, &i)
	if err == nil {
		takesNonnil(nonnilPtr)
	}
}

func usesErrFunc() {
	i := 0
	nonnilPtr, nilablePtr, err := retsNonnilNilableWithErr(&i, &i)
	err2 := retsJustErr()

	switch 0 {
	case 1:
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 2:
		if err == nil {
			takesNonnil(nonnilPtr)
			takesNonnil(nilablePtr) //want "passed"
			return
		}
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 3:
		if err != nil {
			takesNonnil(nonnilPtr)  //want "passed"
			takesNonnil(nilablePtr) //want "passed"
			return
		}
		takesNonnil(nonnilPtr)
		takesNonnil(nilablePtr) //want "passed"
	case 6:
		if err2 == nil {
			takesNonnil(nonnilPtr)  //want "passed"
			takesNonnil(nilablePtr) //want "passed"
			return
		}
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 7:
		if dummy {
			if err != nil {
				return
			}
		} else {
			if err != nil {
				return
			}
		}
		takesNonnil(nonnilPtr)
		takesNonnil(nilablePtr) //want "passed"
	case 8:
		if dummy {
			if err == nil {
				return
			}
		} else {
			if err != nil {
				return
			}
		}
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 9:
		if dummy {
			if err != nil {
				return
			}
		} else {
			if err == nil {
				return
			}
		}
		takesNonnil(nonnilPtr)  //want "passed"
		takesNonnil(nilablePtr) //want "passed"
	case 10:
		var nilablePtr, nonnilPtr *int
		var err error
		if dummy {
			nonnilPtr, nilablePtr, err = retsNonnilNilableWithErr(&i, &i)
		} else {
			nonnilPtr, nilablePtr, err = retsNonnilNilableWithErr(&i, &i)
		}

		if err != nil {
			return
		}

		takesNonnil(nonnilPtr)
		takesNonnil(nilablePtr) //want "passed"
	case 11:
		var nonnilPtr *int
		var err error
		switch 0 {
		case 1:
			nonnilPtr, _, err = retsNonnilNilableWithErr(&i, &i)
		case 2:
			nonnilPtr, _, err = retsNonnilNonnilWithErr()
		case 3:
			_, nonnilPtr, err = retsNonnilNonnilWithErr()
		default:
			_, nonnilPtr, err = retsNilableNonnilWithErr()
		}

		if err != nil {
			return
		}

		takesNonnil(nonnilPtr)
	case 12:
		var nilablePtr, nonnilPtr *int
		var err error
		if dummy {
			nonnilPtr, nilablePtr, err = retsNonnilNilableWithErr(&i, &i)
		} else {
			nonnilPtr, nilablePtr = &i, nil
		}

		if err != nil {
			return
		}

		takesNonnil(nonnilPtr)
		takesNonnil(nilablePtr) //want "passed" "passed"
	}
}

func sometimesErrs(e error) error {
	return e
}

func testSometimesErrs(i *int, e error) (*int, error) {
	return i, sometimesErrs(e)
}

func testSometimesErrs2(e error) (*int, error) {
	return nil, sometimesErrs(e) //want "returned from the function `testSometimesErrs2` in position 0"
}

// nilable(result 0)
func testNilableAnyways1() (*int, error) {
	if dummy {
		return nil, nil
	}
	return nil, &myErr{}
}

// nilable(result 0)
func testNilableAnyways2(e error) (*int, error) {
	return nil, sometimesErrs(e)
}

func retsAnyErr() (any, error) {
	return 0, nil
}

func noop() {}

// this test checks to make sure that if a FullTrigger is generated as GuardMatched = true, but becomes
// discovered to be GuardMatched = false later (here because the path including the second `noop` and
// `err != nil` is longer than the path without it and `err == nil`) then GuardMatched is correctly
// updated to false in the final FullTriggers - yielding termination (the matched and unmatched
// triggers don't endlessly cycle through the `range x` loop) and exactly one error message
func testStableThroughLoop(x []string) any {

	for range x {
		noop()
	}

	cert, err := retsAnyErr()

	if err != nil {
		noop()
	}

	return cert //want "returned"
}

// nilable(f, g)
type A struct {
	f *A
	g *A
	e error
}

// nilable(result 1)
func retsNonnilNilableAWithErr() (*A, *A, error) {
	if dummy {
		return &A{}, nil, nil
	}
	return nil, nil, &myErr{}
}

var getInt func() int

func testTrackingThroughDeeperExprParallel() {
	a, b := &A{}, &A{}
	a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
	a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
	a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
	b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

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
		if b.e == nil {
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
		if a.e == nil {
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
		if a.e == nil && b.e == nil {
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
		if a.e == nil || b.e == nil {
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
		if b.e == nil && a.e == nil {
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
		if b.e == nil || a.e == nil {
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
	a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
	b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

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

	if b.e == nil {
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

	if a.e == nil {
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

	if a.e == nil && b.e == nil {
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

	if a.e == nil || b.e == nil {
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

	if b.e == nil && a.e == nil {
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

	if b.e == nil || a.e == nil {
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

func retsI() (I, error) {
	return &A{}, nil
}

// this tests a weird heinous case: type switches don't link their AST node variables to internal
// types.var instances, so we test to make sure that the parsing of ast.AssignStmt's as part of
// contract propagation can handle that
func errContractPassedThroughTypeSwitch() any {
	i, err := retsI()

	if err != nil {
		return &A{}
	}

	switch j := i.(type) {
	case *A:
		return j
	}
	return i
}

// NOTE: For debugging the below function for `errors.New`, change the import to "go.uber.org/errorreturn/errors" in
// this file and update `enclosingPkgRegex: "^errors$",` in `trusted_func.go` to `enclosingPkgRegex: "go.uber.org/errorreturn/errors",`.
// Do the same for debugging `fmt.Errorf`
func testErrorsAndFmtPkg(i int) (*int, error) {
	var e = errors.New("new error")
	var e2 error

	switch i {
	case 1:
		return nil, errors.New("another new error")
	case 2:
		if dummy {
			e2 = errors.New("some new error")
		}
		return nil, e2 //want "position 1 is not guaranteed to be non-nil through all paths"
	case 4:
		return nil, fmt.Errorf("some fmt error")
	case 5:
		e = fmt.Errorf("some fmt error")
		if dummy {
			return nil, e
		}
	}

	return nil, e
}

// ***** the below test checks error return inter-procedurally *****

// nilable(result 0)
func retNilErr() error {
	return nil
}

// nonnil(result 0)
func retNonNilErr() error {
	return &myErr{}
}

func retNilableErr() error {
	if dummy {
		return retNonNilErr()
	}
	return nil
}

func retNilableErrorByDefault() error {
	return retNilErr()
}

// nilable(i)
func testRetNilableErr(i *int) (*int, error) {
	return i, retNilableErr() //want "returned from the function `testRetNilableErr` in position 0 when the error return in position 1 is not guaranteed to be non-nil through all paths"
}

func testRetNilableErrorByDefault(x *int) (*int, error) {
	var err = retNilableErrorByDefault()
	if err != nil {
		return nil, err
	}
	return x, err
}

func retPtrAndErr(i int) (*int, error) {
	var x *int
	switch i {
	case 0:
		return nil, retNonNilErr()
	case 1:
		return x, retNilErr() //want "returned from the function `retPtrAndErr` in position 0"
	}
	return &i, retNilErr()
}

func testFuncRet(i int) (*int, error) {
	var errNil = retNilErr()
	var errNonNil = retNonNilErr()
	switch i {
	case 0:
		return nil, errNil //want "returned from the function `testFuncRet` in position 0"
	case 1:
		return nil, retNilErr() //want "returned from the function `testFuncRet` in position 0"
	case 2:
		return nil, errNonNil
	case 3:
		return nil, retNonNilErr()
	case 4:
		return retPtrAndErr(0)
	}
	return &i, nil
}

// ***** below test case checks error return through multiple hops and global error variable *****

// nonnil(globalErr)
var globalErr = errors.New("some global error")

func foo1() (*int, error) {
	return foo2()
}

func foo2() (*int, error) {
	return foo3()
}

func foo3() (*int, error) {
	v, err := foo4(1)
	if err != nil {
		return nil, err
	}
	y := *v + 1
	return &y, nil
}

func foo4(i int) (*int, error) {
	if dummy {
		return nil, globalErr
	}
	return &i, nil
}

func callBar() {
	if v, err := foo1(); err == nil {
		print(*v)
	}
}

// below test case checks for mixed return values when error return is nil

// nilable(result 0, result 1)
func retPtrPtrErr(i, j int) (*int, *int, error) {
	var e = retNilErr()
	switch i {
	case 0:
		return nil, nil, retNonNilErr()
	case 1:
		return &i, nil, e
	case 2:
		return nil, &j, e
	}
	return &i, &j, e
}

func callRetPtrPtrErr() {
	a, b, err := retPtrPtrErr(0, 1)
	if err != nil {
		print(err.Error())
	} else {
		print(*a) //want "(?s)returned as result 0 from the function `retPtrPtrErr` .* dereferenced"
		print(*b) //want "(?s)returned as result 1 from the function `retPtrPtrErr` .* dereferenced"
	}
}

// ***** below test cases are for functions not conforming to NilAway's idea of an "error returning function". In such cases,
// NilAway would treat them as normal returns, with no special handling for error returns. This might result in some
// false positives, but such patterns are expected to be rare in practice *****

// below test case is for a function with error as not the last return
// nilable(result 1)
func testErrInNonLastPos(i, j int) (error, *int, *int) {
	var e error
	switch i {
	case 0:
		return nil, nil, nil //want "returned from the function `testErrInNonLastPos` in position 2"
	case 1:
		return retNilErr(), &i, &j
	case 2:
		return nil, nil, &j
	case 3:
		return e, &i, nil //want "returned from the function `testErrInNonLastPos` in position 2"
	case 4:
		// the below error can be considered to be a false positive as per the error contract
		return errors.New("some error"), nil, nil //want "returned from the function `testErrInNonLastPos` in position 2"
	case 5:
		return retNonNilErr(), nil, &j
	case 6:
		// the below error can be considered to be a false positive as per the error contract
		return retNonNilErr(), &i, nil //want "returned from the function `testErrInNonLastPos` in position 2"
	}
	return retNonNilErr(), &i, &j
}

// below test case is for a function with multiple error returns
func testMultipleErrs(i int) (*int, error, error) {
	if dummy {
		return &i, nil, nil
	}
	// the below error can be considered to be a false positive
	return nil, retNonNilErr(), retNonNilErr() //want "returned from the function `testMultipleErrs` in position 0"
}
