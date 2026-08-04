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
*/
package methodimplementation

// 1) If the function signature states interface return, but the actual return is a struct.
// The test cases check if Nilaway treats the struct as it implements the corresponding interface.

type I121 interface {
	foo(x *A121) (*A121, string)
}

type J121 interface {
	bar(x *A121, y *B121) *string
}

type A121 struct {
	s string
}

type B121 struct {
	i int
}

func (A121) foo(x *A121) (*A121, string) {
	var b *A121
	return b, x.s //want "passed as param"
}

func (a A121) bar(x *A121, y *B121) *string {
	if x != nil {
		return &x.s
	}
	return &a.s
}

func (b B121) foo(x *A121) (*A121, string) {
	return x, x.s // this is safe because struct of type B is never used as the interface type I
}

func (b *B121) bar(x *A121, y *B121) *string {
	if b.i+y.i > 5 { //want "accessed field `i`"
		return nil
	}
	return &x.s //want "passed as param"
}

func dummy() *A121 {
	return &A121{}
}

func m121(x *A121, y *B121) (I121, J121) {
	// function signature states interface return, but the actual return is a struct
	return dummy(), y
}

func caller121() {
	i, j := m121(&A121{}, &B121{})
	// nil flows into the interface param, making the deref of x inside A121.foo unsafe.
	r, _ := i.foo(nil)
	// A121.foo returns a nil result, making this deref of the interface result unsafe.
	_ = r.s //want "returned as result"
	// nil flows into the interface param x, making the deref of x inside B121.bar unsafe.
	j.bar(nil, new(B121))

	// a direct call passing nil as y makes the deref of y inside B121.bar unsafe.
	b := &B121{}
	b.bar(new(A121), nil)
}

// 2) If slice is declared to be of interface type, but struct is added to it. The test cases check if Nilaway
// treats the struct as it implements the corresponding interface.

type I122 interface {
	foo(x *A122) (*A122, string)
}

type J122 interface {
	bar(x *A122, y *B122) *string
}

type A122 struct {
	s string
}

type B122 struct {
	i int
}

func (A122) foo(x *A122) (*A122, string) {
	var b *A122
	return b, x.s //want "passed as param"
}

func (a A122) bar(x *A122, y *B122) *string {
	if x != nil {
		return &x.s
	}
	return &a.s
}

func (b B122) foo(x *A122) (*A122, string) {
	return x, x.s // this is safe because struct of type B is never used as the interface type I
}

func (b *B122) bar(x *A122, y *B122) *string {
	if b.i+y.i > 5 { //want "accessed field `i`"
		return nil
	}
	return &x.s //want "passed as param"
}

func m122_1() {
	// slice is declared to be of interface type I122, but struct *A122 is added to it
	slice := make([]I122, 2)
	slice[0] = &A122{}
	if v := slice[0]; v != nil {
		// nil flows into the interface param, making the deref of x inside A122.foo unsafe.
		r, _ := v.foo(nil)
		// A122.foo returns a nil result, making this deref of the interface result unsafe.
		_ = r.s //want "returned as result"
	}
}

func m122_2() {
	// slice is declared to be of interface type J122, but struct *B122 is added to it
	slice := make([]J122, 0)
	b := &B122{}
	slice = append(slice, nil, b, nil)
	for _, j := range slice {
		if j != nil {
			// nil flows into the interface param x, making the deref of x inside B122.bar unsafe.
			j.bar(nil, new(B122))
		}
	}

	// a direct call passing nil as y makes the deref of y inside B122.bar unsafe.
	b.bar(new(A122), nil)
}

// Similar case, just the slice is initialized using a composite

type I122_3 interface {
	foo(x *A122_3) (*A122_3, string)
}

type A122_3 struct {
	s string
}

func (A122_3) foo(x *A122_3) (*A122_3, string) {
	var b *A122_3
	return b, x.s //want "passed as param"
}

func m122_3() {
	// Type of slice element is interface, but a struct is added to it
	slice := []I122_3{&A122_3{}}
	if v := slice[0]; v != nil {
		r, _ := v.foo(nil)
		_ = r.s //want "returned as result"
	}
}

// 3) If map is declared to be of interface type, but struct is added to it. The test cases check if Nilaway
// treats the struct as it implements the corresponding interface.

type I123 interface {
	foo(x *A123) (*A123, string)
}

type A123 struct {
	s string
}

func (A123) foo(x *A123) (*A123, string) {
	var b *A123
	return b, x.s //want "passed as param"
}

func m123() {
	mp := make(map[int]I123)
	mp[1] = &A123{}
	if v, ok := mp[1]; ok {
		r, _ := v.foo(nil)
		_ = r.s //want "returned as result"
	}
}

// Similar case, just the struct is added to the map at initialization

type I123_2 interface {
	foo(x *A123_2) (*A123_2, string)
}

type A123_2 struct {
	s string
}

func (A123_2) foo(x *A123_2) (*A123_2, string) {
	var b *A123_2
	return b, x.s //want "passed as param"
}

func m123_2() {
	var mp = map[int]I123_2{0: &A123_2{}}
	if v, ok := mp[0]; ok {
		r, _ := v.foo(nil)
		_ = r.s //want "returned as result"
	}
}
