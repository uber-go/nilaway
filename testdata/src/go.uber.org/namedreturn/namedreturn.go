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
This is a test to check handling of named returns. A named return value is a return value specified in a function
signature with a variable. Nilness is checked for a "raw" return (an empty return statement) with implicit named return
values
*/
package namedreturn

// testNamedReturns derefs the results of the functions below, making their consumed results
// non-nil, so every return path yielding nil in a consumed result is reported here.
// NOTE: this caller is intentionally declared BEFORE the producing functions: inference processes
// declarations in source order, so the derefs here determine the result sites non-nil first, and
// then each unsafe return path in the producers below creates its own conflict (one diagnostic
// per unsafe return path, all reported at the derefs here).
func testNamedReturns() {
	print(*foo1()) //want "returned from `foo1.*` via named return `i`"

	i3, j3 := foo3()
	print(*i3)
	print(*j3) //want "returned from `foo3.*` via named return `j`"

	_, i4, s4, _ := foo4(0, "")
	print(*i4) //want "returned from `foo4.*` in position 1" "named return `i`" "named return `i`" "named return `i`"
	print(*s4) //want "returned from `foo4.*` in position 2" "named return `s`" "named return `s`" "named return `s`"

	print(*foo5(0)) //want "returned from `foo5.*` via named return `i`"

	i6, j6 := foo6()
	print(*i6)
	print(*j6) //want "returned from `foo6.*` via named return `j`"

	i7, j7 := foo7()
	print(*i7) //want "returned from `foo7.*` via named return `i`"
	print(*j7) //want "returned from `foo7.*` via named return `j`"
}

func foo1() (i *int) {
	return
}

// foo2 is never consumed, so its nil named return flows nowhere and is safe.
func foo2() (i *int) {
	return
}

func foo3() (i, j *int) {
	x := 1
	i = &x
	return
}

func foo4(x int, y string) (k bool, i *int, s *string, a []int) {
	switch x {
	case 0:
		return k, i, s, a
	case 1:
		i = &x
		return // (error is reported for `s` at the deref of the result)
	case 2:
		s = &y
		return // (error is reported for `i` at the deref of the result)
	case 3:
		i = &x
		s = &y
		return
	case 4:
		a = make([]int, 5)
		return // (error is reported for `i` and `s` at the deref of the result)
	}
	return // (error is reported for `i` and `s` at the deref of the result)
}

func foo5(n int) (i *int) {
	if n > 0 {
		x := 1
		i := &x
		return i
	}
	return
}

func foo6() (i, j *int) {
	x := 1
	i, k := &x, 0
	func(...any) {}(i, k)
	return
}

func foo7() (i, j *int) {
	x := 1
	if true {
		i := &x
		func(any) {}(i)
	}
	return
}

// testFoo8 is also declared before its producer foo8 (see the NOTE on testNamedReturns).
func testFoo8() {
	print(*foo8("")) //want "returned from `foo8.*` via named return `_`"
}

func foo8(x string) (_ *int) {
	return
}

type myErr struct{}

func (myErr) Error() string { return "myErr message" }

// testErrRets is also declared before its producers foo9, foo10, and foo11 (see the NOTE on
// testNamedReturns).
func testErrRets() {
	if v, err := foo9(); err == nil {
		print(*v)
	}
	if v, err := foo10(); err == nil {
		print(*v)
	}
	if v, err := foo11(); err == nil {
		print(*v) //want "returned from `foo11.*` via named return `x`"
	}
}

func foo9() (_ *int, e error) {
	e = &myErr{}
	return
}

func foo10() (x *int, _ error) {
	i := 0
	x = &i
	return
}

func foo11() (x *int, _ error) {
	return
}

var dummy bool

func takesNonnilRetsNilable(x *int) *int {
	print(*x) //want "dereferenced" "dereferenced" "dereferenced"
	if dummy {
		return nil
	}
	return x
}

// testRetsNonnilNilableWithErr derefs the first result of each function below when no error is
// returned. Since the callers here demand a non-nil first result, every return path that yields a
// nil first result with a nil error is reported at the deref below (one diagnostic per unsafe
// return path - see the NOTE on testNamedReturns for the ordering requirement). `x` is passed nil
// to keep it nilable (as in the original test), while `y` is always passed a non-nil value.
func testRetsNonnilNilableWithErr(cond bool) {
	if r0, _, err := retsNonnilNilableWithErr1(nil, new(int)); err == nil {
		print(*r0)
	}
	if r0, _, err := retsNonnilNilableWithErr2(nil, new(int)); err == nil {
		print(*r0)
	}
	if r0, _, err := retsNonnilNilableWithErr3(nil, new(int)); err == nil {
		print(*r0) //want "named return `r0`"
	}
	if r0, _, err := retsNonnilNilableWithErr4(nil, new(int)); err == nil {
		print(*r0) //want "named return `r0`"
	}
	if r0, _, err := retsNonnilNilableWithErr5(nil, new(int)); err == nil {
		print(*r0) //want "named return `r0`"
	}
	if r0, _, err := retsNonnilNilableWithErr6(nil, new(int)); err == nil {
		print(*r0) //want "named return `r0`"
	}
	if r0, _, err := retsNonnilNilableWithErr7(nil, new(int)); err == nil {
		print(*r0)
	}
	if r0, _, err := retsNonnilNilableWithErr8(nil, new(int)); err == nil {
		print(*r0)
	}
	if r0, _, err := retsNonnilNilableWithErr9(nil, new(int), cond); err == nil {
		print(*r0) //want "named return `r0`" "named return `r0`" "named return `r0`" "named return `r0`"
	}
}

func retsNonnilNilableWithErr1(x *int, y *int) (r0 *int, r1 *int, e error) {
	i := 0
	switch 0 {
	case 1:
		// this safe case indicates that if we return non-nil as our error,
		// we can return nilable values in non-nil results without error
		e = &myErr{}
		return
	case 2:
		// this is the same safe case as above, but involving flow from a nilable param
		r0 = x
		e = &myErr{}
		return
	case 3:
		// this is safe
		r0 = &i
		e = &myErr{}
		return
	case 4:
		// this is safe
		r0 = y
		e = &myErr{}
		return
	}
	return &i, &i, nil
}

func retsNonnilNilableWithErr2(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this checks that even if a non-nil error aborts the consumption of the other returns,
	// the other returns are still checked for inner illegal consumptions
	r0 = takesNonnilRetsNilable(nil)
	e = &myErr{}
	return
}

func retsNonnilNilableWithErr3(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this error case indicates that if we return nil as our error and as a
	// result that is dereferenced by a caller, that result will be interpreted as an error
	return
}

func retsNonnilNilableWithErr4(x *int, y *int) (r0 *int, r1 *int, e error) {
	i := 0
	switch 0 {
	case 7:
		// this is the same error case as above, but involving flow from a nilable param
		r0 = x
		return
	case 8:
		// this is safe
		r0 = &i
		return
	case 9:
		// this is safe
		r0 = y
		return
	}
	return &i, &i, nil
}

func retsNonnilNilableWithErr5(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this illustrates that an unassigned local error variable is interpreted as nil based on its zero value
	var e2 error
	e = e2
	return
}

func retsNonnilNilableWithErr6(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this is similar to the above case - but makes sure that computations in non-error results
	// are not ignored
	r0 = takesNonnilRetsNilable(nil)
	return
}

func retsNonnilNilableWithErr7(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this illustrates that the checking for nilable results really is flow sensitive
	// here, we determine that `e` is non-nil making it a valid error that suppresses consumption
	// of the other returns
	if e != nil {
		return
	}
	return new(int), new(int), nil
}

func retsNonnilNilableWithErr8(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this is similar to the above case - but makes sure that computations in non-error results
	// are not ignored
	if e != nil {
		r0 = takesNonnilRetsNilable(nil)
		return
	}
	return new(int), new(int), nil
}

func retsNonnilNilableWithErr9(x *int, y *int, cond bool) (r0 *int, r1 *int, e error) {
	if cond {
		// this case further tests the flow-sensitivity of the error result
		if e != nil {
			if dummy {
				return
			}
			if dummy {
				if dummy {
					return
				}
				if dummy {
					if dummy {
						return
					}
					e = nil
					if dummy {
						return // (error is reported for `r0` at the deref of the result)
					}
				}
				if dummy { // here - two different flows result in a nilable or non-nil value for e
					return // (error is reported for `r0` at the deref of the result)
				}
			} else {
				if dummy {
					return
				}
				if dummy {
					e = &myErr{}
				}
				if dummy {
					return
				}
			}
			if dummy {
				// here - two different flows result in a nilable or non-nil value for e
				return // (error is reported for `r0` at the deref of the result)
			}
		}
	}
	// here - two different flows result in a nilable or non-nil value for e
	return // (error is reported for `r0` at the deref of the result)
}
