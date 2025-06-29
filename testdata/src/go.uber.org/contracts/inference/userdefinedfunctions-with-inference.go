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

// Package contracts: This file tests the contract of "ok" form for user defined and standard library functions in
// full inference mode.

package inference

var dummy bool

const falseVal = false
const trueVal = true

// ***** below tests check the handling for "always safe" cases and their variants *****

func retAlwaysNonnilPtrBool(i int) (*int, bool) {
	switch i {
	case 0:
		return new(int), false
	case 1:
		return &i, trueVal
	case 2:
		return new(int), falseVal
	}
	return new(int), true
}

func retAlwaysNilPtrBool(i int) (*int, bool) {
	switch i {
	case 0:
		return nil, false
	case 1:
		return nil, trueVal
	case 2:
		return nil, falseVal
	}
	return nil, true
}

func retSometimesNilPtrBool(i int) (*int, bool) {
	switch i {
	case 0:
		return nil, false
	case 1:
		return nil, falseVal
	case 2:
		return new(int), trueVal
	}
	return new(int), true
}

func testAlwaysSafe(i int) {
	switch i {
	// always safe
	case 0:
		x, _ := retAlwaysNonnilPtrBool(i)
		print(*x)
	case 1:
		if x, ok := retAlwaysNonnilPtrBool(i); ok {
			print(*x)
		}
	case 2:
		if x, ok := retAlwaysNonnilPtrBool(i); ok {
			print(*x)
		}
	case 3:
		x, _ := retAlwaysNonnilPtrBool(i)
		y, _ := retAlwaysNonnilPtrBool(i)
		print(*x)
		print(*y)
	case 4:
		x, okx := retAlwaysNonnilPtrBool(i)
		y, oky := retAlwaysNonnilPtrBool(i)

		if oky {
			print(*x)
		}
		if okx {
			print(*y)
		}

	// always unsafe
	case 5:
		x, _ := retAlwaysNilPtrBool(i)
		print(*x) //want "dereferenced"
	case 6:
		if x, ok := retAlwaysNilPtrBool(i); ok {
			print(*x) //want "dereferenced"
		}

	// conditionally safe
	case 7:
		x, _ := retSometimesNilPtrBool(i)
		print(*x) //want "dereferenced"
	case 8:
		if x, ok := retSometimesNilPtrBool(i); ok {
			print(*x)
		}
	}
}

// Test always safe through multiple hops. Currently, we support only immediate function call for "always safe" tracking.
// Hence, the below cases are expected to report errors.
// TODO: add support for multiple hops to address the false positives

func m1() (*int, bool) {
	return m2()
}

func m2() (*int, bool) {
	v, ok := m3()
	if !ok {
		// makes non-error return always non-nil
		return new(int), false
	}
	y := *v + 1
	return &y, true
}

func m3() (*int, bool) {
	if dummy {
		return nil, false
	}
	return new(int), true
}

type S struct {
	f *int
}

func f1(i int) (*int, bool) {
	switch i {
	case 0:
		// direct non-nil non-error return value
		return new(int), false
	case 1:
		s := &S{f: new(int)}
		// indirect non-nil non-error return value via a field read
		return s.f, true
	case 2:
	}
	// indirect non-nil non-error return value via a function return
	return retAlwaysNonnilPtrBool(i)
}

func testAlwaysSafeMultipleHops() {
	// TODO: call to m1() should be reported as always safe. This is a false positive since currently we are limiting the
	//  "always safe" tracking to only immediate function call, not chained error returning function calls.
	v1, _ := m1()
	print(*v1) //want "dereferenced"

	// TODO: call to f1() should be reported as always safe. This is a false positive since currently we are limiting the
	// analysis of "return statements" to only the directly determinable cases (e.g., new(int), &S{}, NegativeNilCheck), not through multiple hops.
	v2, _ := f1(0)
	print(*v2) //want "dereferenced"
}

type Value[T any] struct {
	value T
	valid bool
}

func (v Value[T]) Get() (T, bool) {
	return v.value, v.valid
}

type V struct{}

func retV() (V, bool) {
	return V{}, true
}

func retInt() (int, bool) {
	return 42, true
}

func TestAssignmentToNonPointerTypes(i int, v Value[V]) {
	switch i {
	case 1:
		var sum *V
		if a, ok := v.Get(); ok {
			sum = &a
			_ = *sum
		}

	case 2:
		var sum *V
		if a, ok := v.Get(); ok {
			_ = &a
			_ = *sum //want "dereferenced"
		}

	case 3:
		var sum1, sum2 *V
		if a, ok := v.Get(); ok {
			sum1 = &a
			_ = *sum1
			_ = *sum2 //want "dereferenced"
		}

	case 4:
		var sum *V
		if a, ok := retV(); ok {
			sum = &a
			_ = *sum
		}

	case 5:
		var sum *V
		if a, ok := retV(); ok {
			_ = &a
			_ = *sum //want "dereferenced"
		}

	case 6:
		var sum1, sum2 *V
		if a, ok := retV(); ok {
			sum1 = &a
			_ = *sum1
			_ = *sum2 //want "dereferenced"
		}

	case 7:
		var sum *int
		if a, ok := retInt(); ok {
			sum = &a
			_ = *sum
		}

	case 8:
		var sum *int
		if a, ok := retInt(); ok {
			_ = &a
			_ = *sum //want "dereferenced"
		}

	case 9:
		var sum *V
		a, _ := retV()
		sum = &a
		_ = *sum

	case 10:
		var sum *V
		if a, ok := retV(); !ok {
			sum = &a
			_ = *sum
		}
	}
}
