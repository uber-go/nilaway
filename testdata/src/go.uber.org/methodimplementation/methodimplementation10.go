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
This is a test for checking N-to-1 assignment form (e.g., i1, i2 = foo(), where foo return s1, s2)
*/

package methodimplementation

type i1 interface {
	foo() (x *int)
}

type i2 interface {
	foo() (x *int)
}

type i3 interface {
	foo() (x *int)
}

type i4 interface {
	foo() (x *int) //want "returned as result"
}

type i5 interface {
	foo() (x *int) //want "returned as result"
}

type i6 interface {
	foo() (x *int)
}

type i7 interface {
	foo() (x *int) //want "returned as result"
}

type i8 interface {
	foo() (x *int) //want "returned as result"
}

type s1 struct{}

func (*s1) foo() (x *int) {
	i := 0
	return &i
}

type s2 struct{}

// nilable(x)
func (*s2) foo() (x *int) { return nil }

func rets11() (*s1, *s1) {
	return &s1{}, &s1{}
}

func rets12() (*s1, *s2) {
	return &s1{}, &s2{}
}

func rets21() (*s2, *s1) {
	return &s2{}, &s1{}
}

func rets22() (*s2, *s2) {
	return &s2{}, &s2{}
}

func mainbody() {
	var x1 i1
	var x2 i2
	var x3 i3
	var x4 i4
	var x5 i5
	var x6 i6
	var x7 i7
	var x8 i8

	x1 = &s1{}

	x1, x2 = rets11()
	x3, x4 = rets12()
	x5, x6 = rets21()
	x7, x8 = rets22()
	func(...any) {}(x1, x2, x3, x4, x5, x6, x7, x8)
}
