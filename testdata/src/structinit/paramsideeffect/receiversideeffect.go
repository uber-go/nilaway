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

// Package paramsideeffect Tests when nilability flows through the field of param on a call to a function or a method
package paramsideeffect

// Tests populating Receiver of a method

func (x *A) populateMethod() {
	x.newPtr = &A{}
}

func m21() *int {
	b := &A{}
	b.aptr = &A{}
	b.populateMethod()
	print(b.newPtr.ptr)
	return b.aptr.ptr
}

// Positive test

func (x *A) populateMethod2() {
	x.newPtr = nil
}

func m22() *int {
	b := &A{}
	b.aptr = &A{}
	b.populateMethod2()
	print(b.newPtr.ptr) //want "field `newPtr` of method receiver `x`"
	return b.aptr.ptr
}
