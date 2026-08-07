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

// This package tests error messages for assignment flow tracking.

package errormessage

import "errors"

var dummy bool

func test1(x *int) {
	x = nil
	print(*x) //want "`nil` to `x`"
}

func test2(x *int) {
	x = nil
	y := x
	z := y
	print(*z) //want "`y` to `z`"
}

func test3(x *int) {
	if dummy {
		x = nil
	} else {
		x = new(int)
	}
	y := x
	z := y
	print(*z) //want "`nil` to `x`"
}

// nilable(f)
type S struct {
	f *int
}

func test4(x *int) {
	s := &S{}
	x = nil
	y := x
	z := y
	s.f = z
	print(*s.f) //want "`z` to `s.f`"
}

func test5() {
	x := new(int)
	for i := 0; i < 10; i++ {
		print(*x) //want "`nil` to `y`"
		var y *int = nil
		z := y
		x = z
	}
}

func test6() *int {
	var x *int = nil
	y := x
	z := y
	print(*z) //want "`nil` to `x`"
	return z
}

func test7() {
	var x *int
	if dummy {
		y := new(int)
		x = y
	}
	print(*x) //want "unassigned variable `x` dereferenced"
}

func test8() {
	x := new(int)
	if dummy {
		var y *int
		x = y
	}
	print(*x) //want "`y` to `x`"
}

func test9(m map[int]*int) {
	x, _ := m[0]
	y := x
	print(*y) //want "`m\\[0\\]` to `x`"
}

func test10(nilableChan chan *int, nonnilDeeplyNonnilChan chan *int) {
	x := 1
	nilableChan <- &x
	// Sending nilable values to nonnil and deeply nonnil channels is not OK.
	var y *int
	nonnilDeeplyNonnilChan <- y
	print(*<-nonnilDeeplyNonnilChan) //want "`y` assigned deeply into parameter arg `nonnilDeeplyNonnilChan`"
}

// nilable(s)
func test11(s []*int) {
	x := s[0] //want "`s` sliced into"
	y := x
	print(*y)
}

func test12(mp map[int]*S, i int) {
	x := mp[i] // unrelated assignment, should not be printed in the error message
	_ = x

	y := mp[i] // unrelated assignment, should not be printed in the error message
	_ = y

	s := mp[i] // relevant assignment, should be printed in the error message
	consumeS(s)
}

func consumeS(s *S) {
	print(s.f) //want "`mp\\[i\\]` to `s`"
}

func test12ValueMapRead(mp map[int]S, i int) {
	s := mp[i]
	consumeS(&s)
}

type test12ValueMap map[int]S

func test12NamedValueMapRead(mp test12ValueMap, i int) {
	s := mp[i]
	consumeS(&s)
}

func retErr() error {
	return errors.New("error")
}

func test13() *int {
	if err := retErr(); err != nil { // unrelated assignment, should not be printed in the error message
		return nil
	}
	return new(int)
}

func test13Sink() {
	print(*test13()) //want "literal `nil` returned"
}

// below tests check shortening of expressions in assignment messages

func (s *S) bar(i int) *int {
	return nil
}

func (s *S) foo(a int, b *int, c string, d bool) *S {
	return nil
}

func test14(x *int, i int) {
	s := &S{}
	x = s.foo(1,
		new(int),
		"abc",
		true).bar(i)
	y := x
	print(*y) //want "`s.foo\\(...\\).bar\\(i\\)` to `x`"
}

type test15S S

func (s *test15S) bar(i int) *int {
	return nil
}

func (s *test15S) foo(a int, b *int, c string, d bool) *test15S {
	return nil
}

func test15(x *int) {
	var longVarName, anotherLongVarName, yetAnotherLongName int
	s := &test15S{}
	x = s.foo(longVarName, &anotherLongVarName, "abc", true).bar(yetAnotherLongName)
	y := x
	print(*y) //want "`s.foo\\(...\\).bar\\(...\\)` to `x`"
}

func test16(mp map[int]*int) {
	var aVeryVeryVeryLongIndexVar int
	x := mp[aVeryVeryVeryLongIndexVar]
	y := x
	print(*y) //want "`mp\\[...\\]` to `x`"
}

type test17S S

func (s *test17S) bar(i int) *int {
	return nil
}

func (s *test17S) foo(a int, b *int, c string, d bool) *test17S {
	return nil
}

func test17(x *int, mp map[int]*int) {
	var aVeryVeryVeryLongIndexVar int
	s := &test17S{}

	print(*mp[aVeryVeryVeryLongIndexVar]) //want "deep read"
	x = s.foo(1, mp[aVeryVeryVeryLongIndexVar], "abc", true).bar(2)
	y := x
	print(*y) //want "`s.foo\\(...\\).bar\\(2\\)` to `x`"
}

type test18S S

func (s *test18S) bar(i int) *int {
	return nil
}

func (s *test18S) foo(a int, b *int, c string, d bool) *test18S {
	return nil
}

func test18(x *int, mp map[int]*int) {
	s := &test18S{}
	x = mp[*(s.foo(1, new(int), "abc", true).bar(2))] //want "dereferenced"
	y := x
	print(*y) //want "`mp\\[...\\]` to `x`"
}

type test19S S

func (s *test19S) bar(i int) *int {
	return nil
}

func (s *test19S) foo(a int, b *int, c string, d bool) *test19S {
	return nil
}

func test19() {
	mp := make(map[string]*string)
	x := mp["("]
	y := x
	print(*y) //want "`mp\\[\"\\(\"\\]` to `x`"

	x = mp[")"]
	y = x
	print(*y) //want "`mp\\[\"\\)\"\\]` to `x`"

	x = mp["))"]
	y = x
	print(*y) //want "`mp\\[...\\]` to `x`"

	x = mp["(("]
	y = x
	print(*y) //want "`mp\\[...\\]` to `x`"

	x = mp[")))((("]
	y = x
	print(*y) //want "`mp\\[...\\]` to `x`"

	x = mp[")))((("]
	y = x
	print(*y) //want "`mp\\[...\\]` to `x`"

	x = mp["(((()"]
	y = x
	print(*y) //want "`mp\\[...\\]` to `x`"

	x = mp["())))"]
	y = x
	print(*y) //want "`mp\\[...\\]` to `x`"

	s := &test19S{}
	i := 0
	a := s.foo(1,
		new(int),
		"({[",
		true).bar(i)
	b := a
	print(*b) //want "`s.foo\\(...\\).bar\\(i\\)` to `a`"
}

func test20() {
	mp := make(map[rune]*rune)
	x := mp['(']
	y := x
	print(*y) //want "`mp\\['\\('\\]` to `x`"

	x = mp[')']
	y = x
	print(*y) //want "`mp\\['\\)'\\]` to `x`"
}

// below test checks that NilAway can handle non-English (non-ASCII) identifiers
func test21() {
	var 世界 *int = nil
	print(*世界) //want "`nil` to `世界`"
}

// below tests check assignment flow tracking across many-to-one assignments

func retPtrErr0() (*int, error) { return nil, nil }
func retPtrErr1() (*int, error) { return nil, nil }
func retPtrErr2() (*int, error) { return nil, nil }
func retPtrErr3() (*int, error) { return nil, nil }

func test22(i int) {
	switch i {
	case 0:
		x, err := retPtrErr0()
		if err != nil {
			return
		}
		print(*x) //want "`retPtrErr0\\(\\)` to `x`"

	case 1:
		if x, err := retPtrErr1(); err == nil {
			y := x
			print(*y) //want "`retPtrErr1\\(\\)` to `x`"
		}

	case 2:
		var x *int
		var err error
		x, err = retPtrErr2()
		if err != nil {
			return
		}
		print(*x) //want "`retPtrErr2\\(\\)` to `x`"

	case 3:
		var x, err = retPtrErr3()
		if err != nil {
			return
		}
		print(*x) //want "`retPtrErr3\\(\\)` to `x`"
	}
}

func test23(mp map[int]*int, i int) {
	switch i {
	case 0:
		mp[0] = nil
		v, ok := mp[0]
		if ok {
			print(*v) //want "`mp\\[0\\]` to `v`"
		}

	case 1:
		mp[0] = nil
		if v, ok := mp[0]; ok {
			print(*v) //want "`mp\\[0\\]` to `v`"
		}
	case 2:
		mp[0] = nil
		var v *int
		var ok bool
		v, ok = mp[0]
		if ok {
			print(*v) //want "`mp\\[0\\]` to `v`"
		}
	case 3:
		mp[0] = nil
		var v, ok = mp[0]
		if ok {
			print(*v) //want "`mp\\[0\\]` to `v`"
		}
	}
}

func retMultiple24() (*int, *int, *int) { return nil, new(int), nil }

func test24() {
	a, b, c := retMultiple24()
	print(*a) //want "`retMultiple24\\(\\)` to `a`"
	if dummy {
		a = nil
		b = a
	}
	print(*b) //want "`a` to `b`"
	print(*c) //want "`retMultiple24\\(\\)` to `c`"
}

type A []*int

func retMultiple25() (*int, *int, *int) { return nil, new(int), nil }

func test25(a A) {
	a[0], a[1], _ = retMultiple25()
	print(*a[0]) //want "`retMultiple25\\(\\)` to `a\\[0\\]`"
	print(*a[1])
}
