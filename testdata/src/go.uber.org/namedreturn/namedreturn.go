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

func foo1() (i *int) {
	return //want "named return value `i` in position 0"
}

// nilable(i)
func foo2() (i *int) {
	return
}

func foo3() (i, j *int) {
	x := 1
	i = &x
	return //want "named return value `j` in position 1"
}

func foo4(x int, y string) (k bool, i *int, s *string, a []int) {
	switch x {
	case 0:
		return k, i, s, a //want "nilable value returned" "nilable value returned"
	case 1:
		i = &x
		return //want "named return value `s` in position 2"
	case 2:
		s = &y
		return //want "named return value `i` in position 1"
	case 3:
		i = &x
		s = &y
		return
	case 4:
		a = make([]int, 5)
		return //want "named return value `i` in position 1" "named return value `s` in position 2"
	}
	return //want "named return value `i` in position 1" "named return value `s` in position 2"
}

func foo5(n int) (i *int) {
	if n > 0 {
		x := 1
		i := &x
		return i
	}
	return //want "named return value `i` in position 0"
}

func foo6() (i, j *int) {
	x := 1
	i, k := &x, 0
	func(...any) {}(i, k)
	return //want "named return value `j` in position 1"
}

func foo7() (i, j *int) {
	x := 1
	if true {
		i := &x
		func(any) {}(i)
	}
	return //want "named return value `i` in position 0" "named return value `j` in position 1"
}

func foo8(x string) (_ *int) {
	return //want "named return value `_` in position 0"
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

func foo11() (x *int, _ error) {
	return //want "named return value `x` in position 0"
}

var dummy bool

// nilable(result 0)
func takesNonnilRetsNilable(x *int) *int {
	return x
}

// nilable(x, r1)
func retsNonnilNilableWithErr(x *int, y *int) (r0 *int, r1 *int, e error) {
	i := 0
	switch 0 {
	case 1:
		// this safe case indicates that if we return non-nil as our error,
		// we can return nilable values in non-nil results without error
		e = &myErr{}
		return
	case 2:
		// this is the same safe case as above, but involving flow from a nilableparam
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
	case 5:
		// this checks that even if a non-nil error aborts the consumption of the other returns,
		// the other returns are still checked for inner illegal consumptions
		r0 = takesNonnilRetsNilable(nil) //want "nilable value passed"
		e = &myErr{}
		return
	case 6:
		// this error case indicates that if we return nil as our error and as a
		// non-nilable result, that result will be interpreted as an error
		return //want "nilable value returned"
	case 7:
		// this is the same error case as above, but involving flow from a param
		r0 = x
		return //want "nilable value returned"
	case 8:
		// this is safe
		r0 = &i
		return
	case 9:
		// this is safe
		r0 = y
		return
	case 10:
		// this illustrates that an unassigned local error variable is interpreted as nil based on its zero value
		var e2 error
		e = e2
		return //want "named return value `r0` in position 0"
	case 11:
		return //want "named return value `r0` in position 0"
	case 12:
		// this is similar to the above case - but makes sure that computations in non-error results
		// are not ignored
		r0 = takesNonnilRetsNilable(nil) //want "nilable value passed"
		return                           //want "named return value `r0` in position 0"
	case 13:
		// this illustrates that the checking for nilable results really is flow sensitive
		// here, we determine that `e` is non-nil making it a valid error that suppresses consumption
		// of the other returns
		if e != nil {
			return
		}
	case 14:
		// this is similar to the above case - but makes sure that computations in non-error results
		// are not ignored
		if e != nil {
			r0 = takesNonnilRetsNilable(nil) //want "nilable value passed"
			return
		}
	case 15:
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
						return //want "named return value `r0` in position 0"
					}
				}
				if dummy { // here - two different flows result in a nilable (L187) or non-nil (L175) value for e
					return //want "named return value `r0` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths"
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
				return //want "named return value `r0` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths"
			}
		}
	}
	// here - two different flows result in a nilable (L102, L187) or non-nil (L200) value for e
	return //want "named return value `r0` in position 0 when the error return in position 2 is not guaranteed to be non-nil through all paths"
}
