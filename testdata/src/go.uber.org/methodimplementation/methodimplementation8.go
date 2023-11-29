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

type I8 interface {
	// nilable(x)
	foo(x *A8) (*A8, string) //want "returned as result"
}

type A8 struct {
	s string
}

type NamedType int

// nilable(result 0)
func (NamedType) foo(x *A8) (*A8, string) { //want "passed as param"
	var b *A8
	return b, x.s
}

func m8() {
	// site 7: a named type (non-struct) implementing an interface
	var i I8 = new(NamedType)
	i.foo(&A8{})
}
