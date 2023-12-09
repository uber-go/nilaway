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

// This package aims to test hand-written function contracts in no inference mode.
// <nilaway no inference>
package functioncontracts

import "math/rand"

// Test the contracted function contains a full trigger nilable -> return 0.
// nilable(x, result 0)
// contract(nonnil -> nonnil)
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
	print(*b2)          // want "dereferenced"
}

// Test the contracted function retains a full trigger param 0 -> nonnil.
// nilable(x, result 0)
// contract(nonnil -> nonnil)
func fooParam(x *int) *int {
	if x != nil {
		return new(int)
	}
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		sink(*x) // want "dereferenced"
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
	print(*b2)         // want "dereferenced"
}

func sink(v int) {}

// Test the contracted function contains another contracted function.
// nilable(x, result 0)
// contract(nonnil -> nonnil)
func fooNested(x *int) *int {
	return fooBase(x)
}

// contract(nonnil -> nonnil)
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
	print(*b2)          // want "dereferenced"
}

// Test the contracted function is called multiple times in another function.
// contract(nonnil -> nonnil)
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
	print(*b2)                                              // want "dereferenced"

	m := 2
	a3 := &m
	b3 := fooReturnCalledMultipleTimesInTheSameFunction(a3) // nonnil(param 0, result 0)
	print(*b3)                                              // No "nilable value dereferenced" wanted

	var a4 *int
	b4 := fooReturnCalledMultipleTimesInTheSameFunction(a4) // nilable(param 0, result 0)
	print(*b4)                                              // want "dereferenced"
}

// Test call site annotations are wrongly written.
// nilable(x, result 0)
// contract(nonnil -> nonnil)
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
	b := fooWrongCallSiteAnnotation(a) // nonnil(param 0, result 0) // want "unassigned variable `a`"
	print(*b)                          // safe because the call site annotation is used
}

// nonnil(x) nilable(result 0)
// contract(nonnil -> nonnil)
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
	v := fooNoCallSiteAnnoatation(a) // want "passed"
	print(*v)                        // want "dereferenced"
}

// Contract below isn't useful, since return is always nonnil and argument is ignored, but added to
// check we don't crash on unnamed parameters.
// contract(nonnil -> nonnil)
func fooUnnamedParam(_ *int) *int {
	return new(int)
}

func barUnnamedParam1() {
	var a1 *int
	b1 := fooUnnamedParam(a1) // nilable(param 0) nonnil(result 0)
	print(*b1) // No error here.
}

func barUnnamedParam2() {
	var a2 *int
	b2 := fooUnnamedParam(a2) // nilable(param 0) nonnil(result 0)
	print(*b2) // No error here.
}
