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

// Package global checks if the struct initialization checker works as expected on the global variables.
package global

// Negative test

type A struct {
	ptr  *int
	aptr *A
}

var g *A = nil

func f() {
	g = &A{}
	// g.aptr is initialized here
	g.aptr = new(A)
}

func h() {
	if g == nil {
		return
	}

	print(g.aptr.ptr)
}

// Positive test

type A2 struct {
	ptr  *int
	aptr *A2
}

var g2 *A2 = nil

func f2() {
	g2 = &A2{}
	g2.aptr = nil
}

func h2() {
	if g2 == nil {
		return
	}

	print(g2.aptr.ptr) //want "field `aptr` accessed field `ptr`"
}

var g3 = &A{}

func f3() {
	// g3.aptr is initialized here
	g3.aptr = new(A)
}

func h3() {
	print(g3.aptr.ptr)
}
