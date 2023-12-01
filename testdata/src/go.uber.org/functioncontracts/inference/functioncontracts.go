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


// This package aims to test automated inferred function contracts in full inference mode.

package inference

import "math/rand"

// Test the contracted function contains a full trigger nilable -> return 0.
func fooReturn(x *int) *int {
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
	print(*b1) // No error due to the contract.
}

func barReturn2() {
	var a2 *int
	b2 := fooReturn(a2)
	print(*b2) // want "result 0 of `fooReturn.*` .* dereferenced"
}

// Test the contracted function contains a full trigger param 0 -> nonnil.
func fooParam(x *int) *int {
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		sink(*x) // want "function parameter `x` .* dereferenced"
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
	print(*b2) // want "result 0 of `fooParam.*` .* dereferenced"
}

func sink(v int) {}

// Test the contracted function contains another contracted function.
// TODO: remove the contract here when we can automatically infer the contract for this function.
// contract(nonnil -> nonnil)
func fooNested(x *int) *int {
	return fooBase(x)
}

func fooBase(x *int) *int {
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
	print(*b1) // No error here due to the contract.
}

func barNested2() {
	var a2 *int
	b2 := fooNested(a2)
	print(*b2) // want "result 0 of `fooNested.*` .* dereferenced"
}

// Test the contracted function is called by another function.
func fooParamCalledInAnotherFunction(x *int) *int {
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		sink(*x) // want "function parameter `x` .* dereferenced"
		return nil
	}
}

func barParamCalledInAnotherFunction() {
	var x *int
	call(fooParamCalledInAnotherFunction(x))
}

func call(x *int) {}

// Test a contracted function is called multiple times in another function.
func fooReturnCalledMultipleTimesInTheSameFunction(x *int) *int {
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
	print(*b1) // No error here due to the contract.

	var a2 *int
	b2 := fooReturnCalledMultipleTimesInTheSameFunction(a2)
	print(*b2) // want "result 0 of `fooReturnCalledMultipleTimesInTheSameFunction.*` .* dereferenced"

	m := 2
	a3 := &m
	b3 := fooReturnCalledMultipleTimesInTheSameFunction(a3)
	print(*b3) // No error here due to the contract.

	var a4 *int
	b4 := fooReturnCalledMultipleTimesInTheSameFunction(a4)
	print(*b4) // want "result 0 of `fooReturnCalledMultipleTimesInTheSameFunction.*` .* dereferenced"
}
