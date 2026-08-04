//  Copyright (c) 2026 Uber Technologies, Inc.
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

package inference

// Implicit uses of a pointer to an array dereference the pointer and panic if it is nil:
// slicing (p[low:high] is shorthand for (*p)[low:high], even for forms like p[:0] that are safe
// on a nil slice), indexing (p[i] is (*p)[i], both reads and writes), and ranging with a
// non-blank second iteration variable (which reads the elements). Forms that never read the
// elements are nil-safe: len(p) and cap(p) are constants, and range loops with at most one
// (possibly blank) iteration variable do not even evaluate the range expression.

var arrayPtrDummy bool

func nilArrayPtr() *[4]int {
	if arrayPtrDummy {
		return nil
	}
	return &[4]int{}
}

func testArrayPtrSliceSafeForm() []int {
	p := nilArrayPtr()
	return p[:0] //want "dereferenced"
}

func testArrayPtrSliceFull() []int {
	p := nilArrayPtr()
	return p[:] //want "dereferenced"
}

func testArrayPtrSliceBounds() []int {
	p := nilArrayPtr()
	return p[1:3] //want "dereferenced"
}

func testArrayPtrIndexRead() int {
	p := nilArrayPtr()
	return p[0] //want "dereferenced"
}

func testArrayPtrIndexWrite() {
	p := nilArrayPtr()
	p[0] = 1 //want "dereferenced"
}

func testArrayPtrRangeSecondVar() int {
	p := nilArrayPtr()
	sum := 0
	for _, v := range p { //want "dereferenced"
		sum += v
	}
	return sum
}

// Named pointer-to-array types behave just like *[4]int (their core type is what gets sliced).
type namedArrayPtr *[4]int

func nilNamedArrayPtr() namedArrayPtr {
	if arrayPtrDummy {
		return nil
	}
	return &[4]int{}
}

func testNamedArrayPtrSlice() []int {
	p := nilNamedArrayPtr()
	return p[:] //want "dereferenced"
}

func testArrayPtrSafeUses() int {
	p := nilArrayPtr()
	n := len(p) // len of a pointer to an array is a constant; no dereference happens
	for i := range p {
		n += i
	}
	for i, _ := range p { // a blank second variable is equivalent to the one-variable form
		n += i
	}
	for range p {
		n++
	}
	return n
}

func testArrayPtrNilChecked() int {
	p := nilArrayPtr()
	if p == nil {
		return 0
	}
	for _, v := range p {
		_ = v
	}
	p[0] = 1
	return p[0] + len(p[:]) + len(p[1:3])
}
