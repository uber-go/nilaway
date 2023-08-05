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

// similar tests ad funcreturnfields.go for methods
// In this test, field aptr is initialized in giveA() function and thus no error should be reported

type A21 struct {
	ptr  *int
	aptr *A21
}

type Pool struct{}

func (pool *Pool) giveA() *A21 {
	t := &A21{}
	t.aptr = &A21{}
	return t
}

func m21() *int {
	pool := new(Pool)
	var b = pool.giveA()
	return b.aptr.ptr
}

// In this test, field aptr is set to nil in giveEmptyA() function and thus error should be reported

type A22 struct {
	ptr  *int
	aptr *A22
}

func (pool *Pool) giveEmptyA() *A22 {
	t := &A22{}
	return t
}

func m22() *int {
	pool := new(Pool)
	var b = pool.giveEmptyA()
	return b.aptr.ptr //want "field `aptr` of return of the function `giveEmptyA`"
}
