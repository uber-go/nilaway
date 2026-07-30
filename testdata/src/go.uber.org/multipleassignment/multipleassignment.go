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
	print(*x) //want "dereferenced"
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
		print(*x.f.f) //want "dereferenced"
		return x.f.f
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
		print(*x.f) //want "dereferenced"
		return x.f
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
		a, b, c = leftNonNil()
		print(*c) //want "dereferenced"
		return
	case 2:
		a, b, c = centerNonNil()
		print(*c) //want "dereferenced"
		print(*a) //want "dereferenced"
		return
	case 3:
		a, b, c = rightNonNil()
		print(*a) //want "dereferenced"
		return
	case 4:
		a, b, c = nil, nil, nil
		print(*c) //want "dereferenced"
		print(*a) //want "dereferenced"
		return
	default:
		return &T{}, &T{}, &T{}
	}
}

// nilable(b, c)
func takesLeftNonNil(a *T, b *T, c *T) {
	print(*a) //want "dereferenced" "dereferenced"
}

// nilable(a, c)
func takesCenterNonNil(a *T, b *T, c *T) {
	print(*b) //want "dereferenced" "dereferenced"
}

// nilable(a, b)
func takesRightNonNil(a *T, b *T, c *T) {
	print(*c) //want "dereferenced" "dereferenced"
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
	{
		a, b, c := rightNonNil()
		print(*a) //want "dereferenced"
		print(*b) //want "dereferenced"
		tt.second, tt.second, tt.second = a, b, c
	}
	{
		a, b, c := centerNonNil()
		print(*a) //want "dereferenced"
		print(*c) //want "dereferenced"
		tt.second, tt.second, tt.second = a, b, c
	}
	{
		a, b, c := leftNonNil()
		print(*b) //want "dereferenced"
		print(*c) //want "dereferenced"
		tt.second, tt.second, tt.second = a, b, c
	}
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
		print(*a) //want "dereferenced" "dereferenced" "dereferenced"
		return a
	case 2:
		print(*b) //want "dereferenced" "dereferenced" "dereferenced"
		return b
	default:
		return c
	}
}
