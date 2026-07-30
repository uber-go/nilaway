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
This package aims to test the "deep" nilability annotation mechanism - which records whether
members of slices, maps, and pointers can be nil in addition to the objects themselves being nil

notably, a few are left as TODOs, this is because certain desirable nilability properties are
flow-sensitive beyond top-level nilability tracking, and we do NOT actually track deeper nilability
between expressions - we just read it where appropriate. This will hopefully be remedied in the future
and is tracked in Jira as
*/
package deepnil

// nilable(A[])
type A []*int
type B []*int

// nonnil(a, b)
func takesTwoTypedArrs(a A, b B) *int {
	i := 0
	a[1] = nil
	b[1] = nil
	print(*b[1]) //want "dereferenced"

	a[2] = &i
	b[2] = &i

	switch 0 {
	case 1:
		print(*a[0]) //want "dereferenced"
		return a[0]
	case 2:
		print(*a[1]) //want "dereferenced"
		return a[1]
	case 3:
		return a[2]
	case 4:
		return b[0]
	case 5:
		print(*b[1]) //want "dereferenced"
		return b[1]
	case 6:
		return b[2]
	}
	return &i
}

// nonnil(a, b)
func deferredArrPass(a A, b B) *int {
	a2 := a[0]
	b2 := b[0]
	if true {
		print(*a2) //want "dereferenced"
		return a2
	} else {
		return b2
	}
}

func rangeTest(a A, b B) *int {
	if true {
		for _, a2 := range a {
			print(*a2) //want "dereferenced"
			return a2
		}
	} else {
		for _, b2 := range b {
			return b2
		}
	}
	var nilResult *int
	print(*nilResult) //want "dereferenced"
	return nil
}

func retsNonnilNonnil() (*int, *int) {
	i := 0
	return &i, &i
}

// nilable(result 0)
func retsNilableNonnil() (*int, *int) {
	i := 0
	return nil, &i
}

// nilable(result 1)
func retsNonnilNilable() (*int, *int) {
	i := 0
	return &i, nil
}

// nilable(result 0, result 1)
func retsNilableNilable() (*int, *int) {
	return nil, nil
}

// this function tests the interplay between nilable returns and many-to-one assignment
// nonnil(a, b)
func testsManyToOneDeep(a A, b B) {
	switch "casestotest" {
	case "neither nilable":
		a[0], a[1] = retsNonnilNonnil()
		a[2], b[3] = retsNonnilNonnil()
		b[4], a[5] = retsNonnilNonnil()
		b[6], b[7] = retsNonnilNonnil()
	case "first nilable":
		a[0], a[1] = retsNilableNonnil()
		a[2], b[3] = retsNilableNonnil()
		b[4], a[5] = retsNilableNonnil()
		print(*b[4]) //want "dereferenced"
		b[6], b[7] = retsNilableNonnil()
		print(*b[6]) //want "dereferenced"
	case "second nilable":
		a[10], a[11] = retsNonnilNilable()
		a[12], b[13] = retsNonnilNilable()
		print(*b[13]) //want "dereferenced"
		b[14], a[15] = retsNonnilNilable()
		b[16], b[17] = retsNonnilNilable()
		print(*b[17]) //want "dereferenced"
	case "both nilable":
		a[0], a[1] = retsNilableNilable()
		a[2], b[3] = retsNilableNilable()
		print(*b[3]) //want "dereferenced"
		b[4], a[5] = retsNilableNilable()
		print(*b[4]) //want "dereferenced"
		b[6], b[7] = retsNilableNilable()
		print(*b[6]) //want "dereferenced"
		print(*b[7]) //want "dereferenced"
	}
}

// same as takesTwoTypedArrs but uses parameter annotations instead of annotated types
// nilable(a[]) nonnil(a, b)
func takesTwoAnnotatedArrs(a []*int, b []*int) *int {
	i := 0
	a[1] = nil
	b[1] = nil
	print(*b[1]) //want "dereferenced"

	a[2] = &i
	b[2] = &i

	switch 0 {
	case 1:
		print(*a[0]) //want "dereferenced"
		return a[0]
	case 2:
		print(*a[1]) //want "dereferenced"
		return a[1]
	case 3:
		return a[2]
	case 4:
		return b[0]
	case 5:
		print(*b[1]) //want "dereferenced"
		return b[1]
	case 6:
		return b[2]
	}
	return &i
}

// nilable(result 0[])
// nonnil(result 0)
func retsNilableArr(i int) []*int {
	return []*int{&i}
}

// nonnil(result 0)
func retsNonNilArr(i int) []*int {
	return []*int{&i}
}

func retsNonNilArrBad(i int) (a []*int) {
	return []*int{nil, nil, nil} // TODO:  this should fail
}

func takesNonNilIntStar(i *int) {}

var i = 0

func testsArrRets() *int {
	switch 0 {
	case 1:
		print(*retsNilableArr(0)[0]) //want "dereferenced"
		return retsNilableArr(0)[0]
	case 2:
		return retsNonNilArr(0)[0]
	case 3:
		print(*retsNilableArr(i)[0]) //want "dereferenced"
		return retsNilableArr(i)[0]
	case 4:
		return retsNonNilArr(i)[0]
	case 5:
		print(*retsNilableArr(0)[i]) //want "dereferenced"
		return retsNilableArr(0)[i]
	case 6:
		return retsNonNilArr(0)[i]
	case 7:
		print(*retsNilableArr(i)[i]) //want "dereferenced"
		return retsNilableArr(i)[i]
	case 8:
		return retsNonNilArr(i)[i]
	case 9:
		a := retsNilableArr(0)
		print(*a[0]) //want "dereferenced"
		return a[0]
	case 10:
		a := retsNonNilArr(0)
		return a[0]
	case 11:
		a := retsNilableArr(i)
		print(*a[0]) //want "dereferenced"
		return a[0]
	case 12:
		a := retsNonNilArr(i)
		return a[0]
	case 13:
		a := retsNilableArr(0)
		print(*a[i]) //want "dereferenced"
		return a[i]
	case 14:
		a := retsNonNilArr(0)
		return a[i]
	case 15:
		a := retsNilableArr(i)
		print(*a[i]) //want "dereferenced"
		return a[i]
	case 16:
		a := retsNonNilArr(i)
		return a[i]
	case 17:
		for _, a := range retsNilableArr(0) {
			print(*a) //want "dereferenced"
			return a
		}
		var nilResult17 *int
		print(*nilResult17) //want "dereferenced"
		return nil
	case 18:
		for _, a := range retsNonNilArr(0) {
			return a
		}
		var nilResult18 *int
		print(*nilResult18) //want "dereferenced"
		return nil
	case 19:
		for _, a := range retsNilableArr(0) {
			print(*a) //want "dereferenced"
			takesNonNilIntStar(a)
		}
		var nilResult19 *int
		print(*nilResult19) //want "dereferenced"
		return nil
	default:
		for _, a := range retsNonNilArr(0) {
			takesNonNilIntStar(a)
		}
		var nilResultDefault *int
		print(*nilResultDefault) //want "dereferenced"
		return nil
	}
}

// nilable(f[])
// nonnil(f, g)
type S struct {
	f []*S
	g []*S
}

// same as takesTwoTypedArrs but uses annotated fields of a struct
func takesStruct(s *S) *S {
	s.f[1] = nil
	s.g[1] = nil
	_ = *s.g[1] //want "dereferenced"

	s.f[2] = &S{}
	s.g[2] = &S{}

	switch 0 {
	case 1:
		_ = *s.f[0] //want "dereferenced"
		return s.f[0]
	case 2:
		_ = *s.f[1] //want "dereferenced"
		return s.f[1]
	case 3:
		return s.f[2]
	case 4:
		return s.g[0]
	case 5:
		_ = *s.g[1] //want "dereferenced"
		return s.g[1]
	case 6:
		return s.g[2]
	}
	return &S{}
}

func testDeepNilStruct(s *S) *S {
	switch 0 {
	case 1:
		_ = *s.f[0] //want "dereferenced"
		return s.f[0]
	case 2:
		return s.g[0]
	case 3:
		s2 := s.f[0]
		_ = *s2 //want "dereferenced"
		return s2
	case 4:
		s2 := s.g[0]
		return s2
	case 5:
		g0 := s.g[0]
		if g0 == nil {
			return &S{}
		}
		result := g0.f[0]
		_ = *result //want "dereferenced"
		return result
	case 6:
		g0 := s.g[0]
		if g0 == nil {
			return &S{}
		}
		return g0.g[0]
	case 7:
		result := s.f[0].f[0] //want "deep read from field `f`"
		_ = *result           //want "dereferenced"
		return result
	default:
		return s.f[0].g[0] //want "deep read from field `f`"
	}
}

// nilable(X[])
type X []*int
type Y []*int

type XY []Y

type XX []X

// nonnil(YY[])
type YY []Y

// nonnil(YX[])
type YX []X

// nonnil(xy, xx, yy, yx)
func testSliceTypes(xy XY, xx XX, yy YY, yx YX) *int {
	switch 0 {
	case 1:
		xy[0] = nil
		result := xy[0][0] //want "sliced into"
		return result
	case 2:
		xx[0] = nil
		result := xx[0][0] //want "sliced into"
		print(*result)     //want "dereferenced"
		return result
	case 3:
		return yy[0][0]
	case 4:
		print(*yx[0][0]) //want "dereferenced"
		return yx[0][0]
	case 5:
		xy[i] = nil
		result := xy[i][i] //want "sliced into"
		return result
	case 6:
		xx[i] = nil
		result := xx[i][i] //want "sliced into"
		print(*result)     //want "dereferenced"
		return result
	case 7:
		return yy[i][i]
	case 8:
		print(*yx[i][i]) //want "dereferenced"
		return yx[i][i]
	case 9:
		xy[i] = nil
		result := xy[i][0] //want "sliced into"
		return result
	case 10:
		xx[i] = nil
		result := xx[i][0] //want "sliced into"
		print(*result)     //want "dereferenced"
		return result
	case 11:
		return yy[i][0]
	case 12:
		print(*yx[i][0]) //want "dereferenced"
		return yx[i][0]
	case 13:
		xy[i] = nil
		result := xy[i][0] //want "sliced into"
		return result
	case 14:
		xx[i] = nil
		result := xx[i][0] //want "sliced into"
		print(*result)     //want "dereferenced"
		return result
	case 15:
		return yy[i][0]
	case 16:
		print(*yx[i][0]) //want "dereferenced"
		return yx[i][0]
	}
	var nilResult *int
	print(*nilResult) //want "dereferenced"
	return nil
}
