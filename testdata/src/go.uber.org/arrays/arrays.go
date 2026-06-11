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
This package aims to test nilability behavior surrounding arrays

<nilaway no inference>
*/
package arrays

import "math/rand"

var globalArr [2]*int

// nonnil(twodArr[])
var twodArr [5][5]*int

func testArrayRet() [2]*int {
	return globalArr
}

// nilable(v)
// nonnil(a[])
func testParamArrayWrite(a [4]*int, v *int, b bool) (*int, *int) {
	if b {
		a[0] = v //want "assigned deeply into parameter arg `a`"
	}
	return a[0], a[1] //want "returned from `testParamArrayWrite.*` in position 0"
}

func testGlobalArrayWrite(v *int, b bool) *int {
	if b {
		globalArr[0] = v
	}
	return globalArr[0] //want "returned"
}

func testLocalArrayWrite() *int {
	var a [4]*int
	a[0] = globalArr[0]
	return a[0] //want "returned"
}

// nilable(v)
func testParamNilableArrayWrite(a [4]*int, v *int, b bool) (*int, *int) {
	if b {
		a[0] = v
	}
	i := 0
	a[1] = &i
	return a[0], a[1] //want "deep read from parameter `a`" "returned from `testParamNilableArrayWrite.*` in position 0"
}

// nonnil(a[])
func testArrayWriteNil(a [4]*int) *int {
	a[0] = nil  //want "assigned deeply into parameter arg `a`"
	return a[0] //want "returned"
}

func testArrayWriteInit(a [2]int) *int {
	a = [2]int{1, 2}
	return &a[1]
}

func testGlobals(i int) *int {
	switch i {
	case 1:
		return globalArr[0] //want "returned"
	case 3:
		return twodArr[0][0]
	case 4:
		local := twodArr[0]
		return local[0]
	}
	return &i
}

func dummyBool() bool {
	if rand.Int() < 50 {
		return false
	}
	return true
}

func testArrayCopy(a [2]*int) *int {
	var b [2]*int
	b = a
	return b[1] //want "returned"
}

// nonnil(i[])
type t struct {
	i [2]*int
}

// nonnil(a[])
func testArrayMultiLevelAssign(a [2]*t) {
	var x *int
	a[0].i[0] = x //want "assigned deeply into field `i`"
}

func testEmptyArrayReturn(a [0]*int) [0]*int {
	return a
}

func testPrimitiveArray(a [3]int, i int) int {
	switch i {
	case 0:
		return a[0]
	case 1:
		return 1 + a[1]
	case 2:
		a[2] = a[0] + 3
		return a[2]
	}
	return 0
}

func test2dArrayAssignment() *int {
	var nilableTwodArr [5][5]*int
	nilableTwodArr[0][0] = nil
	twodArr = nilableTwodArr // TODO: an error should be reported here since we are assigning a (default) deeply nilable array 'nilableTwodArr' into a declared deeply nonnil array 'twodArr'
	return twodArr[0][0]     //want "returned"
}

// Test a case where we declare a type alias for an array and then range over it.

type blocks [42]int

type blockSlice []int

// nonnil(aPtr, a, bPtr, b)
func testArrayAliasPtr(aPtr *blocks, a blocks, bPtr *blockSlice, b blockSlice) {
	// blocks is an alias for arrays, and the [language specs] states that it is possible to range
	// over an array or a pointer to an array. Interestingly, you cannot range over a pointer to
	// a slice.
	// [language specs]: https://go.dev/ref/spec#RangeClause

	// Range over a pointer to array alias. OK
	for range aPtr {
	}

	// Range over an array alias. OK
	for range a {
	}

	// Range over a pointer to slice alias. Disallowed!
	// for range bPtr {}

	// Range over a slice alias. OK
	for range b {
	}
}

// testSlicingArrayProducesNonnilSlice verifies that slicing an array always yields a nonnil slice,
// since arrays are value types that can never be nil; the resulting slice is backed by the array's
// storage. Indexing or re-slicing the result must therefore not be flagged as a potential nil
// panic, regardless of the slicing indices or how the array is obtained. See issue #104.
func testSlicingArrayProducesNonnilSlice() {
	var a [4]int

	// `a[:0]` is an empty but nonnil slice (backed by the array), so consuming it is safe.
	b := a[:0]
	_ = b[0]
	_ = b[1:2]

	// Every other slicing form of the array is nonnil too, including those (like `[1:2]`) that for
	// a regular slice would require the operand to be nonnil.
	_ = a[:][0]
	_ = a[0:][0]
	_ = a[1:2][0]
	_ = a[:0:0][0]

	// Slicing a pointer to an array is nonnil as well.
	p := &a
	_ = p[:0][0]
	_ = p[:][0]

	// Named array types (e.g., `blocks` is `[42]int`) behave the same.
	var n blocks
	_ = n[:0][0]

	// Arrays produced by `new` are covered too: `new([N]T)` returns a nonnil `*[N]T`, so both the
	// explicit-deref form `(*new([N]T))[:0]` (operand type `[N]T`) and the implicit-deref form
	// `new([N]T)[:0]` (operand type `*[N]T`, auto-dereferenced) slice an array.
	_ = (*new([64]byte))[:0][0]
	_ = new([64]byte)[:0][0]

	// Contrast: `make([]T, 0, N)` is nonnil as well, but for the unrelated reason that `make` never
	// returns nil (not because of array slicing).
	_ = make([]byte, 0, 64)[0]
}

// testSlicingArrayIssue104 reproduces the original false positive from
// https://github.com/uber-go/nilaway/issues/104, where slicing a stack-allocated array (`buf[:0]`)
// was wrongly treated as nilable, causing spurious nil-panic reports on the later index operations.
func testSlicingArrayIssue104(maxlen int) []byte {
	var buf [64]byte
	answer := buf[:0]

	for i := 0; i < maxlen; i++ {
		answer = append(answer, byte(i))
	}

	alen := len(answer)
	for i := 0; i < alen/2; i++ {
		answer[i], answer[alen-1-i] = answer[alen-1-i], answer[i]
	}

	return answer
}
