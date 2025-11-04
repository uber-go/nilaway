//  Copyright (c) 2025 Uber Technologies, Inc.
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

package customfuncnilchecks

func isPtrNonnilSimple(x *int) bool {
	return x != nil
}

func isPtrNonnilComplex(x *int) bool {
	return x != nil && *x > 0
}

func areBothNonnil(x, y *int) bool {
	return x != nil && y != nil
}

func isOneNonnil(x, y *int) bool {
	return x != nil || y != nil
}

func isStringPtrNonnil(s *string) bool {
	return s != nil
}

func isSlicePtrNonnil(arr *[]int) bool {
	return arr != nil && len(*arr) > 0
}

type Container struct {
	ptr *int
}

func hasValidPtr(c *Container) bool {
	return c != nil && c.ptr != nil
}

func isValidRange(x, y *int) bool {
	return x != nil && y != nil && *x <= *y
}

// Global variables for testing
var globalPtr *int
var globalStr *string
var globalSlice []int

func isGlobalPtrNonnil() bool {
	return globalPtr != nil
}

func isGlobalStrNonnil() bool {
	return globalStr != nil
}

func isGlobalSliceNonnil() bool {
	return len(globalSlice) > 0 && globalSlice[0] != 0
}

func areGlobalsNonnil() bool {
	return globalPtr != nil && globalStr != nil
}

func isAnyGlobalNonnil() bool {
	return globalPtr != nil || globalStr != nil || len(globalSlice) > 0
}

func testSingleParam(s string) {
	switch s {
	case "custom func simple nil check, safe":
		var p *int
		if isPtrNonnilSimple(p) {
			print(*p)
		}

	case "custom func simple nil check, unsafe":
		var p *int
		if !isPtrNonnilSimple(p) {
			print(*p) //want "dereferenced"
		}

	case "custom func simple nil check, safe, nonnil assigned":
		p := new(int)
		if isPtrNonnilSimple(p) {
			print(*p)
		}

		if !isPtrNonnilSimple(p) {
			print(*p)
		}

	case "custom func simple nil check, nested conditional, safe":
		var p *int
		if isPtrNonnilSimple(p) && *p > 0 {
			print(*p)
		}

	case "custom func simple nil check, nested conditional, unsafe":
		var p *int
		if isPtrNonnilSimple(p) || *p > 0 { //want "dereferenced"
			print(*p) //want "dereferenced"
		}

	case "custom func complex nil check, safe":
		var p *int
		if isPtrNonnilComplex(p) {
			print(*p)
		}

	case "custom func complex nil check, unsafe":
		var p *int
		if !isPtrNonnilComplex(p) {
			print(*p) //want "dereferenced"
		}

	case "custom func complex nil check, safe, nonnil assigned":
		p := new(int)
		if isPtrNonnilComplex(p) {
			print(*p)
		}

		if !isPtrNonnilComplex(p) {
			print(*p)
		}

	case "custom func complex nil check, nested conditional, safe":
		var p, q *int
		if isPtrNonnilComplex(p) && q != nil {
			_ = *p
		}

	case "custom func complex nil check, nested conditional, unsafe":
		var p, q *int
		if !isPtrNonnilComplex(p) || q != nil {
			print(*p) //want "dereferenced"
		}
	}
}

func testMultipleParams() {
	var p, q *int

	// Test with both nil
	if areBothNonnil(p, q) {
		print(*p, *q) // should be safe
	}

	if !areBothNonnil(p, q) {
		print(*p) //want "dereferenced"
	}

	if isOneNonnil(p, q) {
		print(*q) //want "dereferenced"
	}

	// Test with one non-nil, one nil
	r := new(int)
	if areBothNonnil(r, q) {
		print(*r, *q)
	}

	if areBothNonnil(p, r) {
		print(*p, *r)
	}

	if isOneNonnil(r, q) {
		print(*r)
	}

	if isOneNonnil(p, r) {
		print(*p) //want "dereferenced"
	}

	// Test with both non-nil
	s := new(int)
	if areBothNonnil(r, s) {
		print(*r, *s) // safe
	}
}

func testDifferentTypes() {
	var s *string
	if isStringPtrNonnil(s) {
		print(*s) // safe
	}

	if !isStringPtrNonnil(s) {
		print(*s) //want "dereferenced"
	}

	str := new(string)
	if isStringPtrNonnil(str) {
		print(*str) // safe
	}

	var arr *[]int
	if isSlicePtrNonnil(arr) {
		print((*arr)[0]) // safe
	}

	if !isSlicePtrNonnil(arr) {
		print((*arr)[0]) //want "dereferenced"
	}

	// Test with non-nil but empty slice
	emptySlice := make([]int, 0)
	arrPtr := &emptySlice
	if isSlicePtrNonnil(arrPtr) {
		print((*arrPtr)[0])
	}

	// Test with non-nil and non-empty slice
	nonEmptySlice := []int{1, 2, 3}
	arrPtr2 := &nonEmptySlice
	if isSlicePtrNonnil(arrPtr2) {
		print((*arrPtr2)[0]) // safe
	}
}

func testStructFields() {
	var c *Container
	if hasValidPtr(c) {
		print(*c.ptr) // safe
	}

	if !hasValidPtr(c) {
		print(*c.ptr) //want "accessed field"
	}

	// Test with non-nil container but nil ptr field
	c2 := &Container{ptr: nil}
	if hasValidPtr(c2) {
		print(*c2.ptr) // safe
	}

	if !hasValidPtr(c2) {
		// TODO: below field dereference should be reported after we add support for struct init analysis.
		print(*c2.ptr) // "dereferenced"
	}

	// Test with both non-nil
	val := 42
	c3 := &Container{ptr: &val}
	if hasValidPtr(c3) {
		print(*c3.ptr) // safe
	}
}

func testComplexLogic() {
	var x, y *int
	if isValidRange(x, y) {
		print(*x, *y) // safe
	}

	if !isValidRange(x, y) {
		print(*x) //want "dereferenced"
	}

	// Test with one non-nil
	a := new(int)
	*a = 10
	if isValidRange(a, y) {
		print(*a, *y)
	}

	// Test with both non-nil but invalid range
	b := new(int)
	*b = 5
	if isValidRange(a, b) { // a=10, b=5, so 10 <= 5 is false
		print(*a, *b)
	}

	// Test with valid range
	c := new(int)
	*c = 15
	if isValidRange(a, c) { // a=10, c=15, so 10 <= 15 is true
		print(*a, *c)
	}
}

func testGlobalVariables() {
	// Reset globals to nil
	globalPtr = nil
	globalStr = nil
	globalSlice = nil

	// Test with all globals nil
	if isGlobalPtrNonnil() {
		print(*globalPtr) // safe
	}

	if !isGlobalPtrNonnil() {
		print(*globalPtr) //want "dereferenced"
	}

	if isGlobalStrNonnil() {
		print(*globalStr) // safe
	}

	if !isGlobalStrNonnil() {
		print(*globalStr) //want "dereferenced"
	}

	if isGlobalSliceNonnil() {
		print((globalSlice)[0]) // safe
	}

	if !isGlobalSliceNonnil() {
		print((globalSlice)[0]) //want "sliced"
	}

	// Test multiple global checks
	if areGlobalsNonnil() {
		print(*globalPtr, *globalStr) // safe
	}

	if !areGlobalsNonnil() {
		print(*globalPtr) //want "dereferenced"
		print(*globalStr) //want "dereferenced"
	}

	if isAnyGlobalNonnil() {
		// could be nil even if others are non-nil
		print(*globalPtr) //want "dereferenced"
	}

	// Test with some globals set
	globalPtr = new(int)
	*globalPtr = 42

	if isGlobalPtrNonnil() {
		print(*globalPtr) // safe
	}

	if areGlobalsNonnil() {
		print(*globalPtr, *globalStr)
	}

	if isAnyGlobalNonnil() {
		print(*globalPtr) // safe - we know globalPtr is non-nil
		// globalStr still nil
		print(*globalStr) //want "dereferenced"
	}

	// Test with all globals set
	str := "hello"
	globalStr = &str
	slice := []int{1, 2, 3}
	globalSlice = slice

	if areGlobalsNonnil() {
		print(*globalPtr, *globalStr) // safe
	}

	if isGlobalSliceNonnil() {
		print((globalSlice)[0]) // safe
	}

	if isAnyGlobalNonnil() {
		// All are non-nil, but we can only safely dereference if we know which specific one
		print(*globalPtr)       // safe
		print(*globalStr)       // safe
		print((globalSlice)[0]) // safe
	}
}

func testGlobalInNestedConditions() {
	globalPtr = nil
	globalStr = nil

	// Test global checks in nested conditions
	if isGlobalPtrNonnil() && *globalPtr > 0 {
		print(*globalPtr) // safe
	}

	if isGlobalPtrNonnil() || *globalPtr > 0 { //want "dereferenced"
		print(*globalPtr) //want "dereferenced"
	}

	// Test with non-nil global
	value := 10
	globalPtr = &value

	if isGlobalPtrNonnil() && *globalPtr > 0 {
		print(*globalPtr) // safe
	}

	if isGlobalPtrNonnil() || *globalPtr > 0 {
		print(*globalPtr) // safe
	}

	// Test mixed global and local conditions
	var localPtr *int
	if isGlobalPtrNonnil() && !isPtrNonnilSimple(localPtr) {
		// localPtr is nil
		print(*globalPtr, *localPtr) //want "dereferenced"
	}

	localPtr = new(int)
	if isGlobalPtrNonnil() && !isPtrNonnilSimple(localPtr) {
		print(*globalPtr, *localPtr) // safe
	}
}
