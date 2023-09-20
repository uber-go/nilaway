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
Package funcreturnfields Tests when nilability flows through the field of return of a function or a method
<nilaway struct enable>
*/
package funcreturnfields

// Testing with direct return of a composite literal initialization of struct

func giveEmptyA() *A11 {
	t := &A11{}
	return t
}

func m07() *int {
	b := giveEmptyA()
	return b.aptr.ptr //want "field `aptr` of return of the function `giveEmptyA`"
}

// Testing with direct return of struct as a composite literal.
func giveEmptyAComposite() *A11 {
	return &A11{}
}

func m08() *int {
	t := giveEmptyAComposite()
	return t.aptr.ptr //want "field `aptr` of return of the function `giveEmptyAComposite`"
}

// Testing with transitive return of struct through a function call.
func giveEmptyA11Fun() *A11 {
	return &A11{}
}

// TODO: Location of the error in this case is inappropriate.
func giveEmptyACallFun() *A11 {
	return giveEmptyA11Fun()
}

func m10() *int {
	t := giveEmptyACallFun()
	return t.aptr.ptr //want "field `aptr` of return of the function `giveEmptyACallFun`"
}
