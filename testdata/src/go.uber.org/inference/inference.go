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

package inference

// var dummyBool bool
// var dummyInt int
//
// func retsNilable1() *int {
// 	return nil
// }
//
// func retsNilable2() *int {
// 	if dummyBool {
// 		return &dummyInt
// 	}
// 	return nil
// }
//
// func retsNilable3() *int {
// 	switch dummyInt {
// 	case dummyInt:
// 		return retsNilable1()
// 	case dummyInt:
// 		return retsNilable2()
// 	case dummyInt:
// 		return retsNilable3()
// 	}
// 	return &dummyInt
// }
//
// func retsNonnil1() *int {
// 	return &dummyInt
// }
//
// func retsNonnil2() *int {
// 	if dummyBool {
// 		return &dummyInt
// 	}
// 	return &dummyInt
// }
//
// func retsNonnil3() *int {
// 	switch dummyInt {
// 	case dummyInt:
// 		return retsNonnil1()
// 	case dummyInt:
// 		return retsNonnil2()
// 	case dummyInt:
// 		return retsNonnil3()
// 	}
// 	return &dummyInt
// }
//
// func retsNilable4() *int {
// 	if dummyBool {
// 		return retsNilable3()
// 	}
// 	return retsNilable3()
// }
//
// func takesNonnil(x *int) int {
// 	return *x
// }
//
// func takesNilable(x *int) int {
// 	if x == nil {
// 		return 0
// 	}
// 	return *x
// }
//
// func retsAndTakes() {
// 	switch dummyInt {
// 	case dummyInt:
// 		takesNonnil(retsNonnil1())
// 		takesNonnil(retsNonnil2())
// 		takesNonnil(retsNonnil3())
//
// 		takesNilable(retsNonnil1())
// 		takesNilable(retsNonnil2())
// 		takesNilable(retsNonnil3())
//
// 		takesNilable(retsNilable1())
// 		takesNilable(retsNilable2())
// 		takesNilable(retsNilable3())
// 		takesNilable(retsNilable4())
// 	}
// }
//
// // Below test checks the working of inference in the presence of annotations
// // nonnil(x) nilable(result 0)
// func foo(x *int) *int { //want "NONNIL because it is annotated as so"
// 	print(*x)
// 	return nil
// }
//
// func callFoo() {
// 	ptr := foo(nil)
// 	print(*ptr) //want "NILABLE because it is annotated as so"
// }

var dummy bool

func retPtrErr() (map[string]*int, bool) {
	if dummy {
		return make(map[string]*int), true
	}
	return nil, false
}

func testme() {
	m, ok := retPtrErr()
	if !ok {
		return
	}
	_ = *m["abc"] //want "dereferenced"
}

func testme2(mp map[int]*int) {
	if _, ok := mp[0]; !ok {
		mp[0] = new(int)
	}
	_ = *mp[0]
}

func testRegression(mp map[int]*int) {
	if v1, ok1 := mp[0]; ok1 {
		_ = *v1
	}
	if v2, ok2 := mp[0]; ok2 {
		_ = *v2
	}
}
