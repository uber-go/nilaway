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
This set of tests aims to capture a grab bag of strange go features

<nilaway no inference>
*/
package goquirks

type A int
type B = *int

// nilable(b1, b3)
func (a A) add(b1, b2, b3 B) {}

// this tests that both forms of method call are parsed for non-pointer receivers
// nilable(b1)
func foo(a A, b1, b2 B) {
	a.add(b1, b2, b1)
	A.add(a, b1, b2, b1)
	a.add(b2, b1, b2)    //want "nilable value passed"
	A.add(a, b2, b1, b2) //want "nilable value passed"
}

// nilable(b1, b3)
func (a *A) add2(b1, b2, b3 B) {}

// this tests that both forms of method call are parsed for pointer receivers
// nilable(b1)
func foo2(a *A, b1, b2 B) {
	a.add2(b1, b2, b1)
	(*A).add2(a, b1, b2, b1)
	a.add2(b2, b1, b2)       //want "nilable value passed"
	(*A).add2(a, b2, b1, b2) //want "nilable value passed"
}

// this tests the common paradigm in go of a nilable return of error type
// we want to assume these are nilable
func fooThatErrs() error {
	return nil
}

func fooThatErrs2() (*int, error, *int) {
	i := 0
	return &i, nil, &i
}

func fooThatConsumesErrs() interface{} {
	a := fooThatErrs()
	b, c, d := fooThatErrs2()
	switch 0 {
	case 1:
		return a //want "nilable value returned"
	case 2:
		return b
	case 3:
		return c //want "nilable value returned"
	default:
		return d
	}
}
