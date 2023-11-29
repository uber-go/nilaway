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

<nilaway no inference>
*/
package namedreturn

func foo1() (i *int) { //want "returned from `foo1.*` via named return `i`"
	return
}

// nilable(i)
func foo2() (i *int) {
	return
}

func foo3() (i, j *int) { //want "returned from `foo3.*` via named return `j`"
	x := 1
	i = &x
	return
}

func foo4(x int, y string) (k bool, i *int, s *string, a []int) { //want "named return `s`" "named return `i`" "named return `i`" "named return `s`" "named return `i`" "named return `s`"
	switch x {
	case 0:
		return k, i, s, a //want "returned" "returned"
	case 1:
		i = &x
		return // (error is reported for `s` at function declaration)
	case 2:
		s = &y
		return // (error is reported for `i` at function declaration)
	case 3:
		i = &x
		s = &y
		return
	case 4:
		a = make([]int, 5)
		return // (error is reported for `i` and `s` at function declaration)
	}
	return // (error is reported for `i` and `s` at function declaration)
}

func foo5(n int) (i *int) { //want "named return `i`"
	if n > 0 {
		x := 1
		i := &x
		return i
	}
	return
}

func foo6() (i, j *int) { //want "named return `j`"
	x := 1
	i, k := &x, 0
	func(...any) {}(i, k)
	return
}

func foo7() (i, j *int) { //want "named return `i`" "named return `j`"
	x := 1
	if true {
		i := &x
		func(any) {}(i)
	}
	return
}

func foo8(x string) (_ *int) { //want "named return `_`"
	return
}

type myErr struct{}

func (myErr) Error() string { return "myErr message" }

func foo9() (_ *int, e error) {
	e = &myErr{}
	return
}

func foo10() (x *int, _ error) {
	i := 0
	x = &i
	return
}

func foo11() (x *int, _ error) { //want "named return `x`"
	return
}

var dummy bool

// nilable(result 0)
func takesNonnilRetsNilable(x *int) *int {
	return x
}

// nilable(x, r1)
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

// nilable(x, r1)
func retsNonnilNilableWithErr2(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this checks that even if a non-nil error aborts the consumption of the other returns,
	// the other returns are still checked for inner illegal consumptions
	r0 = takesNonnilRetsNilable(nil) //want "passed"
	e = &myErr{}
	return
}

// nilable(x, r1)
func retsNonnilNilableWithErr3(x *int, y *int) (r0 *int, r1 *int, e error) { //want "named return `r0`"
	// this error case indicates that if we return nil as our error and as a
	// non-nilable result, that result will be interpreted as an error
	return
}

// nilable(x, r1)
func retsNonnilNilableWithErr4(x *int, y *int) (r0 *int, r1 *int, e error) { //want "named return `r0`"
	i := 0
	switch 0 {
	case 7:
		// this is the same error case as above, but involving flow from a param
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

// nilable(x, r1)
func retsNonnilNilableWithErr5(x *int, y *int) (r0 *int, r1 *int, e error) { //want "named return `r0`"
	// this illustrates that an unassigned local error variable is interpreted as nil based on its zero value
	var e2 error
	e = e2
	return
}

// nilable(x, r1)
func retsNonnilNilableWithErr6(x *int, y *int) (r0 *int, r1 *int, e error) { //want "named return `r0`"
	// this is similar to the above case - but makes sure that computations in non-error results
	// are not ignored
	r0 = takesNonnilRetsNilable(nil) //want "passed"
	return
}

// nilable(x, r1)
func retsNonnilNilableWithErr7(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this illustrates that the checking for nilable results really is flow sensitive
	// here, we determine that `e` is non-nil making it a valid error that suppresses consumption
	// of the other returns
	if e != nil {
		return
	}
	return new(int), new(int), nil
}

// nilable(x, r1)
func retsNonnilNilableWithErr8(x *int, y *int) (r0 *int, r1 *int, e error) {
	// this is similar to the above case - but makes sure that computations in non-error results
	// are not ignored
	if e != nil {
		r0 = takesNonnilRetsNilable(nil) //want "passed"
		return
	}
	return new(int), new(int), nil
}

// nilable(x, r1)
func retsNonnilNilableWithErr9(x *int, y *int, cond bool) (r0 *int, r1 *int, e error) { //want "named return" "named return" "named return" "named return"
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
						return // (error is reported for `r0` at function declaration)
					}
				}
				if dummy { // here - two different flows result in a nilable (L187) or non-nil (L175) value for e
					return // (error is reported for `r0` at function declaration)
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
				// here - two different flows result in a nilable (L187) or non-nil (L175, L200) value for e
				return // (error is reported for `r0` at function declaration)
			}
		}
	}
	// here - two different flows result in a nilable (L102, L187) or non-nil (L200) value for e
	return // (error is reported for `r0` at function declaration)
}
