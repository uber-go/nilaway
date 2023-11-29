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

<nilaway no inference>
*/
package nilabletypes

type A struct {
	f int
}

type A2 A

type B interface{}

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
		return aptr //want "returned"
	case 2:
		return a
	case 3:
		return a2ptr //want "returned"
	case 4:
		return a2
	case 5:
		return bptr //want "returned"
	case 6:
		return b //want "returned"
	case 7:
		return iptr //want "returned"
	case 8:
		return i
	case 9:
		return f
	case 10:
		return mi
	case 11:
		return miptr //want "returned"
	case 12:
		return &A{}
	case 13:
		return A{}
	case 14:
		return &A2{}
	case 15:
		return A2{}
	case 16:
		return nil //want "returned"
	case 17:
		return func(i int) int { return i }
	case 18:
		return 0
	case 19:
		return slc1 //want "returned"
	case 20:
		return slc2 //want "returned"
	case 21:
		return mp1 //want "returned"
	case 22:
		return mp2 //want "returned"
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
		return &x //want "unassigned variable `x` returned"
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
