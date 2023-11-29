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

// <nilaway no inference>
package packageA

type I1 interface {
	Foo1() *int //want "returned as result"

	// nilable(n)
	Foo2(n *int) bool
}

type S1 struct{}

// nilable(result 0)
func (*S1) Foo1() *int {
	var v *int
	return v
}

func (*S1) Foo2(n *int) bool { //want "passed as param"
	v := &n
	return v != nil
}

func main() {
	var i1 I1 = new(S1)
	i1.Foo2(i1.Foo1())
}
