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

type I5 interface {
	// nilable(x)
	foo(x *A5) (*A5, string) //want "nilable value could be returned as result"
}

type A5 struct {
	s string
}

// nilable(result 0)
func (A5) foo(x *A5) (*A5, string) { //want "nilable value could be passed as param"
	var b *A5
	return b, x.s
}

func ret5() *A5 {
	return &A5{}
}

func param5(i I5) {
	i.foo(&A5{})
}

func m5() {
	// site 5: function return flowing into a function param2
	param5(ret5())
}
