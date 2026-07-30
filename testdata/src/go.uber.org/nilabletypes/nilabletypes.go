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
This test aims to check some common cases where even though a variable is never
assigned to, it is still known to be non-nil from its type. We make sure that in
these cases no diagnostics are emitted
*/
package nilabletypes

type A struct {
	f int
}

type A2 A

type B interface {
	isB()
}

type myInt int

func nilableTypesTest() interface{} {
	var aptr *A
	var a A
	var a2ptr *A2
	var a2 A2
	var bptr *B
	var b B
	var iptr *int
	var i int
	var f func()
	var mi myInt
	var miptr *myInt
	var slc1 []int
	var slc2 []*int
	var mp1 map[int]int
	var mp2 map[*int]*int

	switch 0 {
	case 1:
		_ = aptr.f //want "accessed field"
		return aptr
	case 2:
		return a
	case 3:
		_ = a2ptr.f //want "accessed field"
		return a2ptr
	case 4:
		return a2
	case 5:
		_ = *bptr //want "dereferenced"
		return bptr
	case 6:
		b.isB() //want "called"
		return b
	case 7:
		_ = *iptr //want "dereferenced"
		return iptr
	case 8:
		return i
	case 9:
		return f
	case 10:
		return mi
	case 11:
		_ = *miptr //want "dereferenced"
		return miptr
	case 12:
		return &A{}
	case 13:
		return A{}
	case 14:
		return &A2{}
	case 15:
		return A2{}
	case 16:
		var result *A
		_ = result.f //want "accessed field"
		return result
	case 17:
		return func(i int) int { return i }
	case 18:
		return 0
	case 19:
		_ = slc1[0] //want "sliced into"
		return slc1
	case 20:
		_ = slc2[0] //want "sliced into"
		return slc2
	case 21:
		mp1[0] = 0 //want "written to at an index"
		return mp1
	case 22:
		mp2[nil] = nil //want "written to at an index"
		return mp2
	case 23:
		var x A
		y := &x
		return y
	case 24:
		var x A
		y := &x
		return y.f
	case 25:
		var x A
		return x
	case 26:
		var x A
		return x.f
	case 27:
		var x A
		y := &x
		return *y
	case 28:
		var x A
		return *(&x)
	case 29:
		var x A
		return (&(*(&(*(&x)))))
	case 30:
		var x *A
		y := *x //want "unassigned variable `x` dereferenced"
		return &y
	case 31:
		var x *A
		_ = x.f //want "unassigned variable `x` accessed field"
		return &x
	case 32:
		var x *A
		return x.f //want "unassigned variable `x` accessed field `f`"
	case 33:
		var x *A
		return &(*x) //want "unassigned variable `x` dereferenced"
	case 34:
		var x *A
		return (*(&(*(&(*x))))) //want "unassigned variable `x` dereferenced"
	default:
		return nilableTypesTest()
	}
}
