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

package returnlocal

// Same as funcreturnbasic.go, but for methods.

// giveA initializes aptr, so the field read is safe.

type A21 struct {
	ptr  *int
	aptr *leaf
}

type Pool struct{}

func (pool *Pool) giveA() *A21 {
	t := &A21{}
	t.aptr = &leaf{}
	return t
}

func m21() *int {
	pool := new(Pool)
	var b = pool.giveA()
	return b.aptr.ptr
}

// giveEmptyA leaves aptr nil, so the field read is flagged.

type A22 struct {
	ptr  *int
	aptr *leaf
}

func (pool *Pool) giveEmptyA() *A22 {
	t := &A22{}
	return t
}

func m22() *int {
	pool := new(Pool)
	var b = pool.giveEmptyA()
	return b.aptr.ptr //want "accessed field `ptr`"
}
