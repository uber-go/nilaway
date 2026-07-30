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

// This file tests automatically inferred function contracts.

package functioncontracts

import "math/rand"

// Test the contracted function contains a full trigger nilable -> return 0.
func inferredFooReturn(x *int) *int {
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

func inferredBarReturn1() {
	n := 1
	a1 := &n
	b1 := inferredFooReturn(a1)
	print(*b1) // No error due to the contract.
}

func inferredBarReturn2() {
	var a2 *int
	b2 := inferredFooReturn(a2)
	print(*b2) // want "result 0 of `inferredFooReturn.*` .* dereferenced"
}

// Test the contracted function contains a full trigger param 0 -> nonnil.
func inferredFooParam(x *int) *int {
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

func inferredBarParam1() {
	n := 1
	a1 := &n
	b1 := inferredFooParam(a1)
	print(*b1)
}

func inferredBarParam2() {
	var a2 *int
	b2 := inferredFooParam(a2)
	print(*b2) // want "result 0 of `inferredFooParam.*` .* dereferenced"
}

// Test the contracted function contains another contracted function.
// TODO: remove the contract here when we can automatically infer the contract for this function.
// contract(nonnil -> nonnil)
func inferredFooNested(x *int) *int {
	return inferredFooBase(x)
}

func inferredFooBase(x *int) *int {
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return nil
	} else {
		return new(int)
	}
}

func inferredBarNested1() {
	n := 1
	a1 := &n
	b1 := inferredFooNested(a1)
	print(*b1) // No error here due to the contract.
}

func inferredBarNested2() {
	var a2 *int
	b2 := inferredFooNested(a2)
	print(*b2) // want "result 0 of `inferredFooNested.*` .* dereferenced"
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
func inferredFooReturnCalledMultipleTimesInTheSameFunction(x *int) *int {
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		return nil
	}
}

func inferredBarReturnCalledMultipleTimesInTheSameFunction() {
	n := 1
	a1 := &n
	b1 := inferredFooReturnCalledMultipleTimesInTheSameFunction(a1)
	print(*b1) // No error here due to the contract.

	var a2 *int
	b2 := inferredFooReturnCalledMultipleTimesInTheSameFunction(a2)
	print(*b2) // want "result 0 of `inferredFooReturnCalledMultipleTimesInTheSameFunction.*` .* dereferenced"

	m := 2
	a3 := &m
	b3 := inferredFooReturnCalledMultipleTimesInTheSameFunction(a3)
	print(*b3) // No error here due to the contract.

	var a4 *int
	b4 := inferredFooReturnCalledMultipleTimesInTheSameFunction(a4)
	print(*b4) // want "result 0 of `inferredFooReturnCalledMultipleTimesInTheSameFunction.*` .* dereferenced"
}
