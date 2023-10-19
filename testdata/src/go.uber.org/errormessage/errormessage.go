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

// This package tests _single_ package inference. Due to limitations of `analysistest` framework,
// multi-package inference is tested by our integration test suites. Please see
// `testdata/README.md` for more details.

package errormessage

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
	return z
}

func callTest6() {
	print(*test6()) //want "`nil` to `x`"
}

func test7() {
	var x *int
	if dummy {
		y := new(int)
		x = y
	}
	print(*x) //want "unassigned variable `x` dereferenced"
}

func test8(i int) {
	x := new(int)
	if dummy {
		var y *int
		x = y
	}
	print(*x) //want "`y` to `x`"
}
