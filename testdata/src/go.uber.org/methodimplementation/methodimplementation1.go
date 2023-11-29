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

<nilaway no inference>
*/
package methodimplementation

type I interface {
	// nilable(x)
	foo(x *A) (*A, string) //want "returned as result"
}

type J interface {
	// nilable(x, result 0)
	bar(x *A, y *B) *string
}

type A struct {
	s string
}

type B struct {
	i int
}

// nilable(result 0)
func (A) foo(x *A) (*A, string) { //want "passed as param"
	var b *A
	return b, x.s
}

// nilable(x)
func (a A) bar(x *A, y *B) *string {
	if x != nil {
		return &x.s
	}
	return &a.s
}

func (b B) foo(x *A) (*A, string) {
	return x, x.s // this is safe because struct of type B is never used as the interface type I
}

// nilable(y, result 0)
func (b *B) bar(x *A, y *B) *string { //want "passed as param"
	if b.i+y.i > 5 { //want "accessed field `i`"
		return nil
	}
	return &x.s
}

func m() {
	// site 1: assignment of a concrete implementation to an interface type
	var v1 I
	v1 = &A{}
	v1.foo(new(A))

	var v2 J
	v2 = &B{}
	v2.bar(new(A), new(B))
}
