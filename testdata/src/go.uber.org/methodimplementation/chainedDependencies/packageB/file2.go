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
package packageB

import (
	"go.uber.org/methodimplementation/chainedDependencies/packageA"
)

type S1 struct{}

func (*S1) Foo(n *int) bool { //want "could be passed as param"
	v := &n
	return v != nil
}

func m() {
	var i1 packageA.I1 = new(S1)
	i1.Foo(nil)
}

type I2 interface {
	Bar() *string
}
