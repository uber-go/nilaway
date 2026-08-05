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

// takesNonnilRetsNilable requires a non-nil argument (it dereferences it in its body) and returns
// a nilable result (it returns nil on some path).
func takesNonnilRetsNilable(x *int) *int {
	_ = *x //want "passed as arg" "passed as arg" "passed as arg" "passed as arg"
	if dummy {
		return nil
	}
	return x
}

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

func retsNilableNonnilWithErr() (*int, *int, error) {
	i := 0
	if dummy {
		return nil, &i, nil
	}
	return nil, nil, &myErr{}
}

// ***** the block below tests the error contract for the different kinds of return statements of
// an error-returning function. It used to be a single mega function, but is now split into
// sibling functions (dispatched to by retsNonnilNilableWithErr further below), each with a
// dedicated consumer, so that the expected errors are distributed across the dedicated
// dereference lines instead of being stacked on a single line.
//
// NOTE: each consumer is intentionally declared _before_ its producer function: the inference
// engine processes declarations in source order, so the dereference in the consumer first
// determines that result 0 of the producer must be non-nil when the error is nil. Each
// contract-violating return statement in the producer then opposes that already determined site
// and is reported as a separate conflict, stacked on the dereference line of the consumer
// (preserving one diagnostic per contract-violating return statement). *****

func derefRetsNonnilNilableWithErrBasicCases(i int) {
	nonnilPtr, _, err := retsNonnilNilableWithErrBasicCases(&i, &i)
	if err == nil {
		print(*nonnilPtr) //want "literal `nil` returned from `retsNonnilNilableWithErrBasicCases.*` in position 0" "function parameter `x` returned from `retsNonnilNilableWithErrBasicCases.*` in position 0"
	}
}

// TODO: : check that this function body actually obeys the error contract
func retsNonnilNilableWithErrBasicCases(x *int, y *int) (*int, *int, error) {
	i := 0
	switch 0 {
	case 1:
		// this safe case indicates that if we return non-nil as our error,
		// we can return nilable values in non-nil results without error
		return nil, nil, myErr{}
	case 2:
		// this is the same safe case as above, but involving flow from a nilable param
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
		return takesNonnilRetsNilable(nil), nil, myErr{}
	case 6:
		// this error case indicates that if we return nil as our error and as a
		// non-nilable result, that result will be interpreted as an error
		return nil, nil, nil
	case 7:
		// this is the same error case as above, but involving flow from a param
		return x, nil, nil
	case 8:
		// this is safe
		return &i, nil, nil
	case 9:
		// this is safe
		return y, nil, nil
	}
	return &i, &i, nil
}

func derefRetsNonnilNilableWithErrZeroValueErrs() {
	nonnilPtr, _, err := retsNonnilNilableWithErrZeroValueErrs()
	if err == nil {
		print(*nonnilPtr) //want "result 0 of `takesNonnilRetsNilable.*` returned from `retsNonnilNilableWithErrZeroValueErrs.*` in position 0" "literal `nil` returned from `retsNonnilNilableWithErrZeroValueErrs.*` in position 0" "literal `nil` returned from `retsNonnilNilableWithErrZeroValueErrs.*` in position 0"
	}
}

func retsNonnilNilableWithErrZeroValueErrs() (*int, *int, error) {
	var e2 error
	i := 0
	switch 0 {
	case 10:
		// this illustrates that an unassigned local error variable is interpreted as nil based on its zero value
		var e error
		return nil, nil, e
	case 11:
		return nil, nil, e2
	case 12:

		// this is similar to the above case - but makes sure that computations in non-error results
		// are not ignored
		return takesNonnilRetsNilable(nil), nil, e2
	}
	return &i, &i, nil
}

func derefRetsNonnilNilableWithErrFlowSensitive() {
	nonnilPtr, _, err := retsNonnilNilableWithErrFlowSensitive()
	if err == nil {
		print(*nonnilPtr) //want "returned from `retsNonnilNilableWithErrFlowSensitive.*` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths" "returned from `retsNonnilNilableWithErrFlowSensitive.*` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths" "returned from `retsNonnilNilableWithErrFlowSensitive.*` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths" "literal `nil` returned from `retsNonnilNilableWithErrFlowSensitive.*` in position 0"
	}
}

func retsNonnilNilableWithErrFlowSensitive() (*int, *int, error) {
	var e2 error
	i := 0
	switch 0 {
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
			return takesNonnilRetsNilable(nil), nil, e2
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
						return nil, nil, e2
					}
				}
				if dummy { // here - two different flows result in a nilable or non-nil value for e2
					return nil, nil, e2
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
				// here - two different flows result in a nilable or non-nil value for e2
				return nil, nil, e2
			}
		}
		// here - two different flows result in a nilable or non-nil value for e2
		return nil, nil, e2
	}
	return &i, &i, nil
}

func derefRetsNonnilNilableWithErrPassthrough(i int) {
	nonnilPtr, _, err := retsNonnilNilableWithErrPassthrough(&i, &i)
	if err == nil {
		print(*nonnilPtr) //want "result 0 of `retsNilableNilableWithErr.*` returned from `retsNonnilNilableWithErrPassthrough.*` in position 0" "result 0 of `retsNilableNonnilWithErr.*` returned from `retsNonnilNilableWithErrPassthrough.*` in position 0"
	}
}

// these cases test the direct return of other error-returning functions
func retsNonnilNilableWithErrPassthrough(x *int, y *int) (*int, *int, error) {
	switch 0 {
	case 1:
		return retsNilableNilableWithErr()
	case 2:
		return retsNilableNonnilWithErr()
	case 3:
		return retsNonnilNonnilWithErr()
	default:
		return retsNonnilNilableWithErr(x, y)
	}
}

// this is mostly here to identify failures of the error checking mechanism in its most basic form
// if this test fails then the mechanism is very broken
func simpleUsesErrFunc(i int) {
	nonnilPtr, _, err := retsNonnilNilableWithErr(&i, &i)
	if err == nil {
		print(*nonnilPtr)
	}
}

// retsNonnilNilableWithErr dispatches to the sibling functions above, combining their
// error-contract properties: result 0 is non-nil when the error is nil (demanded by the guarded
// dereferences above), while result 1 is nilable even when the error is nil.
func retsNonnilNilableWithErr(x *int, y *int) (*int, *int, error) {
	switch 0 {
	case 1:
		return retsNonnilNilableWithErrBasicCases(x, y)
	case 2:
		return retsNonnilNilableWithErrZeroValueErrs()
	case 3:
		return retsNonnilNilableWithErrFlowSensitive()
	default:
		return retsNonnilNilableWithErrPassthrough(x, y)
	}
}

// passNilToRetsNonnilNilableWithErr passes nil as the first argument to
// retsNonnilNilableWithErr, making its parameter `x` (and, through the dispatch above, the
// parameter `x` of retsNonnilNilableWithErrBasicCases) nilable under inference.
func passNilToRetsNonnilNilableWithErr(i int) {
	_, _, _ = retsNonnilNilableWithErr(nil, &i)
}

// takesNonnilInt requires a non-nil argument (it dereferences it in its body). It is kept (and
// used once in case 1 of usesErrFunc below) as a representative of consuming an error-returning
// function's result via argument passing; all other cases dereference the results in place.
func takesNonnilInt(x *int) {
	_ = *x //want "result 1 of `retsNonnilNilableWithErr.*` lacking guarding"
}

// usesErrFunc checks various patterns of guarding (or failing to guard) the results of an
// error-returning function. Each case makes its own call to retsNonnilNilableWithErr so that
// each unsafe dereference is reported individually on the line that exercises it.
func usesErrFunc() {
	i := 0

	switch 0 {
	case 1:
		// dereference / argument passing without any error check
		nonnilPtr, nilablePtr, _ := retsNonnilNilableWithErr(&i, &i)
		_ = *nonnilPtr //want "result 0 of `retsNonnilNilableWithErr.*` lacking guarding"
		takesNonnilInt(nilablePtr)
	case 2:
		// dereference after an `err == nil` check
		nonnilPtr, nilablePtr, err := retsNonnilNilableWithErr(&i, &i)
		if err == nil {
			_ = *nonnilPtr
			_ = *nilablePtr //want "returned from `retsNonnilNilableWithErr.*` in position 1"
			return
		}
		_ = *nonnilPtr  //want "result 0 of `retsNonnilNilableWithErr.*` lacking guarding"
		_ = *nilablePtr //want "result 1 of `retsNonnilNilableWithErr.*` lacking guarding"
	case 3:
		// dereference after an `err != nil` check
		nonnilPtr, nilablePtr, err := retsNonnilNilableWithErr(&i, &i)
		if err != nil {
			_ = *nonnilPtr  //want "result 0 of `retsNonnilNilableWithErr.*` lacking guarding"
			_ = *nilablePtr //want "result 1 of `retsNonnilNilableWithErr.*` lacking guarding"
			return
		}
		_ = *nonnilPtr
		_ = *nilablePtr //want "returned from `retsNonnilNilableWithErr.*` in position 1"
	case 6:
		// dereference guarded only by a check on an unrelated error
		nonnilPtr, nilablePtr, _ := retsNonnilNilableWithErr(&i, &i)
		err2 := retsJustErr()
		if err2 == nil {
			_ = *nonnilPtr  //want "result 0 of `retsNonnilNilableWithErr.*` lacking guarding"
			_ = *nilablePtr //want "result 1 of `retsNonnilNilableWithErr.*` lacking guarding"
			return
		}
		_ = *nonnilPtr  //want "result 0 of `retsNonnilNilableWithErr.*` lacking guarding"
		_ = *nilablePtr //want "result 1 of `retsNonnilNilableWithErr.*` lacking guarding"
	case 7:
		// dereference after err checks in both branches
		nonnilPtr, nilablePtr, err := retsNonnilNilableWithErr(&i, &i)
		if dummy {
			if err != nil {
				return
			}
		} else {
			if err != nil {
				return
			}
		}
		_ = *nonnilPtr
		_ = *nilablePtr //want "returned from `retsNonnilNilableWithErr.*` in position 1"
	case 8:
		// dereference after err checks with mixed polarity across branches (so the error is not
		// properly checked on all paths)
		nonnilPtr, nilablePtr, err := retsNonnilNilableWithErr(&i, &i)
		if dummy {
			if err == nil {
				return
			}
		} else {
			if err != nil {
				return
			}
		}
		_ = *nonnilPtr  //want "result 0 of `retsNonnilNilableWithErr.*` lacking guarding"
		_ = *nilablePtr //want "result 1 of `retsNonnilNilableWithErr.*` lacking guarding"
	case 9:
		// same as case 8, but with the branches mirrored
		nonnilPtr, nilablePtr, err := retsNonnilNilableWithErr(&i, &i)
		if dummy {
			if err != nil {
				return
			}
		} else {
			if err == nil {
				return
			}
		}
		_ = *nonnilPtr  //want "result 0 of `retsNonnilNilableWithErr.*` lacking guarding"
		_ = *nilablePtr //want "result 1 of `retsNonnilNilableWithErr.*` lacking guarding"
	case 10:
		// dereference of values assigned in both branches of a conditional
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

		_ = *nonnilPtr
		_ = *nilablePtr //want "returned from `retsNonnilNilableWithErr.*` in position 1"
	case 11:
		// dereference of a value assigned from multiple functions (all safe, so no errors are
		// expected here)
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

		_ = *nonnilPtr
	case 12:
		// dereference of values only partially assigned from the error-returning function
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

		_ = *nonnilPtr
		_ = *nilablePtr //want "returned from `retsNonnilNilableWithErr.*` in position 1" "literal `nil` dereferenced"
	case 13:
		// dereference of values declared via a var declaration
		var nonnilPtr, nilablePtr, err = retsNonnilNilableWithErr(&i, &i)
		if err != nil {
			return
		}
		_ = *nonnilPtr
		_ = *nilablePtr //want "returned from `retsNonnilNilableWithErr.*` in position 1"
	}
}

func sometimesErrs(e error) error {
	return e
}

func testSometimesErrs(i *int, e error) (*int, error) {
	return i, sometimesErrs(e)
}

func testSometimesErrs2(e error) (*int, error) {
	return nil, sometimesErrs(e)
}

func callTestSometimesErrs(i int) {
	// safe: the non-error return flows from the non-nil argument `&i`
	if v, err := testSometimesErrs(&i, nil); err == nil {
		_ = *v
	}

	// unsafe: testSometimesErrs2 returns nil in position 0 while its error return is not
	// guaranteed to be non-nil
	if v, err := testSometimesErrs2(nil); err == nil {
		_ = *v //want "returned from `testSometimesErrs2.*` in position 0"
	}
}

func testNilableAnyways1() (*int, error) {
	if dummy {
		return nil, nil
	}
	return nil, &myErr{}
}

func testNilableAnyways2(e error) (*int, error) {
	return nil, sometimesErrs(e)
}

func callTestNilableAnyways() {
	// safe: even though the results are nilable (even with a nil error), the derefs below are
	// guarded by explicit nil checks
	if v, err := testNilableAnyways1(); err == nil && v != nil {
		_ = *v
	}
	if v, err := testNilableAnyways2(nil); err == nil && v != nil {
		_ = *v
	}
}

// stableCert is an interface with a method so that callers can create a non-nil demand on values
// of this type (by invoking the method on them) under inference.
type stableCert interface{ certID() int }

type stableCertImpl struct{}

func (stableCertImpl) certID() int { return 0 }

func retsCertErr() (stableCert, error) {
	if dummy {
		return nil, &myErr{}
	}
	return stableCertImpl{}, nil
}

func noop() {}

// this test checks to make sure that if a FullTrigger is generated as GuardMatched = true, but becomes
// discovered to be GuardMatched = false later (here because the path including the second `noop` and
// `err != nil` is longer than the path without it and `err == nil`) then GuardMatched is correctly
// updated to false in the final FullTriggers - yielding termination (the matched and unmatched
// triggers don't endlessly cycle through the `range x` loop) and exactly one error message
func testStableThroughLoop(x []string) stableCert {

	for range x {
		noop()
	}

	cert, err := retsCertErr()

	if err != nil {
		noop()
	}

	return cert
}

func callTestStableThroughLoop(x []string) {
	print(testStableThroughLoop(x).certID()) //want "returned"
}

type A struct {
	f *A
	g *A
	e error
}

func retsNonnilNilableAWithErr() (*A, *A, error) {
	if dummy {
		return &A{}, nil, nil
	}
	return nil, nil, &myErr{}
}

var getInt func() int

// testTrackingThroughDeeperExprParallel checks the tracking of nilability through deeper
// expressions assigned via parallel assignments. Each switch arm performs its own assignments
// (so that each unsafe dereference is reported individually on the line that exercises it) and
// then dereferences the tracked expressions under a different guard.
func testTrackingThroughDeeperExprParallel() {
	switch getInt() {
	case getInt():
		// no error check
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		_ = *a
		_ = *b
		_ = *a.f
		_ = *a.g
		_ = *b.f
		_ = *b.g
		_ = *a.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
		_ = *a.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
		_ = *b.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
		_ = *b.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
	case getInt():
		// checking `b.e == nil` (guards only the first call)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if b.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
		}
	case getInt():
		// checking `a.e == nil` (guards only the second call)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if a.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *a.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
			_ = *b.f.g
			_ = *b.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
		}
	case getInt():
		// checking `a.e == nil && b.e == nil` (guards both calls)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if a.e == nil && b.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
			_ = *b.f.g
			_ = *b.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
		}
	case getInt():
		// checking `a.e == nil || b.e == nil` (guards neither call)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if a.e == nil || b.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *a.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
		}
	case getInt():
		// checking `b.e == nil && a.e == nil` (same as above `&&` case, with operands swapped)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if b.e == nil && a.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
			_ = *b.f.g
			_ = *b.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
		}
	case getInt():
		// checking `b.e == nil || a.e == nil` (same as above `||` case, with operands swapped)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if b.e == nil || a.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *a.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
		}
	}
}

// testTrackingThroughDeeperExprSeries is the same as testTrackingThroughDeeperExprParallel
// above, but with the guard scenarios laid out as sequential blocks on a single path instead of
// switch arms. Each block performs its own assignments (so that each unsafe dereference is
// reported individually on the line that exercises it).
func testTrackingThroughDeeperExprSeries() {
	{
		// no error check
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		_ = *a
		_ = *b
		_ = *a.f
		_ = *a.g
		_ = *b.f
		_ = *b.g
		_ = *a.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
		_ = *a.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
		_ = *b.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
		_ = *b.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
	}

	{
		// checking `b.e == nil` (guards only the first call)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if b.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
		}
	}

	{
		// checking `a.e == nil` (guards only the second call)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if a.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *a.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
			_ = *b.f.g
			_ = *b.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
		}
	}

	{
		// checking `a.e == nil && b.e == nil` (guards both calls)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if a.e == nil && b.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
			_ = *b.f.g
			_ = *b.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
		}
	}

	{
		// checking `a.e == nil || b.e == nil` (guards neither call)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if a.e == nil || b.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *a.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
		}
	}

	{
		// checking `b.e == nil && a.e == nil` (same as above `&&` case, with operands swapped)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if b.e == nil && a.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
			_ = *b.f.g
			_ = *b.g.f //want "returned from `retsNonnilNilableAWithErr.*` in position 1"
		}
	}

	{
		// checking `b.e == nil || a.e == nil` (same as above `||` case, with operands swapped)
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.e = retsNonnilNilableAWithErr()
		b.f.g, a.g.f, a.e = retsNonnilNilableAWithErr()

		if b.e == nil || a.e == nil {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *a.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.f.g //want "result 0 of `retsNonnilNilableAWithErr.*` lacking guarding"
			_ = *b.g.f //want "result 1 of `retsNonnilNilableAWithErr.*` lacking guarding"
		}
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
		return nil, e2
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

func callTestErrorsAndFmtPkg() {
	if v, err := testErrorsAndFmtPkg(2); err == nil {
		_ = *v //want "returned from `testErrorsAndFmtPkg.*` in position 0 when the error return in position 1 is not guaranteed to be non-nil through all paths"
	}
}

// ***** the below test checks error return inter-procedurally *****

func retNilErr() error {
	return nil
}

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

func testRetNilableErr(i *int) (*int, error) {
	return i, retNilableErr()
}

func callTestRetNilableErr() {
	if v, err := testRetNilableErr(nil); err == nil {
		_ = *v //want "returned from `testRetNilableErr.*` in position 0 when the error return in position 1 is not guaranteed to be non-nil through all paths"
	}
}

func testRetNilableErrorByDefault(x *int) (*int, error) {
	var err = retNilableErrorByDefault()
	if err != nil {
		return nil, err
	}
	return x, err
}

func callTestRetNilableErrorByDefault(i int) {
	// safe: the non-error return flows from the non-nil argument `&i`
	if v, err := testRetNilableErrorByDefault(&i); err == nil {
		_ = *v
	}
}

func retPtrAndErr(i int) (*int, error) {
	var x *int
	switch i {
	case 0:
		return nil, retNonNilErr()
	case 1:
		return x, retNilErr()
	}
	return &i, retNilErr()
}

func testFuncRet(i int) (*int, error) {
	var errNil = retNilErr()
	var errNonNil = retNonNilErr()
	switch i {
	case 0:
		return nil, errNil
	case 1:
		return nil, retNilErr()
	case 2:
		return nil, errNonNil
	case 3:
		return nil, retNonNilErr()
	case 4:
		return retPtrAndErr(0)
	}
	return &i, nil
}

func callTestFuncRet() {
	if v, err := testFuncRet(0); err == nil {
		_ = *v //want "returned from `retPtrAndErr.*` in position 0 when the error return in position 1 is not guaranteed to be non-nil through all paths" "literal `nil` returned from `testFuncRet.*` in position 0 when the error return in position 1 is not guaranteed to be non-nil through all paths" "literal `nil` returned from `testFuncRet.*` in position 0 when the error return in position 1 is not guaranteed to be non-nil through all paths"
	}
}

// ***** below test case checks error return through multiple hops and global error variable *****

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
		print(*a) //want "result 0 of `retPtrPtrErr.*` dereferenced"
		print(*b) //want "result 1 of `retPtrPtrErr.*` dereferenced"
	}
}

// ***** below test cases are for functions not conforming to NilAway's idea of an "error returning function". In such cases,
// NilAway would treat them as normal returns, with no special handling for error returns. This might result in some
// false positives, but such patterns are expected to be rare in practice *****

// below test case is for a function with error as not the last return

// NOTE: this caller is intentionally declared _before_ testErrInNonLastPos so that each
// nil-returning statement of testErrInNonLastPos is reported as a separate conflict (see the note
// on the retsNonnilNilableWithErr siblings above for details on declaration ordering).
func callTestErrInNonLastPos(i, j int) {
	// since testErrInNonLastPos is not an error-returning function per NilAway's definition, the
	// deref below is checked as a normal (unguarded) dereference of its result 2
	_, _, p := testErrInNonLastPos(i, j)
	_ = *p //want "returned from `testErrInNonLastPos.*` in position 2" "returned from `testErrInNonLastPos.*` in position 2" "returned from `testErrInNonLastPos.*` in position 2" "returned from `testErrInNonLastPos.*` in position 2"
}

func testErrInNonLastPos(i, j int) (error, *int, *int) {
	var e error
	switch i {
	case 0:
		return nil, nil, nil
	case 1:
		return retNilErr(), &i, &j
	case 2:
		return nil, nil, &j
	case 3:
		return e, &i, nil
	case 4:
		// the below error can be considered to be a false positive as per the error contract
		return errors.New("some error"), nil, nil
	case 5:
		return retNonNilErr(), nil, &j
	case 6:
		// the below error can be considered to be a false positive as per the error contract
		return retNonNilErr(), &i, nil
	}
	return retNonNilErr(), &i, &j
}

// below test case is for a function with multiple error returns
func testMultipleErrs(i int) (*int, error, error) {
	if dummy {
		return &i, nil, nil
	}
	// the below can be considered to be a false positive as per the error contract
	return nil, retNonNilErr(), retNonNilErr()
}

func callTestMultipleErrs(i int) {
	// since testMultipleErrs is not an error-returning function per NilAway's definition, the
	// deref below is checked as a normal (unguarded) dereference of its result 0
	v, _, _ := testMultipleErrs(i)
	_ = *v //want "returned from `testMultipleErrs.*` in position 0"
}

// below test case checks for the error wrapper heuristic.
func testErrorWrapper(i int) (*int, *int, error) {
	e := retNonNilErr()
	switch i {
	case 1:
		if e != nil {
			return nil, nil, Wrapf(e)
		}
	case 2:
		if e != nil {
			return takesNonnilRetsNilable(nil), nil, Wrapf(e)
		}
	case 3:
		if dummy {
			e = nil
		} else {
			e = Wrapf(e)
		}
		// here - two different flows result in a nilable or non-nil value for e2
		return nil, nil, e
	}
	return new(int), new(int), nil
}

func derefTestErrorWrapper(i int) {
	if v, _, err := testErrorWrapper(i); err == nil {
		_ = *v //want "returned from `testErrorWrapper.*`"
	}
}

func Wrapf(e error) error {
	if e == nil {
		return nil
	}
	return fmt.Errorf("wrapped: %w", e)
}

// ***** the below test cases check that an error value whose type cannot be inhabited by nil (e.g.,
// a named basic type such as `type Error string`) is correctly treated as a non-nil error. This
// addresses the false positive reported in https://github.com/uber-go/nilaway/issues/108 *****

type typedConstErr string

func (e typedConstErr) Error() string { return string(e) }

const anError typedConstErr = "this is an error"

// retTypedConstErr returns a typed-constant error in the non-nil error position. Since
// `typedConstErr` is a named basic type that nil cannot inhabit, `anError` is a valid non-nil error
// that suppresses consumption of the (nil) non-error result, so no error is expected here.
func retTypedConstErr(x bool) (*int, error) {
	if x {
		return nil, anError
	}
	i := 0
	return &i, nil
}

// callRetTypedConstErr exercises the exact pattern from issue #108: the result is dereferenced only
// on the non-error path, which must not be flagged as a potential nil panic.
func callRetTypedConstErr() error {
	ptr, err := retTypedConstErr(true)
	if err != nil {
		return err
	}
	print(*ptr)
	return nil
}

// retTypedErrVar checks the same behavior when the error flows through a variable of the named basic
// type rather than being returned directly.
func retTypedErrVar(x bool) (*int, error) {
	var e typedConstErr = "boom"
	if x {
		return nil, e
	}
	i := 0
	return &i, nil
}

func callRetTypedErrVar() {
	ptr, err := retTypedErrVar(true)
	if err == nil {
		print(*ptr)
	}
}
