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
		return nil, flag
	}
	return new(int), retTrue()
}

func testCasesWithNonExplicitBool() {
	if v, ok := retPtrAndBoolExpr(); ok {
		print(*v) //want "literal `nil` returned from `retPtrAndBoolExpr"
	}
}

func retPtrBoolShadowBuiltIn() (*int, bool) {
	if dummy {
		// this is a false positive since we don't support variables shadowing built-in types yet
		var false bool = false
		return nil, false
	}
	var true bool = true
	return new(int), true
}

func testShadowBuiltIn() {
	if v, ok := retPtrBoolShadowBuiltIn(); ok {
		print(*v) //want "literal `nil` returned from `retPtrBoolShadowBuiltIn"
	}
}

// below tests are relevant excerpts from the `errorreturn` test suite adapted to the "ok" form for user defined functions

func takesNonnilRetsNilable(x *int) *int {
	_ = *x //want "passed as arg"
	return x
}

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

func retsNilableNonnilWithBool() (*int, *int, bool) {
	i := 0
	if dummy {
		return nil, &i, true
	}
	return nil, nil, false
}

// retsNonnilNilableWithBool respects the boolean ok-form contract for result 0: it never returns
// nil in position 0 together with a true boolean, so result 0 is inferred nonnil (when guarded),
// while result 1 is inferred nilable (returned as nil with a true boolean in cases 8 and 9).
// The contract-violating behaviors (returning nil in position 0 with a true boolean) are pinned
// separately below by `retsNilWithTrue`, `retsNilableParamWithTrue`, and the passthrough tests,
// since under inference such violations would flow through every guarded consumption site.
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
		return takesNonnilRetsNilable(nil), nil, false
	case 8:
		// this is safe
		return &i, nil, true
	case 9:
		// this is safe
		return y, nil, true
	}

	// these cases now test the direct return of other ok-form-returning functions
	switch 0 {
	case 3:
		return retsNonnilNonnilWithBool()
	default:
		return retsNonnilNilableWithBool(x, y, i)
	}
}

// passesNilToNilableParam makes `x` of `retsNonnilNilableWithBool` nilable under inference; this
// is safe since `x` only flows to result 0 with a false boolean (case 2 above).
func passesNilToNilableParam(i int) {
	_, _, _ = retsNonnilNilableWithBool(nil, &i, i)
}

// retsNilWithTrue returns a literal nil in position 0 together with a true boolean, violating
// the ok-form contract (originally case 6 of retsNonnilNilableWithBool).
func retsNilWithTrue(i int) (*int, *int, bool) {
	if dummy {
		return nil, nil, true
	}
	return &i, nil, false
}

func testRetsNilWithTrue(i int) {
	if v, _, ok := retsNilWithTrue(i); ok {
		print(*v) //want "returned from `retsNilWithTrue.*` in position 0"
	}
}

// retsNilableParamWithTrue is the same violation as above, but involving flow from a nilable
// param (originally case 7 of retsNonnilNilableWithBool).
func retsNilableParamWithTrue(x *int, i int) (*int, *int, bool) {
	if dummy {
		return x, nil, true
	}
	return &i, nil, false
}

func testRetsNilableParamWithTrue(i int) {
	if v, _, ok := retsNilableParamWithTrue(nil, i); ok {
		print(*v) //want "returned from `retsNilableParamWithTrue.*` in position 0"
	}
}

// the passthrough functions below test the direct return of other ok-form-returning functions
// whose result 0 is nilable even with a true boolean (originally the passthrough cases of
// retsNonnilNilableWithBool).
func passthroughNilableNilable() (*int, *int, bool) {
	return retsNilableNilableWithBool()
}

func testPassthroughNilableNilable() {
	if v, _, ok := passthroughNilableNilable(); ok {
		print(*v) //want "returned from `passthroughNilableNilable.*` in position 0"
	}
}

func passthroughNilableNonnil() (*int, *int, bool) {
	return retsNilableNonnilWithBool()
}

func testPassthroughNilableNonnil() {
	if v, _, ok := passthroughNilableNonnil(); ok {
		print(*v) //want "returned from `passthroughNilableNonnil.*` in position 0"
	}
}

// takesNonnil is a representative arg-passing consumer: it is called from guarded (safe) sites
// in `simpleUsesBoolFunc` and `usesBoolFunc`, and from exactly one unguarded site (case 1 of
// `usesBoolFunc`), whose nil flow reports at the dereference below. All other unsafe
// consumptions in `usesBoolFunc` are in-place dereferences reporting at their own case lines.
func takesNonnil(x *int) {
	_ = *x //want "passed as arg"
}

// this is mostly here to identify failures of the `ok` checking mechanism in its most basic form
// if this test fails then the mechanism is very broken
func simpleUsesBoolFunc(i int) {
	nonnilPtr, _, ok := retsNonnilNilableWithBool(&i, &i, i)
	if ok {
		takesNonnil(nonnilPtr)
	}
}

// Each case below makes its own call to retsNonnilNilableWithBool (and friends) so that every
// unsafe in-place dereference reports individually at its own line -- dereferences consuming the
// same result of one shared call under the same guard state would be grouped into a single
// diagnostic. Dereferences of one result under different guard states (e.g., inside and after a
// guarded block) are distinct flows and report separately.
func usesBoolFunc() {
	i := 0

	switch 0 {
	case 1:
		nonnilPtr, nilablePtr, ok := retsNonnilNilableWithBool(&i, &i, i)
		_ = ok
		takesNonnil(nonnilPtr) // representative unguarded arg-passing consumption
		_ = *nilablePtr        //want "dereferenced"
	case 2:
		nonnilPtr, nilablePtr, ok := retsNonnilNilableWithBool(&i, &i, i)
		if ok {
			_ = *nonnilPtr
			_ = *nilablePtr //want "dereferenced"
			return
		}
		_ = *nonnilPtr  //want "dereferenced"
		_ = *nilablePtr //want "dereferenced"
	case 3:
		nonnilPtr, nilablePtr, ok := retsNonnilNilableWithBool(&i, &i, i)
		if !ok {
			_ = *nonnilPtr  //want "dereferenced"
			_ = *nilablePtr //want "dereferenced"
			return
		}
		_ = *nonnilPtr
		_ = *nilablePtr //want "dereferenced"
	case 6:
		nonnilPtr, nilablePtr, ok := retsNonnilNilableWithBool(&i, &i, i)
		_ = ok
		var ok2 bool
		if ok2 {
			_ = *nonnilPtr  //want "dereferenced"
			_ = *nilablePtr //want "dereferenced"
			return
		}
		_ = *nonnilPtr  //want "dereferenced"
		_ = *nilablePtr //want "dereferenced"
	case 7:
		nonnilPtr, nilablePtr, ok := retsNonnilNilableWithBool(&i, &i, i)
		if dummy {
			if !ok {
				return
			}
		} else {
			if !ok {
				return
			}
		}
		_ = *nonnilPtr
		_ = *nilablePtr //want "dereferenced"
	case 8:
		nonnilPtr, nilablePtr, ok := retsNonnilNilableWithBool(&i, &i, i)
		if dummy {
			if ok {
				return
			}
		} else {
			if !ok {
				return
			}
		}
		_ = *nonnilPtr  //want "dereferenced"
		_ = *nilablePtr //want "dereferenced"
	case 9:
		nonnilPtr, nilablePtr, ok := retsNonnilNilableWithBool(&i, &i, i)
		if dummy {
			if !ok {
				return
			}
		} else {
			if ok {
				return
			}
		}
		_ = *nonnilPtr  //want "dereferenced"
		_ = *nilablePtr //want "dereferenced"
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

		_ = *nonnilPtr
		// the two symmetric branch calls above merge into a single flow at the dereference below
		_ = *nilablePtr //want "dereferenced"
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

		_ = *nonnilPtr
		// the guarded call and the literal nil assignment above are two distinct nil flows
		_ = *nilablePtr //want "dereferenced" "dereferenced"
	}
}

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

func retsPtrBool() (*int, bool) {
	if dummy {
		return nil, false
	}
	return new(int), true
}

func noop() {}

// this test checks to make sure that if a FullTrigger is generated as GuardMatched = true, but becomes
// discovered to be GuardMatched = false later (here because the path including the second `noop` and
// `!ok` is longer than the path without it and `ok`) then GuardMatched is correctly
// updated to false in the final FullTriggers - yielding termination (the matched and unmatched
// triggers don't endlessly cycle through the `range x` loop) and exactly one error message
func testStableThroughLoop(x []string) *int {

	for range x {
		noop()
	}

	cert, ok := retsPtrBool()

	if !ok {
		noop()
	}

	return cert
}

func testStableThroughLoopCaller(x []string) {
	print(*testStableThroughLoop(x)) //want "returned from `testStableThroughLoop.*` in position 0"
}

// fields `f` and `g` are nilable under inference since they are assigned nil (and nilable
// values) in the testTrackingThroughDeeperExpr* functions below.
type A struct {
	f  *A
	g  *A
	ok bool
}

// retsNonnilNilableAWithBool has an inferred nonnil result 0 (never nil with a true boolean) and
// an inferred nilable result 1 (nil with a true boolean).
func retsNonnilNilableAWithBool() (*A, *A, bool) {
	if dummy {
		return &A{}, nil, true
	}
	return nil, nil, false
}

// The two functions below check tracking of the ok-form contract through deeper expressions
// (struct field chains). Each guard-scenario block makes its own pair of calls so that every
// unsafe in-place dereference reports individually at its own line -- dereferences consuming
// the same result of one shared call would be grouped into a single diagnostic.

func testTrackingThroughDeeperExprParallel() {
	switch getInt() {
	case getInt():
		// no guard: all four deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		_ = *a
		_ = *b
		_ = *a.f
		_ = *a.g
		_ = *b.f
		_ = *b.g
		_ = *a.f.g //want "dereferenced"
		_ = *a.g.f //want "dereferenced"
		_ = *b.f.g //want "dereferenced"
		_ = *b.g.f //want "dereferenced"
	case getInt():
		// b.ok guards only the first call: a.f.g is safe, the other deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if b.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g //want "dereferenced"
			_ = *b.g.f //want "dereferenced"
		}
	case getInt():
		// a.ok guards only the second call: b.f.g is safe, the other deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if a.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "dereferenced"
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g
			_ = *b.g.f //want "dereferenced"
		}
	case getInt():
		// both calls guarded: only the nilable results a.g.f and b.g.f are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if a.ok && b.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g
			_ = *b.g.f //want "dereferenced"
		}
	case getInt():
		// either-or guard is insufficient: all four deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if a.ok || b.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "dereferenced"
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g //want "dereferenced"
			_ = *b.g.f //want "dereferenced"
		}
	case getInt():
		// both calls guarded (swapped order): only a.g.f and b.g.f are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if b.ok && a.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g
			_ = *b.g.f //want "dereferenced"
		}
	case getInt():
		// either-or guard (swapped order) is insufficient: all four deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if b.ok || a.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "dereferenced"
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g //want "dereferenced"
			_ = *b.g.f //want "dereferenced"
		}
	}
}

func testTrackingThroughDeeperExprSeries() {
	{
		// no guard: all four deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		_ = *a
		_ = *b
		_ = *a.f
		_ = *a.g
		_ = *b.f
		_ = *b.g
		_ = *a.f.g //want "dereferenced"
		_ = *a.g.f //want "dereferenced"
		_ = *b.f.g //want "dereferenced"
		_ = *b.g.f //want "dereferenced"
	}

	{
		// b.ok guards only the first call: a.f.g is safe, the other deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if b.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g //want "dereferenced"
			_ = *b.g.f //want "dereferenced"
		}
	}

	{
		// a.ok guards only the second call: b.f.g is safe, the other deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if a.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "dereferenced"
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g
			_ = *b.g.f //want "dereferenced"
		}
	}

	{
		// both calls guarded: only the nilable results a.g.f and b.g.f are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if a.ok && b.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g
			_ = *b.g.f //want "dereferenced"
		}
	}

	{
		// either-or guard is insufficient: all four deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if a.ok || b.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "dereferenced"
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g //want "dereferenced"
			_ = *b.g.f //want "dereferenced"
		}
	}

	{
		// both calls guarded (swapped order): only a.g.f and b.g.f are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if b.ok && a.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g
			_ = *b.g.f //want "dereferenced"
		}
	}

	{
		// either-or guard (swapped order) is insufficient: all four deep expressions are unsafe
		a, b := &A{}, &A{}
		a.f, a.g, b.f, b.g = &A{}, &A{}, &A{}, &A{}
		a.f.g, a.g.f, b.f.g, b.g.f = nil, nil, nil, nil
		a.f.g, b.g.f, b.ok = retsNonnilNilableAWithBool()
		b.f.g, a.g.f, a.ok = retsNonnilNilableAWithBool()

		if b.ok || a.ok {
			_ = *a
			_ = *b
			_ = *a.f
			_ = *a.g
			_ = *b.f
			_ = *b.g
			_ = *a.f.g //want "dereferenced"
			_ = *a.g.f //want "dereferenced"
			_ = *b.f.g //want "dereferenced"
			_ = *b.g.f //want "dereferenced"
		}
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

// testBoolInNonLastPosCaller dereferences result 2 of testBoolInNonLastPos to demand it nonnil
// under inference. It is declared *before* testBoolInNonLastPos so that each nil return in
// position 2 (cases 0, 3, and 6 below) reports its own nil flow at the dereference below.
// Result 1 is nilable and stays unconsumed, so the nil returns in position 1 stay silent.
func testBoolInNonLastPosCaller(i, j int) {
	_, _, v := testBoolInNonLastPos(i, j)
	print(*v) //want "returned from `testBoolInNonLastPos.*` in position 2" "returned from `testBoolInNonLastPos.*` in position 2" "returned from `testBoolInNonLastPos.*` in position 2"
}

func testBoolInNonLastPos(i, j int) (bool, *int, *int) {
	switch i {
	case 0:
		return true, nil, nil
	case 1:
		return true, &i, &j
	case 2:
		return true, nil, &j
	case 3:
		return true, &i, nil
	case 5:
		return false, nil, &j
	case 6:
		// the below error can be considered to be a false positive as per the boolean ok-form contract
		return false, &i, nil
	}
	return false, &i, &j
}

// below test case is for a function with multiple boolean returns
func testMultipleBools(i int) (*int, bool, bool) {
	if dummy {
		return &i, true, true
	}
	// the below error can be considered to be a false positive
	return nil, true, false
}

func testMultipleBoolsCaller(i int) {
	v, _, _ := testMultipleBools(i)
	print(*v) //want "returned from `testMultipleBools.*` in position 0"
}

// below cases test boolean ok-form handling logic for mixed nilable (e.g., pointer) and non-nilable (e.g., string) n-1 returns

func retStrNilBool() (string, *int, bool) {
	if dummy2 {
		return "abc", nil, true
	}
	return "", nil, false
}

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
	return retStrNilBool()
}

func testMixedReturnsPassToAnotherFuncCaller() {
	if _, v, ok := testMixedReturnsPassToAnotherFunc(); ok {
		print(*v) //want "returned from `testMixedReturnsPassToAnotherFunc.*` in position 1"
	}
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

func retPtrBoolConstIncorrect() (*int, bool) {
	if dummy {
		return nil, trueVal
	}
	return new(int), falseVal
}

func testConstants() {
	// safe
	if v, ok := retPtrBoolConst(); ok {
		print(*v)
	}

	// unsafe
	if v, ok := retPtrBoolConst(); !ok {
		print(*v) //want "result 0 of `retPtrBoolConst.*` lacking guarding; dereferenced"
	}

	if v, ok := retPtrBoolConstIncorrect(); ok {
		print(*v) //want "dereferenced"
	}
}
