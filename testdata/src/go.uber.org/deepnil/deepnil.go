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

nilaway no inference>
*/
package deepnil

// // nilable(A[])
// type A []*int
// type B []*int
//
// // nonnil(a, b)
// func takesTwoTypedArrs(a A, b B) *int {
// 	i := 0
// 	a[1] = nil
// 	b[1] = nil //want "assigned"
//
// 	a[2] = &i
// 	b[2] = &i
//
// 	switch 0 {
// 	case 1:
// 		return a[0] //want "returned"
// 	case 2:
// 		return a[1] //want "returned"
// 	case 3:
// 		return a[2]
// 	case 4:
// 		return b[0]
// 	case 5:
// 		return b[1] //want "returned"
// 	case 6:
// 		return b[2]
// 	}
// 	return &i
// }
//
// // nonnil(a, b)
// func deferredArrPass(a A, b B) *int {
// 	a2 := a[0]
// 	b2 := b[0]
// 	if true {
// 		return a2 //want "returned"
// 	} else {
// 		return b2
// 	}
// }
//
// func rangeTest(a A, b B) *int {
// 	if true {
// 		for _, a2 := range a {
// 			return a2 //want "returned"
// 		}
// 	} else {
// 		for _, b2 := range b {
// 			return b2
// 		}
// 	}
// 	return nil //want "returned"
// }
//
// func retsNonnilNonnil() (*int, *int) {
// 	i := 0
// 	return &i, &i
// }
//
// // nilable(result 0)
// func retsNilableNonnil() (*int, *int) {
// 	i := 0
// 	return nil, &i
// }
//
// // nilable(result 1)
// func retsNonnilNilable() (*int, *int) {
// 	i := 0
// 	return &i, nil
// }
//
// // nilable(result 0, result 1)
// func retsNilableNilable() (*int, *int) {
// 	return nil, nil
// }
//
// // this function tests the interplay between nilable returns and many-to-one assignment
// // nonnil(a, b)
// func testsManyToOneDeep(a A, b B) {
// 	switch "casestotest" {
// 	case "neither nilable":
// 		a[0], a[1] = retsNonnilNonnil()
// 		a[2], b[3] = retsNonnilNonnil()
// 		b[4], a[5] = retsNonnilNonnil()
// 		b[6], b[7] = retsNonnilNonnil()
// 	case "first nilable":
// 		a[0], a[1] = retsNilableNonnil()
// 		a[2], b[3] = retsNilableNonnil()
// 		b[4], a[5] = retsNilableNonnil() //want "assigned"
// 		b[6], b[7] = retsNilableNonnil() //want "assigned"
// 	case "second nilable":
// 		a[10], a[11] = retsNonnilNilable()
// 		a[12], b[13] = retsNonnilNilable() //want "assigned"
// 		b[14], a[15] = retsNonnilNilable()
// 		b[16], b[17] = retsNonnilNilable() //want "assigned"
// 	case "both nilable":
// 		a[0], a[1] = retsNilableNilable()
// 		a[2], b[3] = retsNilableNilable() //want "assigned"
// 		b[4], a[5] = retsNilableNilable() //want "assigned"
// 		b[6], b[7] = retsNilableNilable() //want "assigned" "assigned"
// 	}
// }
//
// // same as takesTwoTypedArrs but uses parameter annotations instead of annotated types
// // nilable(a[]) nonnil(a, b)
// func takesTwoAnnotatedArrs(a []*int, b []*int) *int {
// 	i := 0
// 	a[1] = nil
// 	b[1] = nil //want "assigned"
//
// 	a[2] = &i
// 	b[2] = &i
//
// 	switch 0 {
// 	case 1:
// 		return a[0] //want "returned"
// 	case 2:
// 		return a[1] //want "returned"
// 	case 3:
// 		return a[2]
// 	case 4:
// 		return b[0]
// 	case 5:
// 		return b[1] //want "returned"
// 	case 6:
// 		return b[2]
// 	}
// 	return &i
// }
//
// // nilable(result 0[])
// // nonnil(result 0)
// func retsNilableArr(i int) []*int {
// 	return []*int{&i}
// }
//
// // nonnil(result 0)
// func retsNonNilArr(i int) []*int {
// 	return []*int{&i}
// }
//
// func retsNonNilArrBad(i int) (a []*int) {
// 	return []*int{nil, nil, nil} // TODO:  this should fail
// }
//
// func takesNonNilIntStar(i *int) {}
//
// var i = 0
//
// func testsArrRets() *int {
// 	switch 0 {
// 	case 1:
// 		return retsNilableArr(0)[0] //want "returned"
// 	case 2:
// 		return retsNonNilArr(0)[0]
// 	case 3:
// 		return retsNilableArr(i)[0] //want "returned"
// 	case 4:
// 		return retsNonNilArr(i)[0]
// 	case 5:
// 		return retsNilableArr(0)[i] //want "returned"
// 	case 6:
// 		return retsNonNilArr(0)[i]
// 	case 7:
// 		return retsNilableArr(i)[i] //want "returned"
// 	case 8:
// 		return retsNonNilArr(i)[i]
// 	case 9:
// 		a := retsNilableArr(0)
// 		return a[0] //want "returned"
// 	case 10:
// 		a := retsNonNilArr(0)
// 		return a[0]
// 	case 11:
// 		a := retsNilableArr(i)
// 		return a[0] //want "returned"
// 	case 12:
// 		a := retsNonNilArr(i)
// 		return a[0]
// 	case 13:
// 		a := retsNilableArr(0)
// 		// unfortunately, the type system here gives `a` the type []*int, which is not deeply nilable
// 		return a[i] // TODO:  want "returned"
// 	case 14:
// 		a := retsNonNilArr(0)
// 		return a[i]
// 	case 15:
// 		a := retsNilableArr(i)
// 		// same flow error as case 13 above
// 		return a[i] // TODO:  want "returned"
// 	case 16:
// 		a := retsNonNilArr(i)
// 		return a[i]
// 	case 17:
// 		for _, a := range retsNilableArr(0) {
// 			return a //want "returned"
// 		}
// 		return nil //want "returned"
// 	case 18:
// 		for _, a := range retsNonNilArr(0) {
// 			return a
// 		}
// 		return nil //want "returned"
// 	case 19:
// 		for _, a := range retsNilableArr(0) {
// 			takesNonNilIntStar(a) //want "passed"
// 		}
// 		return nil //want "returned"
// 	default:
// 		for _, a := range retsNonNilArr(0) {
// 			takesNonNilIntStar(a)
// 		}
// 		return nil //want "returned"
// 	}
// }
//
// // nilable(f[])
// // nonnil(f, g)
// type S struct {
// 	f []*S
// 	g []*S
// }
//
// // same as takesTwoTypedArrs but uses annotated fields of a struct
// func takesStruct(s *S) *S {
// 	s.f[1] = nil
// 	s.g[1] = nil //want "assigned"
//
// 	s.f[2] = &S{}
// 	s.g[2] = &S{}
//
// 	switch 0 {
// 	case 1:
// 		return s.f[0] //want "returned"
// 	case 2:
// 		return s.f[1] //want "returned"
// 	case 3:
// 		return s.f[2]
// 	case 4:
// 		return s.g[0]
// 	case 5:
// 		return s.g[1] //want "returned"
// 	case 6:
// 		return s.g[2]
// 	}
// 	return &S{}
// }
//
// func testDeepNilStruct(s *S) *S {
// 	switch 0 {
// 	case 1:
// 		return s.f[0] //want "returned"
// 	case 2:
// 		return s.g[0]
// 	case 3:
// 		s2 := s.f[0]
// 		return s2 //want "returned"
// 	case 4:
// 		s2 := s.g[0]
// 		return s2
// 	case 5:
// 		return s.g[0].f[0] //want "returned"
// 	case 6:
// 		return s.g[0].g[0]
// 	case 7:
// 		return s.f[0].f[0] //want "deep read from field `f`" "returned"
// 	default:
// 		return s.f[0].g[0] //want "deep read from field `f`"
// 	}
// }
//
// // nilable(X[])
// type X []*int
// type Y []*int
//
// type XY []Y
//
// type XX []X
//
// // nonnil(YY[])
// type YY []Y
//
// // nonnil(YX[])
// type YX []X
//
// // nonnil(xy, xx, yy, yx)
// func testSliceTypes(xy XY, xx XX, yy YY, yx YX) *int {
// 	switch 0 {
// 	case 1:
// 		return xy[0][0] //want "sliced into"
// 	case 2:
// 		return xx[0][0] //want "returned" "sliced into"
// 	case 3:
// 		return yy[0][0]
// 	case 4:
// 		return yx[0][0] //want "returned"
// 	case 5:
// 		return xy[i][i] //want "sliced into"
// 	case 6:
// 		return xx[i][i] //want "returned" "sliced into"
// 	case 7:
// 		return yy[i][i]
// 	case 8:
// 		return yx[i][i] //want "returned"
// 	case 9:
// 		return xy[i][0] //want "sliced into"
// 	case 10:
// 		return xx[i][0] //want "returned" "sliced into"
// 	case 11:
// 		return yy[i][0]
// 	case 12:
// 		return yx[i][0] //want "returned"
// 	case 13:
// 		return xy[i][0] //want "sliced into"
// 	case 14:
// 		return xx[i][0] //want "returned" "sliced into"
// 	case 15:
// 		return yy[i][0]
// 	case 16:
// 		return yx[i][0] //want "returned"
// 	}
// 	return nil //want "returned"
// }

func testDeepAssignNil(i int) {
	m := make(map[int]*string)
	m[i] = nil
	if v, ok := m[i]; ok {
		_ = *v
	}
}
