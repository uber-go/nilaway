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
These tests are for checking inter-procedural error return in full inference mode
*/

package inference

import (
	"errors"

	"go.uber.org/errorreturn/inference/otherPkg"
)

var dummy2 bool

type myErr2 struct{}

func (myErr2) Error() string { return "myErr2 message" }

func retNilErr2() error {
	return nil
}

func retNonNilErr2() error {
	return &myErr2{}
}

// ***** the below test case checks error return via a function and assigned to a variable *****
func retPtrAndErr2(i int) (*int, error) {
	if dummy2 {
		return nil, retNonNilErr2()
	}
	return &i, retNilErr2()
}

// same as retPtrAndErr2 but with the return statements swapped. This is to check that the order of return statements
// does not affect the error return analysis
func retPtrAndErr3() (*int, error) {
	if dummy2 {
		return new(int), retNilErr2()
	}
	return nil, retNonNilErr3()
}

// duplicated from retNonNilErr2 to make a fresh instance of the function for supporting the testing of retPtrAndErr3
func retNonNilErr3() error {
	return &myErr2{}
}

func testFuncRet2(i int) (*int, error) {
	var errNil = retNilErr2()
	var errNonNil = retNonNilErr2()
	switch i {
	case 0:
		return nil, errNil // reports error here for result 0 being nil when error is also potentially nil
	case 1:
		return nil, retNilErr2() // reports error here for result 0 being nil when error is also potentially nil
	case 2:
		return nil, errNonNil
	case 3:
		return nil, retNonNilErr2()
	case 4:
		return retPtrAndErr2(0)
	case 5:
		return &i, errNil
	case 6:
		return &i, retNilErr2()
	case 7:
		return &i, errNonNil
	case 8:
		return &i, retNonNilErr2()
	case 9:
		return retPtrAndErr3()
	}
	return &i, nil
}

func calltestFuncRet2() {
	if v, err := testFuncRet2(0); err == nil {
		print(*v) //want "error return in position 1 is not guaranteed to be non-nil through all paths" "error return in position 1 is not guaranteed to be non-nil through all paths"
	}
}

// ***** below test case checks error return through multiple hops and a global error variable declared in the same package *****
var globalErr2 = errors.New("some global error")

func foo5() (*int, error) {
	return foo6()
}

func foo6() (*int, error) {
	return foo7()
}

func foo7() (*int, error) {
	v, err := foo8(1)
	if err != nil {
		return nil, err
	}
	y := *v + 1
	return &y, nil
}

func foo8(i int) (*int, error) {
	if dummy2 {
		return nil, globalErr2
	}
	return &i, nil
}

func callFoo5() {
	if v, err := foo5(); err == nil {
		print(*v)
	}
}

// ***** the below test case checks mixed nilability in the presence of a nil error return expression *****
func retPtrPtrErr(i, j int) (*int, *int, error) {
	var e = retNilErr2()
	switch i {
	case 0:
		return nil, nil, retNonNilErr2()
	case 1:
		// This constrains result 1 to be nilable, because it's returned as nilable without a nonnil error,
		// which conflicts with the usage of the result in callRetPtrPtrErr() below, even after checking
		// for err
		return &i, nil, e
	case 2:
		// Similar case as above, for result 0
		return nil, &j, e
	}
	return &i, &j, nil
}

func callRetPtrPtrErr() {
	a, b, err := retPtrPtrErr(0, 1)
	if err != nil {
		print(err.Error())
	} else {
		// Even with error checking, these are nilable pointers (see retPtrPtrErr above)!
		_, _ = *a, *b //want "error return in position 2 is not guaranteed to be non-nil through all paths" "error return in position 2 is not guaranteed to be non-nil through all paths"
	}
}

// ***** below test cases check when the error returning function and global variable are in another package *****
func testOtherPkg(i int) (*int, error) {
	if i < 0 {
		return nil, otherPkg.GlobalErrorFromOtherPkg
	}
	if i > 100 {
		return nil, otherPkg.RetErr()
	}
	return &i, nil
}

func callTestOtherPkg() {
	if x, err := testOtherPkg(0); err == nil {
		_ = *x
	}
}

// ***** below is a test case with mixed up error usage *****
func launderWrongError(i int) (*int, error) {
	v1, err1 := retPtrAndErr2(i)
	v2, err2 := retPtrAndErr2(i + 1)
	if err1 == nil {
		// v2 being returned without error checking (err2 == nil needs to be checked)
		return v2, err1
	}

	func(...any) {}(v1, err2)

	return &i, nil
}

func checkAndDeref() {
	if x, err := launderWrongError(0); err == nil {
		_ = *x //want "error return in position 1 is not guaranteed to be non-nil through all paths"
	}
}

// ***** below tests error handling logic for mixed nilable (e.g., pointer) and non-nilable (e.g., string) non-error returns *****
func retStrNilErr() (string, *int, error) {
	if dummy2 {
		return "abc", nil, nil
	}
	return "", nil, &myErr2{}
}

func retNilStrErr() (*int, string, error) {
	if dummy2 {
		return nil, "abc", nil
	}
	return nil, "", &myErr2{}
}

func testMixedReturns() {
	if _, x, err := retStrNilErr(); err == nil {
		print(*x) //want "dereferenced"
	}

	if _, x, _ := retStrNilErr(); x != nil {
		print(*x)
	}

	if x, _, err := retNilStrErr(); err == nil {
		print(*x) //want "dereferenced"
	}
}

// nonnil(result 1)
func testMixedReturnsPassToAnotherFunc() (string, *int, error) { //want "returned"
	return retStrNilErr()
}

type myPointer *int

func retAliasedNilStrErr() (string, myPointer, error) {
	if dummy2 {
		return "abc", nil, nil
	}
	return "", nil, &myErr2{}
}

func testAliasedMixedReturns() {
	if _, x, err := retAliasedNilStrErr(); err == nil {
		print(*x) //want "dereferenced"
	}

}

// ***** below tests check the handling for "always safe" cases and their variants *****

func retAlwaysNonnilPtrErr(i int) (*int, error) {
	switch i {
	case 0:
		return new(int), &myErr2{}
	case 1:
		return &i, retNonNilErr2()
	case 2:
		return new(int), retNilErr2()
	}
	return new(int), nil
}

func retAlwaysNilPtrErr(i int) (*int, error) {
	switch i {
	case 0:
		return nil, &myErr2{}
	case 1:
		return nil, retNonNilErr2()
	case 2:
		return nil, retNilErr2()
	}
	return nil, nil
}

func retSometimesNilPtrErr(i int) (*int, error) {
	switch i {
	case 0:
		return nil, &myErr2{}
	case 1:
		return nil, retNonNilErr2()
	case 2:
		return new(int), retNilErr2()
	}
	return new(int), nil
}

func testAlwaysSafe(i int) {
	switch i {
	// always safe
	case 0:
		x, _ := retAlwaysNonnilPtrErr(i)
		print(*x)
	case 1:
		if x, err := retAlwaysNonnilPtrErr(i); err != nil {
			print(*x)
		}
	case 2:
		if x, err := retAlwaysNonnilPtrErr(i); err == nil {
			print(*x)
		}
	case 3:
		x, _ := retAlwaysNonnilPtrErr(i)
		y, _ := retAlwaysNonnilPtrErr(i)
		print(*x)
		print(*y)
	case 4:
		x, errx := retAlwaysNonnilPtrErr(i)
		y, erry := retAlwaysNonnilPtrErr(i)

		if erry == nil {
			print(*x)
		}
		if errx == nil {
			print(*y)
		}

	// always unsafe
	case 5:
		x, _ := retAlwaysNilPtrErr(i)
		print(*x) //want "dereferenced"
	case 6:
		if x, err := retAlwaysNilPtrErr(i); err == nil {
			print(*x) //want "dereferenced"
		}

	// conditionally safe
	case 7:
		x, _ := retSometimesNilPtrErr(i)
		print(*x) //want "dereferenced"
	case 8:
		if x, err := retSometimesNilPtrErr(i); err == nil {
			print(*x)
		}
	}
}

// Test always safe through multiple hops. Currently, we support only immediate function call for "always safe" tracking.
// Hence, the below cases are expected to report errors.
// TODO: add support for multiple hops to address the false positives

func m1() (*int, error) {
	return m2()
}

func m2() (*int, error) {
	v, err := m3()
	if err != nil {
		// makes non-error return always non-nil
		return new(int), err
	}
	y := *v + 1
	return &y, nil
}

func m3() (*int, error) {
	if dummy2 {
		return nil, &myErr2{}
	}
	return new(int), nil
}

type S struct {
	f *int
}

func f1(i int) (*int, error) {
	switch i {
	case 0:
		// direct non-nil non-error return value
		return new(int), &myErr2{}
	case 1:
		s := &S{f: new(int)}
		// indirect non-nil non-error return value via a field read
		return s.f, nil
	case 2:
	}
	// indirect non-nil non-error return value via a function return
	return retAlwaysNonnilPtrErr(i)
}

func testAlwaysSafeMultipleHops() {
	// TODO: call to m1() should be reported as always safe. This is a false positive since currently we are limiting the
	//  "always safe" tracking to only immediate function call, not chained error returning function calls.
	v1, _ := m1()
	print(*v1) //want "dereferenced"

	// TODO: call to f1() should be reported as always safe. This is a false positive since currently we are limiting the
	// analysis of "return statements" to only the directly determinable cases (e.g., new(int), &S{}, NegativeNilCheck), not through multiple hops.
	v2, _ := f1(0)
	print(*v2) //want "dereferenced"
}
