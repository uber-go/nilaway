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
This test aims to make sure that annotations on functions and structs are parsed correctly.
If any annotation on the struct bar or the function foo is not parsed correctly, then some source
line from the function foo will not throw the expected set of diagnostics
*/
package annotationparse

// nilable(lug)
/*
nilable(jar)
nilable(karp)
nonnil(myr)
*/
type bar struct {
	jar  *bar
	karp *bar
	lug  *bar
	myr  *bar
}

/*
nilable(e)
*/
// nilable(a, c)
// nonnil(d)
/*
nilable(f, h)
nonnil(g)
*/
func foo(a, b *bar, c *bar, d, e *bar) (f, g *bar, h *bar) { //want "returned from `foo.*` in position 1" "returned from `foo.*` in position 1" "returned from `foo.*` in position 1"
	myBar := &bar{}

	myBar.jar = a
	myBar.karp = a
	myBar.lug = a
	myBar.myr = a
	print(*myBar.myr) //want "dereferenced"

	myBar.jar = b
	myBar.karp = b
	myBar.lug = b
	myBar.myr = b

	myBar.jar = c
	myBar.karp = c
	myBar.lug = c
	myBar.myr = c
	print(*myBar.myr) //want "dereferenced"

	myBar.jar = d
	myBar.karp = d
	myBar.lug = d
	myBar.myr = d
	print(*myBar.myr)

	myBar.jar = e
	myBar.karp = e
	myBar.lug = e
	myBar.myr = e
	print(*myBar.myr) //want "dereferenced"

	switch 0 {
	case 1:
		return a, a, a
	case 2:
		return b, b, b
	case 3:
		return c, c, c
	case 4:
		return d, d, d
	default:
		return e, e, e
	}
}

type A struct{}

// the following two functions test that variadic parameters are handled appropriately within their
// function bodies.
// calling these functions in variadicTest tests that variadic parameters are handled appropriately
// as sites for external argument passing

// nilable(e), nonnil(result 0)
func variadicNilable(a, b, c *A, d *A, e ...*A) *A { //want "returned from `variadicNilable.*` in position 0"
	if len(e) > 1 {
		e[1] = nil
		return e[0]
	}
	return a
}

// nonnil(e)
func variadicNonNil(a, b, c *A, d *A, e ...*A) *A { //want "assigned deeply into variadic parameter" "passed as arg `e`" "passed as arg `e`"
	if len(e) > 1 {
		e[1] = nil
		return e[0]
	}
	return a
}

// variadicNonNilLonger is identical to variadicNonNil (minus the in-body deep assignment, which
// only needs to be tested once) and consumes the longer variadic calls, so that the
// annotation-anchored diagnostics of the two functions stay on separate lines.

// nonnil(e)
func variadicNonNilLonger(a, b, c *A, d *A, e ...*A) *A { //want "passed as arg `e`" "passed as arg `e`" "passed as arg `e`"
	if len(e) > 1 {
		return e[0]
	}
	return a
}

func variadicTest() {
	a := &A{}
	variadicNilable(a, a, a, a)
	variadicNilable(a, a, a, a, nil)
	variadicNilable(a, a, a, a, a)
	variadicNilable(a, a, a, a, nil, nil)
	variadicNilable(a, a, a, a, a, nil)
	variadicNonNil(a, a, a, a)
	variadicNonNil(a, a, a, a, nil)
	variadicNonNil(a, a, a, a, a)
	variadicNonNil(a, a, a, a, a, nil)
	variadicNonNilLonger(a, a, a, a, a)
	variadicNonNilLonger(a, a, a, a, a, a, nil)
	variadicNonNilLonger(a, a, a, a, a, nil, nil)
}

type (
	// nilable(a)
	multiStructOne struct {
		a *A
		b *A
	}

	// nilable(b)
	multiStructTwo struct {
		a *A
		b *A
	}
)

// nonnil(result 0)
func testMultiStructDecl(m1 *multiStructOne, m2 *multiStructTwo) *A { //want "returned from `testMultiStructDecl.*` in position 0" "returned from `testMultiStructDecl.*` in position 0"
	a1 := m1.a
	b1 := m1.b
	a2 := m2.a
	b2 := m2.b

	switch 0 {
	case 1:
		return a1
	case 2:
		return b1
	case 3:
		return a2
	case 4:
		return b2
	default:
		m1.a = nil
		m1.b = nil
		print(*m1.b) //want "dereferenced"
		m2.a = nil
		print(*m2.a) //want "dereferenced"
		m2.b = nil
		return &A{}
	}
}

// nilable(param 0, param 2), nonnil(param 1, param 3)
func anonParams(*int, *int, *int, *int) { //want "passed as arg 1" "passed as arg 3"
	i := 0
	anonParams(&i, &i, &i, &i)
	anonParams(nil, &i, nil, &i)
	anonParams(nil, nil, nil, nil)
}

// nilable(result 0, result 2), nonnil(result 1, result 3)
func anonResults() (*int, *int, *int, *int) { //want "returned from `anonResults.*` in position 1" "returned from `anonResults.*` in position 3"
	i := 0
	switch 0 {
	case 1:
		return &i, &i, &i, &i
	case 2:
		return nil, &i, nil, &i
	default:
		return nil, nil, nil, nil
	}
}

// nonnil(b)
func takesPacked(b ...*int) {} //want "passed as arg `b`"

// takesPackedPair and takesPackedSpread are identical to takesPacked; the call sites in
// testPacking are spread across the three so that the annotation-anchored diagnostics stay on
// separate lines.

// nonnil(b)
func takesPackedPair(b ...*int) {} //want "passed as arg `b`" "passed as arg `b`" "passed as arg `b`" "passed as arg `b`"

// nonnil(b)
func takesPackedSpread(b ...*int) {} //want "passed as arg `b`"

// nilable(b)
// nilable(d[])
func testPacking(a *int, b *int, c []*int, d []*int) {
	takesPacked(a)
	takesPacked(b)
	takesPackedPair(a, a)
	takesPackedPair(a, b)
	takesPackedPair(b, a)
	takesPackedPair(b, b)
	takesPackedSpread(c...)
	takesPackedSpread(d...)
}
