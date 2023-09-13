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

<nilaway no inference>
*/
package multipleassignment

// nilable(f)
type T struct {
	f *T
}

// each of the following two functions is safe, and nilaway should realize that

// nilable(x)
func swapToSafety1(x *T) *T {
	y := &T{}
	x, y = y, x
	return x
}

// nilable(x)
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
	return x //want "returned"
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
		return x.f.f //want "returned"
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
		return x.f //want "returned"
	}
}

func twoNonNil() (*T, *T) {
	return &T{}, &T{}
}

// nilable(b, c)
func leftNonNil() (a *T, b *T, c *T) {
	return &T{}, nil, nil
}

// nilable(a, c)
func centerNonNil() (a *T, b *T, c *T) {
	return nil, &T{}, nil
}

// nilable(a, b)
func rightNonNil() (a *T, b *T, c *T) {
	return nil, nil, &T{}
}

// nilable(b)
func testThreeRets() (a *T, b *T, c *T) {
	switch 0 {
	case 1:
		return leftNonNil() //want "returned from `testThreeRets.*` in position 2"
	case 2:
		return centerNonNil() //want "returned from `testThreeRets.*` in position 2" "returned from `testThreeRets.*` in position 0"
	case 3:
		return rightNonNil() //want "returned from `testThreeRets.*` in position 0"
	case 4:
		return nil, nil, nil //want "returned from `testThreeRets.*` in position 2" "returned from `testThreeRets.*` in position 0"
	default:
		return &T{}, &T{}, &T{}
	}
}

// nilable(b, c)
func takesLeftNonNil(a *T, b *T, c *T) {}

// nilable(a, c)
func takesCenterNonNil(a *T, b *T, c *T) {}

// nilable(a, b)
func takesRightNonNil(a *T, b *T, c *T) {}

// multiple returners can be passed directly to multiple param funcs - test that here
func testMultiToMultiCalls() {
	takesLeftNonNil(leftNonNil())
	takesLeftNonNil(centerNonNil()) //want "passed as arg `a`"
	takesLeftNonNil(rightNonNil())  //want "passed as arg `a`"
	takesCenterNonNil(leftNonNil()) //want "passed as arg `b`"
	takesCenterNonNil(centerNonNil())
	takesCenterNonNil(rightNonNil()) //want "passed as arg `b`"
	takesRightNonNil(leftNonNil())   //want "passed as arg `c`"
	takesRightNonNil(centerNonNil()) //want "passed as arg `c`"
	takesRightNonNil(rightNonNil())
}

// nilable(first)
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

func assignThreeNonNil(tt *twoTs) {
	tt.second, tt.second, tt.second = rightNonNil()  //want "assigned into field" "assigned into field"
	tt.second, tt.second, tt.second = centerNonNil() //want "assigned into field" "assigned into field"
	tt.second, tt.second, tt.second = leftNonNil()   //want "assigned into field" "assigned into field"
	tt.first, tt.first, tt.second = rightNonNil()
	tt.first, tt.second, tt.first = centerNonNil()
	tt.second, tt.first, tt.first = leftNonNil()
}

func oneTrueNonNil() *T {
	var a, b, c *T
	switch 0 {
	case 1:
		a, b, c = rightNonNil()
	case 2:
		b, c, a = centerNonNil()
	default:
		c, a, b = leftNonNil()
	}
	switch 0 {
	case 1:
		return a //want "returned" "returned" "returned"
	case 2:
		return b //want "returned" "returned" "returned"
	default:
		return c
	}
}
