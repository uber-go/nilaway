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
This package aims to test nilability behavior surrounding slices

<nilaway no inference>
*/
package slices

var aBool = true

var nilableSl []int

// nonnil(nonNilSl)
var nonNilSl []int = []int{1, 2}

var nilablenilableSl [][]int

// nonnil(nonNilnilableSl)
var nonNilnilableSl [][]int = make([][]int, 0)

// nonnil(nilablenonNilSl[])
var nilablenonNilSl [][]int

// nonnil(nonNilnonNilSl)
// nonnil(nonNilnonNilSl[])
var nonNilnonNilSl [][]int = [][]int{{1, 2}}

func testGlobals() int {
	switch 0 {
	case 1:
		return nilableSl[0] //want "sliced into"
	case 2:
		return nonNilSl[0]
	case 3:
		return nilablenilableSl[0][0] //want "sliced into" "sliced into"
	case 4:
		local := nilablenilableSl[0] //want "sliced into"
		return local[0]              //want "sliced into"
	case 5:
		return nonNilnilableSl[0][0] //want "sliced into"
	case 6:
		local := nonNilnilableSl[0]
		return local[0] //want "sliced into"
	case 7:
		return nilablenonNilSl[0][0] //want "sliced into"
	case 8:
		local := nilablenonNilSl[0] //want "sliced into"
		return local[0]
	case 9:
		return nonNilnonNilSl[0][0]
	case 10:
		local := nonNilnonNilSl[0]
		return local[0]
	}
	return 0
}

type simpleWrap []int

// nonnil(nonNilWrap)
func testSimpleWrap(nilableWrap, nonNilWrap simpleWrap) int {
	if aBool {
		return nilableWrap[0] //want "sliced into"
	} else {
		return nonNilWrap[0]
	}
}

type wrappednilableSl [][]int

// nonnil(wrappednonNilSl[])
type wrappednonNilSl [][]int

// nonnil(nonNilSl, nonNilnilableSl, nonNilnonNilSl)
func testTypedParams(
	nilableSl, nonNilSl []int,
	nilablenilableSl wrappednilableSl,
	nonNilnilableSl wrappednilableSl,
	nilablenonNilSl wrappednonNilSl,
	nonNilnonNilSl wrappednonNilSl,
) int {
	switch 0 {
	case 1:
		return nilableSl[0] //want "sliced into"
	case 2:
		return nonNilSl[0]
	case 3:
		return nilablenilableSl[0][0] //want "sliced into" "sliced into"
	case 4:
		local := nilablenilableSl[0] //want "sliced into"
		return local[0]              //want "sliced into"
	case 5:
		return nonNilnilableSl[0][0] //want "sliced into"
	case 6:
		local := nonNilnilableSl[0]
		return local[0] //want "sliced into"
	case 7:
		return nilablenonNilSl[0][0] //want "sliced into"
	case 8:
		local := nilablenonNilSl[0] //want "sliced into"
		return local[0]
	case 9:
		return nonNilnonNilSl[0][0]
	case 10:
		local := nonNilnonNilSl[0]
		return local[0]
	}
	return 0
}

func lengthCheckAsNilCheckTest(a []int) int {
	switch 0 {
	case 1:
		return a[0] //want "sliced into"
	case 2:
		if a != nil {
			return a[0]
		}
	case 3:
		if a == nil {
			return a[0] //want "sliced into"
		}
	case 4:
		if len(a) != 0 {
			return a[0]
		}
	case 5:
		if len(a) == 0 {
			return a[0] //want "sliced into"
		}
	case 6:
		if len(a) > 0 {
			return a[0]
		}
	case 7:
		if len(a) >= 0 {
			return a[0] //want "sliced into"
		}
	case 8:
		if len(a) <= 0 {
			return a[0] //want "sliced into"
		}
	case 9:
		if len(a) < 0 {
			// this can never occur - so just treated as a no-op
			return a[0] //want "sliced into"
		}
	case 10:
		if len(a) != 1 {
			return a[0] //want "sliced into"
		}
	case 11:
		if len(a) == 1 {
			return a[0]
		}
	case 12:
		if len(a) > 1 {
			return a[0]
		}
	case 13:
		if len(a) >= 1 {
			return a[0]
		}
	case 14:
		if len(a) <= 1 {
			return a[0] //want "sliced into"
		}
	case 15:
		if len(a) < 1 {
			return a[0] //want "sliced into"
		}

		// the following cases are the same as above - but flipped

	case 16:
		if nil != a {
			return a[0]
		}
	case 17:
		if nil == a {
			return a[0] //want "sliced into"
		}
	case 18:
		if 0 != len(a) {
			return a[0]
		}
	case 19:
		if 0 == len(a) {
			return a[0] //want "sliced into"
		}
	case 20:
		if 0 < len(a) {
			return a[0]
		}
	case 21:
		if 0 <= len(a) {
			return a[0] //want "sliced into"
		}
	case 22:
		if 0 >= len(a) {
			return a[0] //want "sliced into"
		}
	case 23:
		if 0 > len(a) {
			// this can never occur - so just treated as a no-op
			return a[0] //want "sliced into"
		}
	case 24:
		if 1 != len(a) {
			return a[0] //want "sliced into"
		}
	case 25:
		if 1 == len(a) {
			return a[0]
		}
	case 26:
		if 1 < len(a) {
			return a[0]
		}
	case 27:
		if 1 <= len(a) {
			return a[0]
		}
	case 28:
		if 1 >= len(a) {
			return a[0] //want "sliced into"
		}
	case 29:
		if 1 > len(a) {
			return a[0] //want "sliced into"
		}
	case 30:
		for i := 0; i < len(a); i++ {
			_ = a[i]
		}
	case 31:
		for i := 0; i < len(a)-1; i++ {
			_ = a[i]
		}
	case 32:
		var b int
		for i := 0; i < len(a) + 2 + b; i ++ {
			_ = a[i]
		}
	case 33:
		var b int
		for i := 0; i + 1 < len(a) + 2 + b; i ++ {
			_ = a[i]
		}
	case 34:
		var b int
		for i := 0; i <= len(a) + 2 + b; i ++ {
			_ = a[i]
		}
	}
	return 0
}

func lengthCheckByIntExprTest(a []int, i int) int {
	var j int
	k := 7
	switch 0 {
	case 1:
		return a[0] //want "sliced into"
	case 2:
		if len(a) > i {
			return a[0]
		}
	case 3:
		if len(a) > j {
			return a[0]
		}
	case 4:
		if len(a) > k {
			return a[0]
		}
	case 5:
		if len(a) >= i {
			return a[0]
		}
	case 6:
		if len(a) >= j {
			return a[0]
		}
	case 7:
		if len(a) >= k {
			return a[0]
		}
	case 8:
		if len(a) < i {
			return a[0] //want "sliced into"
		}
	case 9:
		if len(a) < j {
			return a[0] //want "sliced into"
		}
	case 10:
		if len(a) < k {
			return a[0] //want "sliced into"
		}
	case 11:
		if len(a) <= i {
			return a[0] //want "sliced into"
		}
	case 12:
		if len(a) <= j {
			return a[0] //want "sliced into"
		}
	case 13:
		if len(a) <= k {
			return a[0] //want "sliced into"
		}
	case 14:
		// these cases test that non-literal integers are treated optimistically
		if len(a) == i {
			return a[0]
		}
	case 15:
		if len(a) == j {
			return a[0]
		}
	case 16:
		if len(a) == k {
			return a[0]
		}
	case 17:
		if len(a) != i {
			return a[0] //want "sliced into"
		}
	case 18:
		if len(a) != j {
			return a[0] //want "sliced into"
		}
	case 19:
		if len(a) != k {
			return a[0] //want "sliced into"
		}
	case 20:
		var b int
		if len(a) + 2 + b > j {
			return a[0]
		}
	}
	return 0
}

func dummyConsume(interface{}) {}
func dummyBool() bool          { return true }

// this function tests whether we properly interpret double len equality checks
// as producing non-nil - this is technically unsound, but used so often in practice
// that we support it
func testDoubleLenCheck(a, b, c, d []int) int {
	switch 0 {
	case 1:
		if dummyBool() {
			return a[0] //want "sliced into"
		}
		if len(a) == len(b) {
			return a[0]
		}
	case 2:
		if dummyBool() {
			return b[0] //want "sliced into"
		}
		if len(a) == len(b) {
			return b[0]
		}
	case 3:
		if len(a) != len(b) {
			return 0
		}
		return a[0]
	case 4:
		if len(a) != len(b) {
			return 0
		}
		return b[0]
	case 5:
		// We will optimistically assume all slices are non-nil.
		if len(a) - len(c) == len(b) * len(d) {
			_, _, _, _ = a[0], b[0], c[0], d[0]
		}
	}
	return 0
}

func testSwitchAsLenCheck(a []int) int {
	var i int
	switch len(a) {
	case -1:
		return a[0] //want "sliced into"
	case 0:
		return a[0] //want "sliced into"
	case 1:
		return a[0]
	case 39845978:
		return a[0]
	case i:
		return a[0]
	}
	return 0
}

func testSlicingDoesNotCreateConsumersForNilableSlice() []int {
	var nilA, b []int
	const zero = 0
	switch 0 {
	case 1:
		// [:0]
		b = nilA[:0]
		b = nilA[:1-1]
		b = nilA[:zero]
		b = nilA[:zero+1-1]
	case 2:
		// [0:0]
		b = nilA[0:0]
		b = nilA[1-1 : 0-0]
		b = nilA[zero:zero]
		b = nilA[zero+1-1 : zero+0-0]
	case 3:
		// [0:]
		b = nilA[0:]
		b = nilA[1-1:]
		b = nilA[zero:]
		b = nilA[zero+0-0:]
	case 4:
		// [:]
		b = nilA[:]
	case 5:
		// [0:0:0]
		b = nilA[0:0:0]
		b = nilA[1-1 : 1-1 : 0-0]
		b = nilA[zero:zero:zero]
		b = nilA[zero+1-1 : zero+1-1 : zero+1-1]
	case 6:
		// [:0:0]
		b = nilA[:0:0]
		b = nilA[: 1-1 : 0-0]
		b = nilA[:zero:zero]
		b = nilA[: zero+1-1 : zero+1-1]
	}
	return b
}

func testOtherSlicingCreatesConsumerForNilableSlice() []int {
	var nilA, b []int
	l, m, n := 1, 1, 1
	const zero = 0
	switch 0 {
	case 1:
		// [:n]
		b = nilA[:n] //want "sliced into"
	case 2:
		// [n:0]
		b = nilA[n:0]          //want "sliced into"
		b = nilA[n : 1-1]      //want "sliced into"
		b = nilA[n:zero]       //want "sliced into"
		b = nilA[n : zero+1-1] //want "sliced into"
	case 3:
		// [0:n]
		b = nilA[0:n]          //want "sliced into"
		b = nilA[1-1 : n]      //want "sliced into"
		b = nilA[zero:n]       //want "sliced into"
		b = nilA[zero+1-1 : n] //want "sliced into"
	case 4:
		// [m:n]
		b = nilA[m:n] //want "sliced into"
	case 5:
		// [0:0:n]
		b = nilA[0:0:n]                   //want "sliced into"
		b = nilA[1-1 : 1-1 : n]           //want "sliced into"
		b = nilA[zero:zero:n]             //want "sliced into"
		b = nilA[zero+1-1 : zero+1-1 : n] //want "sliced into"
	case 6:
		// [0:n:0]
		b = nilA[0:n:0]                   //want "sliced into"
		b = nilA[1-1 : n : 1-1]           //want "sliced into"
		b = nilA[zero:n:zero]             //want "sliced into"
		b = nilA[zero+1-1 : n : zero+1-1] //want "sliced into"
	case 7:
		// [n:0:0]
		b = nilA[n:0:0]                   //want "sliced into"
		b = nilA[n : 1-1 : 1-1]           //want "sliced into"
		b = nilA[n:zero:zero]             //want "sliced into"
		b = nilA[n : zero+1-1 : zero+1-1] //want "sliced into"
	case 8:
		// [0:m:n]
		b = nilA[0:m:n]            //want "sliced into"
		b = nilA[1-1 : m : n]      //want "sliced into"
		b = nilA[zero:m:n]         //want "sliced into"
		b = nilA[zero+1-1 : m : n] //want "sliced into"
	case 9:
		// [m:0:n]
		b = nilA[m:0:n]            //want "sliced into"
		b = nilA[m : 1-1 : n]      //want "sliced into"
		b = nilA[m:zero:n]         //want "sliced into"
		b = nilA[m : zero+1-1 : n] //want "sliced into"
	case 10:
		// [m:n:0]
		b = nilA[m:n:0]            //want "sliced into"
		b = nilA[m : n : 1-1]      //want "sliced into"
		b = nilA[m:n:zero]         //want "sliced into"
		b = nilA[m : n : zero+1-1] //want "sliced into"
	case 11:
		// [l:m:n]
		b = nilA[l:m:n] //want "sliced into"
	}
	return b
}

func testCertainSlicingCreatesNilProducerForAnySlice() {
	var nilA, nonNilA []int = nil, []int{1}
	m, n := 1, 1
	switch 0 {
	case 1:
		// [:0]
		b := nilA[:0]
		print(b[0]) //want "sliced into"
		c := nonNilA[:0]
		print(c[0]) //want "sliced into"
		// We could test const zero or binary expressions that evaluates to zero as well but I feel
		// the related util function *RootAssertionNode#isZero is well tested in the previous two
		// tests so we don't test again here (and following tests).
	case 2:
		// [0:0]
		b := nilA[0:0]
		print(b[0]) //want "sliced into"
		c := nonNilA[0:0]
		print(c[0]) //want "sliced into"
	case 3:
		// [n:0]
		b := nilA[n:0] //want "sliced into"
		print(b[0])    //want "sliced into"
		c := nonNilA[n:0]
		print(c[0]) //want "sliced into"
	case 4:
		// [0:0:0]
		b := nilA[0:0:0]
		print(b[0]) //want "sliced into"
		c := nonNilA[0:0:0]
		print(c[0]) //want "sliced into"
	case 5:
		// [0:0:n]
		b := nilA[0:0:n] //want "sliced into"
		print(b[0])      //want "sliced into"
		c := nonNilA[0:0:n]
		print(c[0]) //want "sliced into"
	case 6:
		// [n:0:0]
		b := nilA[n:0:0] //want "sliced into"
		print(b[0])      //want "sliced into"
		c := nonNilA[n:0:0]
		print(c[0]) //want "sliced into"
	case 7:
		// [m:0:n]
		b := nilA[m:0:n] //want "sliced into"
		print(b[0])      //want "sliced into"
		c := nonNilA[m:0:n]
		print(c[0]) //want "sliced into"
	}
}

func testCertainSlicingPreserveNilabilityOfOriginalSlice() {
	var nilA, nonNilA []int = nil, []int{1}
	switch 0 {
	case 1:
		// [0:]
		b := nilA[0:]
		print(b[0]) //want "sliced into"
		c := nonNilA[0:]
		print(c[0])
	case 2:
		// [:]
		b := nilA[:]
		print(b[0]) //want "sliced into"
		c := nonNilA[:]
		print(c[0])
	}
}

func testOtherSlicingCreatesNonNilProducerForAnySlice() {
	var nilA, nonNilA []int = nil, []int{1}
	l, m, n := 1, 1, 1
	switch 0 {
	case 1:
		// [:n]
		b := nilA[:n] //want "sliced into"
		print(b[0])
		c := nonNilA[:n]
		print(c[0])
	case 2:
		// [0:n]
		b := nilA[0:n] //want "sliced into"
		print(b[0])
		c := nonNilA[0:n]
		print(c[0])
	case 3:
		// [m:n]
		b := nilA[m:n] //want "sliced into"
		print(b[0])
		c := nonNilA[m:n]
		print(c[0])
	case 4:
		// [0:n:0]
		b := nilA[0:n:0] //want "sliced into"
		print(b[0])
		c := nonNilA[0:n:0]
		print(c[0])
	case 5:
		// [0:m:n]
		b := nilA[0:m:n] //want "sliced into"
		print(b[0])
		c := nonNilA[0:m:n]
		print(c[0])
	case 6:
		// [m:n:0]
		b := nilA[m:n:0] //want "sliced into"
		print(b[0])
		c := nonNilA[m:n:0]
		print(c[0])
	case 7:
		// [l:m:n]
		b := nilA[l:m:n] //want "sliced into"
		print(b[0])
		c := nonNilA[l:m:n]
		print(c[0])
	}
}

func testOtherInterestingCasesOnZeroSlicing() {
	var twoDSlice [][]int
	switch 0 {
	case 1:
		// cascaded slice expressions
		c := twoDSlice[:0]
		d := c[:]
		e := d[0:0]
		f := e[0:]
		print(f[0]) //want "sliced into"
	case 2:
		// nested slice expressions
		c := twoDSlice[:0][:0]
		print(c[0]) //want "sliced into"

		c = twoDSlice[:0][0:]
		print(c[0]) //want "sliced into"

		c = twoDSlice[:0][1:3] //want "sliced into"
		print(c[1])

		c = twoDSlice[0:][:0]
		print(c[1]) //want "sliced into"

		c = twoDSlice[0:][0:]
		print(c[1]) //want "sliced into"

		c = twoDSlice[:0][1:3] //want "sliced into"
		print(c[1])

		c = twoDSlice[1:3][:0] //want "sliced into"
		print(c[1])            //want "sliced into"

		c = twoDSlice[1:3][0:] //want "sliced into"
		print(c[1])

		c = twoDSlice[1:3][1:3] //want "sliced into"
		print(c[1])
	}
}

func testInterProcedural() {
	var nilA, nonNilA []int = nil, []int{1}
	switch 0 {
	case 1:
		// use slicing as parameter

		// We make several copies of the same function as we want to make sure nilaway reports an
		// error for each call.

		// zero slicing triggering nil producer all the time, e.g., [:0]
		helperSliceParamForNilableParam1(nilA[:0])
		helperSliceParamForNilableParam2(nonNilA[:0])

		// zero slicing that preserves nilability of original slice, e.g., [0:]
		helperSliceParamForNilableParam3(nilA[0:])
		helperSliceParamForNonNilParam(nonNilA[0:])

		// non-zero slicing triggering non-nil producer all the time, e.g, [1:3]
		helperSliceParamForNonNilParam(nilA[1:3]) //want "sliced into"
		helperSliceParamForNonNilParam(nonNilA[1:3])
	case 2:
		// use slicing as return result

		// zero slicing triggering nil producer all the time, e.g., [:0]
		b := helperReturnZeroSlicingAlwaysNilProducerForNilableParam(nilA)
		print(b[1]) //want "sliced into"
		c := helperReturnZeroSlicingAlwaysNilProducerForNonNilParam(nonNilA)
		print(c[1]) //want "sliced into"

		// zero slicing that preserves nilability of original slice, e.g., [0:]
		b = helperReturnZeroSlicingPreserveForNilableParam(nilA)
		print(b[1]) //want "sliced into"
		c = helperReturnZeroSlicingPreserveForNonNilParam(nonNilA)
		print(c[1])

		// non-zero slicing triggering non-nil producer all the time, e.g, [1:3]
		b = helperReturnNonZeroSlicingNonNilProducerForNilableParam(nilA)
		print(b[1])
		c = helperReturnNonZeroSlicingNonNilProducerForNonNilParam(nonNilA)
		print(c[1])
	}
}

func helperSliceParamForNilableParam1(b []int) {
	print(b[0]) //want "sliced into"
}

func helperSliceParamForNilableParam2(b []int) {
	print(b[0]) //want "sliced into"
}

func helperSliceParamForNilableParam3(b []int) {
	print(b[0]) //want "sliced into"
}

// Must annotate because the default nilability of a slice type is nilable; see
// `annotation.TypeIsDefaultNilable`. Same for a few helper functions following.
// nonnil(b)
func helperSliceParamForNonNilParam(b []int) {
	print(b[0])
}

func helperReturnZeroSlicingAlwaysNilProducerForNilableParam(b []int) []int {
	return b[:0]
}

// nonnil(b)
func helperReturnZeroSlicingAlwaysNilProducerForNonNilParam(b []int) []int {
	return b[:0]
}

func helperReturnZeroSlicingPreserveForNilableParam(b []int) []int {
	return b[0:]
}

// nonnil(b, result 0)
func helperReturnZeroSlicingPreserveForNonNilParam(b []int) []int {
	return b[0:]
}

// nonnil(result 0)
func helperReturnNonZeroSlicingNonNilProducerForNilableParam(b []int) []int {
	return b[1:3] //want "sliced into"
}

// nonnil(b, result 0)
func helperReturnNonZeroSlicingNonNilProducerForNonNilParam(b []int) []int {
	return b[1:3]
}

// TODO: Uncomment this test after we finish
/* func testSlicingTrackSliceExprAsWhole(c int) {
	var b []int
	if b[:] != nil { // direct pass the trackable expression can only handle if b != nil
		a := b[:]
		c = a[1] // OK!
	}
} */

// nonnil(a, a[])
func testAppendNil(a []*int) {
	a[0] = nil //want "assigned deeply into parameter arg `a`"
	// Now, we append a literal nil into a deeply nonnil slice.
	a = append(a, nil) //want "assigned deeply into parameter arg `a`"
}

// nonnil(a, a[], b)
// nilable(c, result 0)
func testAppend(a []*int, b, c *int) {
	b = c
	a = append(a, b) //want "assigned deeply into parameter arg `a`"
	a = append(a, c) //want "assigned deeply into parameter arg `a`"
}

// nilable(result 0)
func nilableFun() *int {
	return nil
}

// nonnil(a, a[], b)
func testAppendNilableFunc(a []*int) {
	a[0] = nilableFun()         //want "assigned deeply into parameter arg `a`"
	a = append(a, nilableFun()) //want "assigned deeply into parameter arg `a`"
}

// nonnil(a, a[])
// nilable(b, b[])
func testTheFirstArgumentOfAppend(a, b []*int) {
	t := 1
	a = append(b, &t) // TODO: this will be handled once we fix
	print(*a[0])
}

// nonnil(a, a[])
// nilable(b, b[])
func testVariadicArgs(a, b []*int) {
	a = append(a, b...) //want "assigned deeply into parameter arg `a`"
	b = append(b, a...)
}

// nonnil(a, a[], nonnilvar)
// nilable(nilablevar)
func testMultipleAppendArgs(a []*int, nilablevar, nonnilvar *int) {
	a = append(a, nonnilvar, nilablevar, nil) // TODO: this will be handled once we fix
}

func testAppendNilableForLocalVar() {
	var a = make([]*int, 0)
	a = append(a, nil)
	print(*a[0]) //want "literal `nil` sliced into"
}

var a = make([]*int, 0)

func testAppendNilableForGlobalVar() {
	a = append(a, nil) //want "literal `nil` assigned into global variable `a`"
	print(*a[0])       //want "literal `nil` sliced into"
}

func testShadowAppend() {
	// Shadow the builtin append function that returns the same slice without modifications.
	var append = func(s []*int, x ...*int) []*int { return s }
	a = append(a, nil) // Safe here because the shadowed append does not touch the elements.
}
