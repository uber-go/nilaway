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

package inference

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
		helperSliceParamForNonNilParam(nilA[1:3]) //want "literal `nil`"
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

func helperSliceParamForNonNilParam(b []int) {
	print(b[0])
}

func helperReturnZeroSlicingAlwaysNilProducerForNilableParam(b []int) []int {
	return b[:0]
}

func helperReturnZeroSlicingAlwaysNilProducerForNonNilParam(b []int) []int {
	return b[:0]
}

func helperReturnZeroSlicingPreserveForNilableParam(b []int) []int {
	return b[0:]
}

func helperReturnZeroSlicingPreserveForNonNilParam(b []int) []int {
	return b[0:]
}

func helperReturnNonZeroSlicingNonNilProducerForNilableParam(b []int) []int {
	return b[1:3] //want "sliced into"
}

func helperReturnNonZeroSlicingNonNilProducerForNonNilParam(b []int) []int {
	return b[1:3]
}
