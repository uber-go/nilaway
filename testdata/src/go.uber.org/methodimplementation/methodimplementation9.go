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
This is a test for checking the *true negative* case for nilability-type variance. Covariance for return types and contravariance for parameter
types of methods implementing an interface.
interface method returns non-nil, implementing method returns nil --> covariance violation
interface method param2 nil, implementing method param2 non-nil --> contravariance violation
*/

package methodimplementation

type I9 interface {
	// nilable(result 0)
	foo(x *A9) *string
}

type A9 struct {
	s string
}

// nilable(x)
func (A9) foo(x *A9) *string {
	if x != nil {
		return &x.s
	}
	s := ""
	return &s
}

func m9() {
	var i I9 = &A9{}
	i.foo(&A9{})
}
