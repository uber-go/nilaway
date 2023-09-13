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
Package paramfield Tests when nilability flows through the field of param of a function or a method
<nilaway struct enable>
*/
package paramfield

// Negative example

func callM21() {
	t := &A{}
	t.aptr = &A{}
	t.m21()
}

func (c *A) m21() {
	print(c.aptr.ptr)
}

// Positive example

func callM22() {
	t := &A{}
	t.m22()
}

func (c *A) m22() {
	print(c.aptr.ptr) //want "field `aptr` of receiver of call to function `m22`"
}

// Checking if Nilaway does not crash on unnamed receivers
func (*A) m23() {}

// Positive example with direct composite as parameter

func callF24() {
	(A{}).f24()
}

func (c A) f24() {
	print(c.aptr.ptr) //want "field `aptr` of receiver of call to function `f24`"
}

// Positive example with direct composite as parameter
func giveA25() *A {
	return &A{}
}

func callF25() {
	giveA25().f25()
}

func (c *A) f25() {
	print(c.aptr.ptr) //want "field `aptr` of receiver of call to function `f25`"
}

// Negative example with direct composite as parameter

func callF26() {
	(&A{aptr: &A{}}).f26()
}

func (c *A) f26() {
	print(c.aptr.ptr)
}

// Negative example with direct composite as parameter
func giveA27() *A {
	return &A{aptr: new(A)}
}

func callF27() {
	giveA27().f27()
}

func (c *A) f27() {
	print(c.aptr.ptr)
}

// No Nilaway error in this case.
// But it is a special case and Nilaway should not crash on it

type someError struct {
	error
}

func (n someError) IsSomeError() bool {
	return true
}
