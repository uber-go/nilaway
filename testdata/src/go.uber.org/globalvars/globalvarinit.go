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
These tests check if the nonnil global variables are initialized

<nilaway no inference>
*/
package globalvars

import "go.uber.org/globalvars/upstream"

var x = 3

// This should throw an error since it is not initialized
var noInit *int //want "assigned into global variable"
var _init *int
var _initMult1, _initMult2 *int

func init() {
	_init = new(int)
	_initMult1 = new(int)
	_initMult2 = new(int)
}

// nilable(nilableVar)
var nilableVar *int
var assignedNilable = nilableVar //want "assigned"

var initMult, noInitMult *int = &x, nil //want "assigned"

// Use of 1-1 assignment and a function call
var initNew, noInitAgain = new(*int), nilableFun() //want "assigned"

// nilable(result 0)
func nilableFun() *string {
	return noInitButNilable
}

// nilable(noInitButNilable)
var noInitButNilable *string

// The default value is not nil, thus should not give an error
var primitive string
var prim1, prim2 = "abb", 7

// Check if the call to anonymous function is ignored

type K []func(map[string]interface{})

var k = K(nil)

// Check if blank global variables are ignored

var (
	_ = new(int)
	_ = new(*int)
)

// nilable(A)
type structA struct {
	A *int
}

var stA = &structA{}
var nilableField = stA.A //want "assigned"

// nilable(result 0)
func (structA) methA() *int {
	return nil
}

func (structA) methB() *int {
	return new(int)
}

var nilableMethod, nonnilMethod = stA.methA(), stA.methB() //want "assigned"

// Function with multiple returns

// nilable(result 1)
func funMulti() (int, *int) {
	return 2, new(int)
}

var multiNonNil, multiNil = funMulti() //want "assigned"

// nilable(result 0)
func foo() *int {
	// Just arbitrary use of all the vars to avoid unused var errors
	print(noInit, primitive, prim1, prim2, noInitButNilable, nilableMethod)
	print(noInitMult, initMult, initNew, noInitAgain, nilableField)
	print(multiNonNil, multiNil, nonnilMethod, assignedNilable)
	return nil
}

// Below test checks when a constant is assigned to a global variable.

// ErrorNoFailure is a constant marking a failure.
const ErrorNoFailure = upstream.ErrorNo(42)

// Now, we can assign the (nonnil) constant ErrorNoFailure to a global variable.
var invalidSyscall error = ErrorNoFailure

// Assign it again, but from an upstream package.
var invalidSyscallUpstream error = upstream.ErrorNoFailure
