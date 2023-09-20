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
interface method param2 nil, implementing method param2 non-nil --> contravariance violation
*/

package methodimplementation

type I2 interface {
	// nilable(x)
	foo(x *A2) (*A2, string) //want "could be returned as result"
}

type A2 struct {
	s string
}

// nilable(result 0)
func (A2) foo(x *A2) (*A2, string) { //want "could be passed as param"
	var b *A2
	return b, x.s
}

func m2() {
	var nilV *A2

	// site 2: interface variable declaration and initialization using concrete type
	var v2 I2 = &A2{}
	v2.foo(nilV)
}
