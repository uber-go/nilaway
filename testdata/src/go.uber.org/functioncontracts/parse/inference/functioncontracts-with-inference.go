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
This package aims to test hand-written function contracts in full inference mode.

<nilaway contract enable>
*/
package inference

import "math/rand"

// Test the contracted function contains a full trigger nilable -> return 0.
// contract(nonnil -> nonnil)
func fooReturn(x *int) *int { // want "^ Annotation on Result 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	if x != nil {
		// Return nonnil
		return new(int)
	}
	// Return nonnil or nil randomly
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		return nil
	}
}

func barReturn1() {
	n := 1
	a1 := &n
	b1 := fooReturn(a1)
	print(*b1) // No "nilable value dereferenced" wanted
}

func barReturn2() {
	var a2 *int
	b2 := fooReturn(a2)
	print(*b2) // "nilable value dereferenced" wanted
}

// Test the contracted function contains a full trigger param 0 -> nonnil.
// contract(nonnil -> nonnil)
func fooParam(x *int) *int { // want "^ Annotation on Param 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$" "^ Annotation on Result 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		sink(*x) // "nilable value dereferenced" wanted
		return nil
	}
}

func barParam1() {
	n := 1
	a1 := &n
	b1 := fooParam(a1)
	print(*b1)
}

func barParam2() {
	var a2 *int
	b2 := fooParam(a2)
	print(*b2)
}

func sink(v int) {}

// Test the contracted function contains another contracted function.
// contract(nonnil -> nonnil)
func fooNested(x *int) *int {
	return fooBase(x)
}

// contract(nonnil -> nonnil)
func fooBase(x *int) *int { // want "^ Annotation on Result 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return nil
	} else {
		return new(int)
	}
}

func barNested1() {
	n := 1
	a1 := &n
	b1 := fooNested(a1)
	print(*b1) // No "nilable value dereferenced" wanted
}

func barNested2() {
	var a2 *int
	b2 := fooNested(a2)
	print(*b2) // "nilable value dereferenced" wanted
}

// Test the contracted function is called by another function.
// contract(nonnil -> nonnil)
func fooParamCalledInAnotherFunction(x *int) *int { // want "^ Annotation on Param 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		sink(*x) // "nilable value dereferenced" wanted
		return nil
	}
}

func barParamCalledInAnotherFunction() {
	var x *int
	call(fooParamCalledInAnotherFunction(x))
}

func call(x *int) {}

// Test a contracted function is called multiple times in another function.
// contract(nonnil->nonnil)
func fooReturnCalledMultipleTimesInTheSameFunction(x *int) *int { // want "^ Annotation on Result 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$" "^ Annotation on Result 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		return nil
	}
}

func barReturnCalledMultipleTimesInTheSameFunction() {
	n := 1
	a1 := &n
	b1 := fooReturnCalledMultipleTimesInTheSameFunction(a1)
	print(*b1) // No "nilable value dereferenced" wanted

	var a2 *int
	b2 := fooReturnCalledMultipleTimesInTheSameFunction(a2)
	print(*b2) // "nilable value dereferenced" wanted

	m := 2
	a3 := &m
	b3 := fooReturnCalledMultipleTimesInTheSameFunction(a3)
	print(*b3) // No "nilable value dereferenced" wanted

	var a4 *int
	b4 := fooReturnCalledMultipleTimesInTheSameFunction(a4)
	print(*b4) // "nilable value dereferenced" wanted
}

// Contract below isn't useful, since return is always nonnil and argument is ignored, but added to
// check we don't crash on unnamed parameters.
// contract(nonnil -> nonnil)
func fooUnamedParam(_ *int) *int {
	return new(int)
}

func barUnamedParam1() {
	var a1 *int
	b1 := fooUnamedParam(a1)
	print(*b1) // No "nilable value dereferenced" wanted
}

func barUnamedParam2() {
	var a2 *int
	b2 := fooUnamedParam(a2)
	print(*b2) // No "nilable value dereferenced" wanted
}
