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

// This test file tests a corner case for anonymous functions, where we should not affiliate the
// return statements inside the anonymous functions with the outermost function return type.
// <nilaway no inference>

package methodimplementation

// IA13 and IB13 are two different interfaces coincidentally both having a Get method, but their
// arguments are different.
type IA13 interface {
	// nilable(x)
	Get(x *int) int
}

type IB13 interface {
	// nilable(x, y)
	Get(x *int, y *int) int
}

// A13 and B13 are two structs that implement IA13 and IB13 interfaces, respectively.
type A13 struct{}

// nilable(x)
func (a *A13) Get(x *int) int { return 0 }

type B13 struct{}

// nilable(x, y)
func (b *B13) Get(x *int, y *int) int { return 0 }

func f1() IA13 {
	// We should not affiliate "return B13" with the function return interface type IA13.
	getter := func() IB13 { return &B13{} }
	getter()
	return &A13{}
}

// Now, we test a case where the two interfaces have a same-name method with same arguments (i.e.,
// they are functionally the same interfaces), but the niliability requirements are different.
// No errors should be reported if our assignments follow the nilability requirements.

// IC13 and C13 have a Get method with a nilable argument, meanwhile ID13 and D13 have a Get method
// with a nonnil argument. If we only assign C13 -> IC13, D13 -> ID13, C13 -> ID13, no errors
// should be reported.

type IC13 interface {
	// niable(x)
	Get(x *bool) int
}

type ID13 interface {
	// nonnil(x)
	Get(x *bool) int
}

// C13 and D13 are two structs that implement IC13 and ID13 interfaces, respectively.
type C13 struct{}

// nilable(x)
func (c *C13) Get(x *bool) int { return 0 }

type D13 struct{}

// nonnil(x)
func (d *D13) Get(x *bool) int { return 0 }

func f2() ID13 {
	getter := func() ID13 { return &D13{} }
	getter()
	return &C13{}
}
