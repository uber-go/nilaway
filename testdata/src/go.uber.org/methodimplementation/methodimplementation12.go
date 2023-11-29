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
This is a test for checking nilability-type variance. Covariance for return types and contravariance for parameter
types of methods implementing an interface.
interface method returns non-nil, implementing method returns nil --> covariance violation
interface method param nil, implementing method param non-nil --> contravariance violation

In particular, this test file checks 3 posible sites for struct implementing an interface:
1) Function signature states interface return, and the actual return is a struct.
2) Slice is declared to be of interface type, but struct is added to it
3) Map is declared to be of interface type, but struct is added to it.

<nilaway no inference>
*/
package methodimplementation

// 1) If the function signature states interface return, but the actual return is a struct.
// The test cases check if Nilaway treats the struct as it implements the corresponding interface.

type I121 interface {
	// nilable(x)
	foo(x *A121) (*A121, string) //want "returned as result"
}

type J121 interface {
	// nilable(x, result 0)
	bar(x *A121, y *B121) *string
}

type A121 struct {
	s string
}

type B121 struct {
	i int
}

// nilable(result 0)
func (A121) foo(x *A121) (*A121, string) { //want "passed as param"
	var b *A121
	return b, x.s
}

// nilable(x)
func (a A121) bar(x *A121, y *B121) *string {
	if x != nil {
		return &x.s
	}
	return &a.s
}

func (b B121) foo(x *A121) (*A121, string) {
	return x, x.s // this is safe because struct of type B is never used as the interface type I
}

// nilable(y, result 0)
func (b *B121) bar(x *A121, y *B121) *string { //want "passed as param"
	if b.i+y.i > 5 { //want "accessed field `i`"
		return nil
	}
	return &x.s
}

func dummy() *A121 {
	return &A121{}
}

func m121(x *A121, y *B121) (I121, J121) {
	// function signature states interface return, but the actual return is a struct
	return dummy(), y
}

// 2) If slice is declared to be of interface type, but struct is added to it. The test cases check if Nilaway
// treats the struct as it implements the corresponding interface.

type I122 interface {
	// nilable(x)
	foo(x *A122) (*A122, string) //want "returned as result"
}

type J122 interface {
	// nilable(x, result 0)
	bar(x *A122, y *B122) *string
}

type A122 struct {
	s string
}

type B122 struct {
	i int
}

// nilable(result 0)
func (A122) foo(x *A122) (*A122, string) { //want "passed as param"
	var b *A122
	return b, x.s
}

// nilable(x)
func (a A122) bar(x *A122, y *B122) *string {
	if x != nil {
		return &x.s
	}
	return &a.s
}

func (b B122) foo(x *A122) (*A122, string) {
	return x, x.s // this is safe because struct of type B is never used as the interface type I
}

// nilable(y, result 0)
func (b *B122) bar(x *A122, y *B122) *string { //want "passed as param"
	if b.i+y.i > 5 { //want "accessed field `i`"
		return nil
	}
	return &x.s
}

func m122_1() {
	// slice is declared to be of interface type I122, but struct *A122 is added to it
	slice := make([]I122, 2)
	slice[0] = &A122{}
	print(slice)
}

func m122_2() {
	// slice is declared to be of interface type J122, but struct *B122 is added to it
	slice := make([]J122, 0)
	b := &B122{}
	slice = append(slice, nil, b, nil)
}

// Similar case, just the slice is initialized using a composite

type I122_3 interface {
	// nilable(x)
	foo(x *A122_3) (*A122_3, string) //want "returned as result"
}

type A122_3 struct {
	s string
}

// nilable(result 0)
func (A122_3) foo(x *A122_3) (*A122_3, string) { //want "passed as param"
	var b *A122_3
	return b, x.s
}

func m122_3() {
	// Type of slice element is interface, but a struct is added to it
	slice := []I122_3{&A122_3{}}
	print(slice)
}

// 3) If map is declared to be of interface type, but struct is added to it. The test cases check if Nilaway
// treats the struct as it implements the corresponding interface.

type I123 interface {
	// nilable(x)
	foo(x *A123) (*A123, string) //want "returned as result"
}

type A123 struct {
	s string
}

// nilable(result 0)
func (A123) foo(x *A123) (*A123, string) { //want "passed as param"
	var b *A123
	return b, x.s
}

func m123() {
	mp := make(map[int]I123)
	mp[1] = &A123{}
}

// Similar case, just the struct is added to the map at initialization

type I123_2 interface {
	// nilable(x)
	foo(x *A123_2) (*A123_2, string) //want "returned as result"
}

type A123_2 struct {
	s string
}

// nilable(result 0)
func (A123_2) foo(x *A123_2) (*A123_2, string) { //want "passed as param"
	var b *A123_2
	return b, x.s
}

func m123_2() {
	var mp = map[int]I123_2{0: &A123_2{}}
	print(mp)
}
