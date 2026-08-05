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
These tests aim to address two related features of the Go language: multiple assignments and
multiply-returning functions. Multiple assignments such as x, y, z = a, b, c have subtle semantics
regarding ordering and shadowing which are tested below. Multiply-returning functions must track
nilability for each return separately, at the sites of assignments, calls, and returns, which are
all tested below.
*/
package multipleassignment

type T struct {
	f *T
}

// each of the following two functions is safe, and nilaway should realize that

func swapToSafety1(x *T) *T {
	y := &T{}
	x, y = y, x
	return x
}

func swapToSafety2(x *T) *T {
	y := &T{}
	y, x = x, y
	return x
}

func swapField1(x *T) *T {
	x.f = &T{}
	y := &T{}

	// Replaces ([old] y).f with nil, and ([new] y) with x
	// So, after this line, y.f = x.f = &T{} and the nil assigned field is unreachable
	y.f, y = nil, x

	return y.f
}

func swapField2(x *T) *T {
	x.f = &T{}
	y := &T{}
	y, y.f = x, nil

	return y.f
}

func unsafeRedundantSwap(x *T) *T {
	x, x = x, nil
	return x
}

func safeRedundantSwap(x *T) *T {
	x, x = nil, x
	return x
}

func slightlyDeeperSwap(x *T) *T {
	x.f = &T{}
	x.f.f = &T{}
	x.f.f, x.f, x.f.f = nil, x.f.f, nil
	switch 0 {
	case 1:
		return x
	case 2:
		return x.f
	default:
		return x.f.f // (error is reported at the deref of the result)
	}
}

func slightlyDeeperSwap2(x *T) *T {
	x.f = &T{}
	x.f.f = &T{}
	x.f.f, x.f, x.f.f, x.f = nil, x.f.f, nil, x.f.f.f
	switch 0 {
	case 1:
		return x
	default:
		return x.f // (error is reported at the deref of the result)
	}
}

// testSwaps derefs the results of the swap functions above. The swaps returning provably non-nil
// values remain safe; unsafeRedundantSwap's nil return is reported at the deref here.
// NOTE: the nil flows through swapped *fields* in slightlyDeeperSwap and slightlyDeeperSwap2
// (originally errors under explicit annotations) are not tracked under full inference (field
// assignment conflicts are suppressed without the experimental struct-init support), so their
// derefs below are (unsoundly) considered safe.
func testSwaps() {
	_ = *swapToSafety1(nil)
	_ = *swapToSafety2(nil)
	_ = *swapField1(&T{})
	_ = *swapField2(&T{})
	_ = *unsafeRedundantSwap(&T{}) //want "returned from `unsafeRedundantSwap.*` in position 0"
	_ = *safeRedundantSwap(&T{})
	_ = *slightlyDeeperSwap(&T{})
	_ = *slightlyDeeperSwap2(&T{})
}

func twoNonNil() (*T, *T) {
	return &T{}, &T{}
}

func leftNonNil() (a *T, b *T, c *T) {
	return &T{}, nil, nil
}

func centerNonNil() (a *T, b *T, c *T) {
	return nil, &T{}, nil
}

func rightNonNil() (a *T, b *T, c *T) {
	return nil, nil, &T{}
}

// testTestThreeRets derefs the first and third results of testThreeRets, so every return path of
// testThreeRets yielding nil in position 0 or 2 is reported here (one diagnostic per unsafe
// path). The second result is not consumed, so its nil paths remain safe.
// NOTE: this caller is intentionally declared BEFORE testThreeRets: inference processes
// declarations in source order, so the derefs here determine the result sites non-nil first, and
// then each unsafe return path of testThreeRets creates its own conflict.
func testTestThreeRets() {
	a, b, c := testThreeRets()
	_ = *a //want "returned from `testThreeRets.*` in position 0" "returned from `testThreeRets.*` in position 0" "returned from `testThreeRets.*` in position 0"
	_ = *c //want "returned from `testThreeRets.*` in position 2" "returned from `testThreeRets.*` in position 2" "returned from `testThreeRets.*` in position 2"
	_ = b
}

func testThreeRets() (a *T, b *T, c *T) {
	switch 0 {
	case 1:
		return leftNonNil()
	case 2:
		return centerNonNil()
	case 3:
		return rightNonNil()
	case 4:
		return nil, nil, nil
	default:
		return &T{}, &T{}, &T{}
	}
}

func takesLeftNonNil(a *T, b *T, c *T) {
	_ = *a //want "dereferenced" "dereferenced"
}

func takesCenterNonNil(a *T, b *T, c *T) {
	_ = *b //want "dereferenced" "dereferenced"
}

func takesRightNonNil(a *T, b *T, c *T) {
	_ = *c //want "dereferenced" "dereferenced"
}

// multiple returners can be passed directly to multiple param funcs - test that here
func testMultiToMultiCalls() {
	takesLeftNonNil(leftNonNil())
	takesLeftNonNil(centerNonNil())
	takesLeftNonNil(rightNonNil())
	takesCenterNonNil(leftNonNil())
	takesCenterNonNil(centerNonNil())
	takesCenterNonNil(rightNonNil())
	takesRightNonNil(leftNonNil())
	takesRightNonNil(centerNonNil())
	takesRightNonNil(rightNonNil())
}

type twoTs struct {
	first  *T
	second *T
}

func returnTwoNonNil() *T {
	a, b := twoNonNil()
	if true {
		return a
	} else {
		return b
	}
}

func testReturnTwoNonNil() {
	_ = *returnTwoNonNil()
}

func assignThreeNonNil(tt *twoTs) {
	// Full inference suppresses field-assignment conflicts, and each repeated assignment leaves
	// tt.second with the final nonnil result. Keep the original assignments, then separately
	// consume the same nilable result positions so both patterns remain covered.
	{
		tt.second, tt.second, tt.second = rightNonNil()
		a, b, c := rightNonNil()
		_ = *a //want "dereferenced"
		_ = *b //want "dereferenced"
		_ = c
	}
	{
		tt.second, tt.second, tt.second = centerNonNil()
		a, b, c := centerNonNil()
		_ = *a //want "dereferenced"
		_ = *c //want "dereferenced"
		_ = b
	}
	{
		tt.second, tt.second, tt.second = leftNonNil()
		a, b, c := leftNonNil()
		_ = *b //want "dereferenced"
		_ = *c //want "dereferenced"
		_ = a
	}
	tt.first, tt.first, tt.second = rightNonNil()
	tt.first, tt.second, tt.first = centerNonNil()
	tt.second, tt.first, tt.first = leftNonNil()
}

// oneTrueNonNil originally returned either `a`, `b` (each nil under all three swapped
// assignments below), or `c` (the one true non-nil variable). It is split into two siblings by
// returned variable - keeping the exact same multiple-assignment block - so that the three
// (nil source, return) flows of each are reported on separate caller lines while every flow is
// still consumed as a caller deref of the function's result.
// NOTE: each caller is intentionally declared BEFORE its producer so that the deref determines
// the result site non-nil first, and then each unsafe flow creates its own conflict (see the
// NOTE on testTestThreeRets).

func testOneTrueNonNilReturnsA() {
	_ = *oneTrueNonNilReturnsA() //want "returned from `oneTrueNonNilReturnsA.*` in position 0" "returned from `oneTrueNonNilReturnsA.*` in position 0" "returned from `oneTrueNonNilReturnsA.*` in position 0"
}

func oneTrueNonNilReturnsA() *T {
	var a, b, c *T
	switch 0 {
	case 1:
		a, b, c = rightNonNil()
	case 2:
		b, c, a = centerNonNil()
	default:
		c, a, b = leftNonNil()
	}
	_ = b
	switch 0 {
	case 1:
		return a // (each of the three nil sources for `a` is reported at the deref of the result)
	default:
		return c
	}
}

func testOneTrueNonNilReturnsB() {
	_ = *oneTrueNonNilReturnsB() //want "returned from `oneTrueNonNilReturnsB.*` in position 0" "returned from `oneTrueNonNilReturnsB.*` in position 0" "returned from `oneTrueNonNilReturnsB.*` in position 0"
}

func oneTrueNonNilReturnsB() *T {
	var a, b, c *T
	switch 0 {
	case 1:
		a, b, c = rightNonNil()
	case 2:
		b, c, a = centerNonNil()
	default:
		c, a, b = leftNonNil()
	}
	_ = a
	switch 0 {
	case 1:
		return b // (each of the three nil sources for `b` is reported at the deref of the result)
	default:
		return c
	}
}
