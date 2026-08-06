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
This package aims to test the "deep" nilability tracking mechanism - which records whether
members of slices, maps, and pointers can be nil in addition to the objects themselves being nil.

Under inference, deep nilability is not annotated but inferred from usage: writing nil (or a
nilable value) deeply into a slice makes its deep site nilable, and dereferencing (or indexing)
a value read deeply from a slice forces its deep site to be nonnil. A conflict between the two is
reported at the dereference site. Note that the inference engine reports a single error per
inference site (with one witness nil flow), so each erroneous flow below is paired with exactly
one dereference of a deeply read value; purely intra-procedural flows of nil literals are tracked
flow-sensitively and get their own error at each dereference.

notably, a few are left as TODOs, this is because certain desirable nilability properties are
flow-sensitive beyond top-level nilability tracking, and we do NOT actually track deeper nilability
between expressions - we just read it where appropriate. This will hopefully be remedied in the future
and is tracked in Jira as
*/
package deepnil

// A and B are named slice types - deep nilability for them is tracked at the type level, so the
// nil written into them below conflicts with the dereferences of their deeply read elements.
type A []*int
type B []*int

func takesTwoTypedArrs(a A, b B) *int {
	i := 0
	a[1] = nil
	b[1] = nil

	a[2] = &i
	b[2] = &i

	switch 0 {
	case 1:
		_ = *a[0] //want "assigned into a slice of deeply nonnil type `A`"
		return a[0]
	case 2:
		_ = *a[1] //want "literal `nil` dereferenced"
		return a[1]
	case 3:
		_ = *a[2]
		return a[2]
	case 4:
		_ = *b[0] //want "assigned into a slice of deeply nonnil type `B`"
		return b[0]
	case 5:
		_ = *b[1] //want "literal `nil` dereferenced"
		return b[1]
	case 6:
		_ = *b[2]
		return b[2]
	}
	return &i
}

// deferredArrPass checks deep reads stored into local variables before being dereferenced.
// Parameter `a` has nil written deeply into it, so the dereference of its deeply read element
// errors, while parameter `b` remains deeply nonnil and its element can be safely dereferenced.
func deferredArrPass(a []*int, b []*int) *int {
	a[1] = nil
	a2 := a[0]
	b2 := b[0]
	if true {
		_ = *a2 //want "deep read from parameter `a` dereferenced"
		return a2
	} else {
		_ = *b2
		return b2
	}
}

// rangeTest checks deep reads via ranging over the slices. The dereference of the result at the
// call site below additionally checks that nilable values (including literal nil) returned from
// the function are caught.
func rangeTest(a []*int, b []*int) *int {
	a[1] = nil
	if true {
		for _, a2 := range a {
			_ = *a2 //want "deep read from parameter `a` dereferenced"
			return a2
		}
	} else {
		for _, b2 := range b {
			_ = *b2
			return b2
		}
	}
	return nil
}

func callsRangeTest(a []*int, b []*int) {
	_ = *rangeTest(a, b) //want "returned from `rangeTest"
}

func retsNonnilNonnil() (*int, *int) {
	i := 0
	return &i, &i
}

func retsNilableNonnil() (*int, *int) {
	i := 0
	return nil, &i
}

func retsNonnilNilable() (*int, *int) {
	i := 0
	return &i, nil
}

func retsNilableNilable() (*int, *int) {
	return nil, nil
}

// this function tests the interplay between nilable returns and many-to-one assignment.
// Parameter `a` is never deeply consumed, so the nilable results flowing into it are safe.
// Parameters `b1`, `b2`, `b3` each have a deeply read element dereferenced, so the nilable
// results flowing into them are errors (one reported per parameter, at the dereference).
func testsManyToOneDeep(a, b1, b2, b3 []*int) {
	switch "casestotest" {
	case "neither nilable":
		a[0], a[1] = retsNonnilNonnil()
		a[2], b1[3] = retsNonnilNonnil()
		b2[4], a[5] = retsNonnilNonnil()
		b3[6], b3[7] = retsNonnilNonnil()
	case "first nilable":
		a[0], a[1] = retsNilableNonnil()
		a[2], b1[3] = retsNilableNonnil()
		b1[4], a[5] = retsNilableNonnil()
		b1[6], b1[7] = retsNilableNonnil()
		_ = *b1[0] //want "assigned deeply into parameter arg `b1`"
	case "second nilable":
		a[10], a[11] = retsNonnilNilable()
		a[12], b2[13] = retsNonnilNilable()
		b2[14], a[15] = retsNonnilNilable()
		b2[16], b2[17] = retsNonnilNilable()
		_ = *b2[0] //want "assigned deeply into parameter arg `b2`"
	case "both nilable":
		a[0], a[1] = retsNilableNilable()
		a[2], b3[3] = retsNilableNilable()
		b3[4], a[5] = retsNilableNilable()
		b3[6], b3[7] = retsNilableNilable()
		_ = *b3[0] //want "assigned deeply into parameter arg `b3`"
	}
}

// same as takesTwoTypedArrs but uses plain (unnamed) slice types, for which deep nilability is
// tracked per parameter instead of per type.
func takesTwoPlainArrs(a []*int, b []*int) *int {
	i := 0
	a[1] = nil
	b[1] = nil

	a[2] = &i
	b[2] = &i

	switch 0 {
	case 1:
		_ = *a[0] //want "assigned deeply into parameter arg `a`"
		return a[0]
	case 2:
		_ = *a[1] //want "literal `nil` dereferenced"
		return a[1]
	case 3:
		_ = *a[2]
		return a[2]
	case 4:
		_ = *b[0] //want "assigned deeply into parameter arg `b`"
		return b[0]
	case 5:
		_ = *b[1] //want "literal `nil` dereferenced"
		return b[1]
	case 6:
		_ = *b[2]
		return b[2]
	}
	return &i
}

// retsNilableArr returns a slice whose deep site is inferred nilable from the nil written into it.
func retsNilableArr(i int) []*int {
	s := make([]*int, 2)
	s[0] = &i
	s[1] = nil
	return s
}

// retsNonNilArr returns a slice with no nil written into it, so its deep site is inferred nonnil.
func retsNonNilArr(i int) []*int {
	s := make([]*int, 1)
	s[0] = &i
	return s
}

func retsNonNilArrBad(i int) (a []*int) {
	return []*int{nil, nil, nil} // TODO: this should fail (deep contents of composite literals are not tracked)
}

func takesNonNilIntStar(i *int) {
	_ = *i //want "passed"
}

var i = 0

func arrRetDirectConst() *int {
	return retsNilableArr(0)[0]
}

func arrRetDirectVariable(i int) *int {
	return retsNilableArr(i)[i]
}

func arrRetLocalConst() *int {
	a := retsNilableArr(0)
	return a[0]
}

func arrRetLocalVariable(i int) *int {
	a := retsNilableArr(i)
	return a[i]
}

func arrRetRange() *int {
	for _, a := range retsNilableArr(0) {
		return a
	}
	return new(int)
}

func testArrRets(i int) {
	_ = *arrRetDirectConst()     //want "returned from `arrRetDirectConst"
	_ = *arrRetDirectVariable(i) //want "returned from `arrRetDirectVariable"
	_ = *arrRetLocalConst()      //want "returned from `arrRetLocalConst"
	_ = *arrRetLocalVariable(i)  //want "returned from `arrRetLocalVariable"
	_ = *arrRetRange()           //want "returned from `arrRetRange"

	_ = *retsNonNilArr(0)[0]
	_ = *retsNonNilArr(i)[i]

	a := retsNonNilArr(0)
	_ = *a[0]
	a = retsNonNilArr(i)
	_ = *a[i]

	for _, a := range retsNonNilArr(0) {
		_ = *a
	}

	for _, a := range retsNilableArr(i) {
		// The deeply read nilable value is passed to a function that dereferences its parameter.
		// The error is reported at the dereference inside takesNonNilIntStar.
		takesNonNilIntStar(a)
	}
	for _, a := range retsNonNilArr(i) {
		takesNonNilIntStar(a)
	}
}

// S has two slice fields - deep nilability for fields is tracked at the field level (shared
// across the whole package). Both fields get nil written deeply into them in takesStruct below,
// which conflicts with the field accesses on their deeply read elements.
type S struct {
	f []*S
	g []*S
}

// same as takesTwoTypedArrs but uses fields of a struct
func takesStruct(s *S) *S {
	s.f[1] = nil
	s.g[1] = nil

	s.f[2] = &S{}
	s.g[2] = &S{}

	switch 0 {
	case 1:
		v := s.f[0]
		_ = v.f //want "assigned deeply into field `f`"
		return v
	case 2:
		v := s.f[1]
		_ = v.f //want "literal `nil` accessed field"
		return v
	case 3:
		_ = s.f[2].f
		return s.f[2]
	case 4:
		v := s.g[0]
		_ = v.f //want "assigned deeply into field `g`"
		return v
	case 5:
		v := s.g[1]
		_ = v.f //want "literal `nil` accessed field"
		return v
	case 6:
		_ = s.g[2].f
		return s.g[2]
	}
	return &S{}
}

// S2 mirrors S, but only field f gets nil written deeply into it, keeping field g deeply nonnil.
// (A separate struct type is used since field deep sites are shared package-wide, and each site
// reports a single error.)
type S2 struct {
	f []*S2
	g []*S2
}

func testDeepNilStruct(s *S2) *S2 {
	s.f[1] = nil
	switch 0 {
	case 1:
		return s.g[0]
	case 2:
		s2 := s.f[1]
		_ = s2.f //want "literal `nil` accessed field"
		return s2
	case 3:
		s2 := s.g[0]
		return s2
	case 4:
		return s.g[0].g[0]
	case 5:
		return s.g[0].f[0]
	default:
		return s.f[0].f[0] //want "deep read from field `f`"
	}
}

// X and Y are the element types of the doubly nested slice types below - X has nil written
// deeply into it (in testSliceTypes), while Y remains deeply nonnil.
type X []*int
type Y []*int

type XY []Y

type XX []X

type YY []Y

type YX []X

// testSliceTypes checks the shallow nilability of elements read from slices of slices: XY and XX
// get nil (inner slice) values written into them, so indexing ("slicing into") their elements
// errors, while elements of YY and YX can be safely indexed.
func testSliceTypes(xy XY, xx XX, yy YY, yx YX) *int {
	xy[1] = nil
	xx[1] = nil
	switch 0 {
	case 1:
		return xy[0][0] //want "sliced into"
	case 2:
		return xx[0][0] //want "sliced into"
	case 3:
		return yy[0][0]
	case 4:
		return yx[0][0]
	case 5:
		return yy[i][i]
	case 6:
		return yx[i][i]
	case 7:
		return yy[i][0]
	case 8:
		return yx[i][0]
	case 9:
		// checks the deep nilability of the inner element types themselves: nil is written
		// deeply into the X-typed element, so dereferencing its deeply read element errors.
		x := yx[0]
		x[1] = nil
		_ = *x[0] //want "assigned into a slice of deeply nonnil type `X`"
		return x[2]
	case 10:
		y := yy[0]
		_ = *y[0]
		return y[0]
	}
	return nil
}
