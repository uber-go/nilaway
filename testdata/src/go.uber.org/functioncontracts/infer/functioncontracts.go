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
This package aims to test automated inferred function contracts in no inference mode.

<nilaway no inference>
<nilaway contract enable>
*/
package infer

import "math/rand"

// Test the contracted function contains a full trigger nilable -> return 0.
// nilable(x, result 0)
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
	b1 := fooReturn(a1) // nonnil(param 0, result 0)
	print(*b1)          // No "nilable value dereferenced" wanted
}

func barReturn2() {
	var a2 *int
	b2 := fooReturn(a2) // nilable(param 0, result 0)
	print(*b2)          // want "nilable value dereferenced"
}

// Test the contracted function retains a full trigger param 0 -> nonnil.
// nilable(x, result 0)
func fooParam(x *int) *int {
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		sink(*x) // want "nilable value dereferenced"
		return nil
	}
}

func barParam1() {
	n := 1
	a1 := &n
	b1 := fooParam(a1) // nonnil(param 0, result 0)
	print(*b1)
}

func barParam2() {
	var a2 *int
	b2 := fooParam(a2) // nilable(param 0, result 0)
	print(*b2)         // want "nilable value dereferenced"
}

func sink(v int) {}

// Test the contracted function contains another contracted function.
// TODO: remove the contract here when we can automatically infer the contract for this function.
// contract(nonnil -> nonnil)
// nilable(x, result 0)
func fooNested(x *int) *int {
	return fooBase(x)
}

// nilable(x, result 0)
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
	b1 := fooNested(a1) // nonnil(param 0, result 0)
	print(*b1)          // No "nilable value dereferenced" wanted
}

func barNested2() {
	var a2 *int
	b2 := fooNested(a2) // nilable(param 0, result 0)
	print(*b2)          // want "nilable value dereferenced"
}

// Test the contracted function is called multiple times in another function.
// nilable(x, result 0)
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
	b1 := fooReturnCalledMultipleTimesInTheSameFunction(a1) // nonnil(param 0, result 0)
	print(*b1)                                              // No "nilable value dereferenced" wanted

	var a2 *int
	b2 := fooReturnCalledMultipleTimesInTheSameFunction(a2) // nilable(param 0, result 0)
	print(*b2)                                              // want "nilable value dereferenced"

	m := 2
	a3 := &m
	b3 := fooReturnCalledMultipleTimesInTheSameFunction(a3) // nonnil(param 0, result 0)
	print(*b3)                                              // No "nilable value dereferenced" wanted

	var a4 *int
	b4 := fooReturnCalledMultipleTimesInTheSameFunction(a4) // nilable(param 0, result 0)
	print(*b4)                                              // want "nilable value dereferenced"
}

// Test call site annotations are wrongly written.
// nilable(x, result 0)
func fooWrongCallSiteAnnotation(x *int) *int {
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

func barWrongCallSiteAnnotation() {
	var a *int
	b := fooWrongCallSiteAnnotation(a) // nonnil(param 0, result 0) // want "read from a variable that was never assigned to"
	print(*b)                          // safe because the call site annotation is used
}

// nonnil(x) nilable(result 0)
func fooNoCallSiteAnnoatation(x *int) *int {
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

func barNoCallSiteAnnoatation() {
	var a *int
	// We should rely on the function header annotations if we do not find any call site
	// annotations.
	v := fooNoCallSiteAnnoatation(a) // want "nilable value passed"
	print(*v)                        // want "nilable value dereferenced"
}

// nilable(x, y, result 0, result 1)
func fooContractAnyNonnilToNonnilAnyReturn(x, y *int) (*int, *int) {
	if y != nil {
		return new(int), nil
	}
	return nil, x
}

func barContractAnyNonnilToNonnilAnyReturn1() {
	n := 1
	a1 := &n
	b1, _ := fooContractAnyNonnilToNonnilAnyReturn(nil, a1) // nonnil(param 1, result 0)
	print(*b1)                                              // No "nilable value dereferenced" wanted
}

func barContractAnyNonnilToNonnilAnyReturn2() {
	var a2 *int
	n := 1
	b2, _ := fooContractAnyNonnilToNonnilAnyReturn(&n, a2) // nilable(param 1, result 0)
	print(*b2)                                             // want "nilable value dereferenced"
}

// nilable(x, y, result 0, result 1)
func fooContractAnyNonnilToNonnilAnyParam(x, y *int) (*int, *int) {
	if y != nil {
		return new(int), nil
	}
	sink(*y) // want "nilable value dereferenced"
	return nil, x
}

func barContractAnyNonnilToNonnilAnyParam1() {
	n := 1
	a1 := &n
	b1, _ := fooContractAnyNonnilToNonnilAnyParam(nil, a1) // nonnil(param 1, result 0)
	print(*b1)                                             // No "nilable value dereferenced" wanted
}

func barContractAnyNonnilToNonnilAnyParam2() {
	var a2 *int
	n := 1
	b2, _ := fooContractAnyNonnilToNonnilAnyParam(&n, a2) // nilable(param 1, result 0)
	print(*b2)                                            // want "nilable value dereferenced"
}
