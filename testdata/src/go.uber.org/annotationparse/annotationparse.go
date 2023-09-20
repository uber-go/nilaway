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

<nilaway no inference>
*/
package annotationparse

// nilable(lug)
/*
nilable(jar)
nilable(karp)
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
/*
nilable(f, h)
*/
func foo(a, b *bar, c *bar, d, e *bar) (f, g *bar, h *bar) {
	myBar := &bar{}

	myBar.jar = a
	myBar.karp = a
	myBar.lug = a
	myBar.myr = a //want "assigned into field `myr`"

	myBar.jar = b
	myBar.karp = b
	myBar.lug = b
	myBar.myr = b

	myBar.jar = c
	myBar.karp = c
	myBar.lug = c
	myBar.myr = c //want "assigned into field `myr`"

	myBar.jar = d
	myBar.karp = d
	myBar.lug = d
	myBar.myr = d

	myBar.jar = e
	myBar.karp = e
	myBar.lug = e
	myBar.myr = e //want "assigned into field `myr`"

	switch 0 {
	case 1:
		return a, a, a //want "returned from `foo.*` in position 1"
	case 2:
		return b, b, b
	case 3:
		return c, c, c //want "returned from `foo.*` in position 1"
	case 4:
		return d, d, d
	default:
		return e, e, e //want "returned from `foo.*` in position 1"
	}
}

type A struct{}

// the following two functions test that variadic parameters are handled appropriately within their
// function bodies.
// calling these functions in variadicTest tests that variadic parameters are handled appropriately
// as sites for external argument passing

// nilable(e)
func variadicNilable(a, b, c *A, d *A, e ...*A) *A {
	if len(e) > 1 {
		e[1] = nil
		return e[0] //want "returned"
	}
	return a
}

func variadicNonNil(a, b, c *A, d *A, e ...*A) *A {
	if len(e) > 1 {
		e[1] = nil //want "assigned"
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
	variadicNonNil(a, a, a, a, nil) //want "passed"
	variadicNonNil(a, a, a, a, a)
	variadicNonNil(a, a, a, a, a, nil)      //want "passed"
	variadicNonNil(a, a, a, a, a, a, nil)   //want "passed"
	variadicNonNil(a, a, a, a, a, nil, nil) //want "passed" "passed"
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

func testMultiStructDecl(m1 *multiStructOne, m2 *multiStructTwo) *A {
	a1 := m1.a
	b1 := m1.b
	a2 := m2.a
	b2 := m2.b

	switch 0 {
	case 1:
		return a1 //want "returned"
	case 2:
		return b1
	case 3:
		return a2
	case 4:
		return b2 //want "returned"
	default:
		m1.a = nil
		m1.b = nil //want "assigned into field"
		m2.a = nil //want "assigned into field"
		m2.b = nil
		return &A{}
	}
}

// nilable(param 0, param 2)
func anonParams(*int, *int, *int, *int) {
	i := 0
	anonParams(&i, &i, &i, &i)
	anonParams(nil, &i, nil, &i)
	anonParams(nil, nil, nil, nil) //want "passed" "passed"
}

// nilable(result 0, result 2)
func anonResults() (*int, *int, *int, *int) {
	i := 0
	switch 0 {
	case 1:
		return &i, &i, &i, &i
	case 2:
		return nil, &i, nil, &i
	default:
		return nil, nil, nil, nil //want "returned" "returned"
	}
}

func takesPacked(b ...*int) {}

// nilable(b)
// nilable(d[])
func testPacking(a *int, b *int, c []*int, d []*int) {
	takesPacked(a)
	takesPacked(b) //want "passed"
	takesPacked(a, a)
	takesPacked(a, b) //want "passed"
	takesPacked(b, a) //want "passed"
	takesPacked(b, b) //want "passed" "passed"
	takesPacked(c...)
	takesPacked(d...) //want "passed"
}
