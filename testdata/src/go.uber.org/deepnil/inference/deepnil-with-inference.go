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

// This package aims to test deep nilability in the inference mode.

package inference

var dummy bool

var globalNil *int = nil

func retNil() *int {
	return nil
}

func retNilSometimes() *int {
	if dummy {
		return nil
	}
	return new(int)
}

func testLocalDeepAssignNil(i int) {
	switch i {
	case 0:
		m := make(map[int]*int)
		m[0] = nil
		if v, ok := m[0]; ok {
			_ = *v //want "literal `nil` dereferenced"
		}
		if m[0] != nil {
			_ = *m[0]
		}

	case 1:
		m := make(map[int]*int)
		m[0] = globalNil
		if v, ok := m[0]; ok && v != nil {
			_ = *v
		} else {
			_ = *v //want "global variable `globalNil` dereferenced"
		}

	case 2:
		m := make(map[int]*int)
		m[i] = nil
		if v, ok := m[i]; ok {
			_ = *v //want "dereferenced"
		}
		if m[i] != nil {
			_ = *m[i]
		}

	case 3:
		m := make(map[int]*int)
		m[i] = retNilSometimes()
		if v, ok := m[i]; ok && v != nil {
			_ = *v
		} else {
			_ = *v //want "literal `nil` returned from `retNilSometimes"
		}

	case 4:
		sl := make([]*int, 1)
		sl[0] = nil
		_ = *sl[0] //want "literal `nil` dereferenced"

		sl[0] = new(int)
		_ = *sl[0]

	case 5:
		sl := make([]*int, 1)
		sl[i] = nil
		_ = *sl[i] //want "dereferenced"

	case 6:
		sl := make([]*int, 1)
		sl[0] = retNil()
		_ = *sl[0] //want "result 0 of `ret.*` dereferenced"

	case 7:
		sl := make([]*int, 1)
		sl[i] = retNilSometimes()
		_ = *sl[i] //want "literal `nil` returned from `retNilSometimes"

	case 8:
		ch := make(chan *int)
		ch <- nil
		_ = *(<-ch) //want "deep read from local variable `ch` dereferenced"
	}
}

// below tests verify the deep nilablility of local variables, where the deep assignment and read
// are separated interprocedurally.
var globalNil2 *int = nil

type A struct {
	f *int
}

func deepLocalReturn1() []*int {
	s := make([]*int, 1)
	s[0] = nil
	return s
}

func deepLocalReturn2() []*int {
	s := make([]*int, 1)
	s[0] = new(int)
	return s
}

func deepLocalReturn3(i int) []*int {
	s := make([]*int, 1)
	s[i] = nil
	return s
}

func deepLocalReturn4(i int) []*int {
	s := make([]*int, 1)
	s[i] = new(int)
	return s
}

func deepLocalReturn5(i int) []*int {
	s := make([]*int, 1)
	s[i] = retNil()
	return s
}

func deepLocalReturn6(i int) []*int {
	s := make([]*int, 1)
	s[i] = retNilSometimes()
	return s
}

func deepLocalReturn7(i int, x *int) []*int {
	s := make([]*int, 1)
	s[i] = x
	return s
}

func deepLocalReturn8() []*int {
	s := make([]*int, 1)
	s[0] = globalNil2
	return s
}

func deepLocalReturn9(i int, o []*int) []*int {
	s := make([]*int, 1)
	s[i] = o[i]
	return s
}

func deepLocalReturn10(a *A) []*int {
	s := make([]*int, 1)
	s[0] = a.f
	return s
}

func testDeepLocalInterprocedural(i int) {
	switch i {
	case 1:
		_ = *deepLocalReturn1()[i] //want "deep read from result 0 of `deepLocalReturn1.*` dereferenced"
	case 2:
		_ = *deepLocalReturn2()[i]
	case 3:
		v := deepLocalReturn3(i)[i]
		_ = *v //want "deep read from result 0 of `deepLocalReturn3.*` dereferenced"
	case 4:
		_ = *deepLocalReturn4(i)[i]
	case 5:
		_ = *deepLocalReturn5(i)[i] //want "deep read from result 0 of `deepLocalReturn5.*` dereferenced"
	case 6:
		_ = *deepLocalReturn6(i)[i] //want "deep read from result 0 of `deepLocalReturn6.*` dereferenced"
	case 7:
		var x *int
		_ = *deepLocalReturn7(i, x)[i] //want "deep read from result 0 of `deepLocalReturn7.*` dereferenced"
	case 8:
		_ = *deepLocalReturn8()[0] //want "deep read from result 0 of `deepLocalReturn8.*` dereferenced"
	case 9:
		o := make([]*int, 1)
		o[0] = nil
		_ = *deepLocalReturn9(i, o)[i] //want "deep read from result 0 of `deepLocalReturn9.*` dereferenced"
	case 10:
		a := &A{}
		a.f = nil
		_ = *deepLocalReturn10(a)[0] //want "deep read from result 0 of `deepLocalReturn10.*` dereferenced"
	}
}

// below test checks deep nilability with named returns
func retDeepNilNamed() (s []*int) {
	s = make([]*int, 1)
	s[0] = nil
	return
}

func testDeepNilNamed() {
	_ = *retDeepNilNamed()[0] //want "deep read from result 0 of `retDeepNilNamed.*` dereferenced"
}

// below test checks deep nilability with multiple return values
func retMultiple() ([]*int, []*int) {
	s1 := make([]*int, 1)
	s1[0] = nil

	s2 := make([]*int, 1)
	s2[0] = nil

	return s1, s2
}

func testMultiple() {
	a, b := retMultiple()
	_ = *a[0] //want "returned deeply from `retMultiple.*` in position 0"
	_ = *b[0] //want "returned deeply from `retMultiple.*` in position 1"
}

// below test checks for deep nilability from a function chain
func retDeepNilChain() []*int {
	return retDeepNil()
}

func retDeepNil() []*int {
	s := make([]*int, 1)
	s[0] = nil
	return s
}

func testDeepNilChain() {
	_ = *retDeepNilChain()[0] //want "deep read from result 0 of `retDeepNil.*` returned deeply from `retDeepNilChain.*`"
}

// below test checks for deep nilability of a parameter
func readDeepParam(m map[int]*int) {
	i := 0
	if v, ok := m[i]; ok {
		_ = *v //want "deep read from parameter `m` dereferenced"
	}
}

func testDeepNilParam() {
	m := make(map[int]*int)
	m[0] = nil
	readDeepParam(m)
}

// below test checks for deep nilabililty of a variadic parameter
func nilVariadicParam(s ...*int) {
	for _, v := range s {
		_ = *v //want "index of variadic parameter `s` dereferenced"
	}
}

func testNilVariadicParam() {
	s := make([]*int, 1)
	s[0] = nil
	nilVariadicParam(s...)
}

// below test checks for deep nilability of a global variable
var globalS []*string = make([]*string, 1)

func deepGlobal() []*string {
	globalS[0] = nil
	return globalS
}

func testDeepGlobal() {
	_ = *deepGlobal()[0] //want "deep read from global variable `globalS`"
}
