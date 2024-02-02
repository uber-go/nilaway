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

package optimization

// This is a simple test to check the effectiveness of the optimization added via the `struct field analyzer` that enables NilAway to
// only create triggers for those fields of the struct that are being actively assigned (implying a potential side effect) in the function.
// This approach creates fewer number of triggers allowing NilAway to converge quicker without losing precision.

// Without `struct field analyzer`, m23() in this simple test creates 670 triggers and converges in 31 iterations
// With `struct field analyzer` for assigned fields only, m23() in this simple test creates 70 triggers and converges in 18 iterations (as of Aug 29, 2022)
// With `struct field analyzer` for assigned and accessed fields, m23() in this simple test creates 40 triggers and converges in 18 iterations (as of Aug 31, 2022)

// (NOTE: above numbers are subject to change as NilAway evolves)

type A struct {
	ptr    *int
	aptr   *A
	newPtr *A
}

func m23() {
	a := &A{}
	for dummy() {
		switch dummy() {
		case dummy():
			a.f1()
		case dummy():
			a.f2()
		case dummy():
			a.f3()
		case dummy():
			a.f4()
		case dummy():
			a.f5()
		case dummy():
			a.f6()
		case dummy():
			a.f7()
		case dummy():
			a.f8()
		case dummy():
			a.f9()
		case dummy():
			a.f10()
		}
	}
}

func (*A) f1()  {}
func (*A) f2()  {}
func (*A) f3()  {}
func (*A) f4()  {}
func (*A) f5()  {}
func (*A) f6()  {}
func (*A) f7()  {}
func (*A) f8()  {}
func (*A) f9()  {}
func (*A) f10() {}

func dummy() bool {
	return true
}
