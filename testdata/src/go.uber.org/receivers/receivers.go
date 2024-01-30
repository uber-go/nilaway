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
In Go, receivers can be nilable without causing a nil panic, if they are handled properly. This test file checks for such cases.

<nilaway no inference>
*/
package receivers

type S struct {
	f string
}

// nilable(s)
func (s *S) nilableRecv(i int) string {
	switch i {
	case 0:
		return s.f //want "accessed field `f`"

	case 1:
		if s == nil {
			return "<nil>"
		}
		return s.f

	case 2:
		if s != nil {
			return s.f
		}
	}
	return "xyz"
}

func (s *S) nonnilRecv() {
	_ = s.f
}

func (s S) nonPointerRecv() {
	_ = s.f
}

func (*S) blankPointerRecv(i int) *int {
	return &i
}

func (S) blankNonPointerRecv(i int) *int {
	return &i
}

type myErr struct{}

func (myErr) Error() string { return "myErr message" }

type E struct {
	errField error
}

func testCaller(dummy bool, i int, e *E) {
	var s *S // DECL_1: s is uninitialized
	var errObj *myErr

	switch i {
	case 0:
		s.nonnilRecv() //want "used as receiver to call `nonnilRecv.*`"
	case 1:
		s = &S{}
		s.nonnilRecv()
	case 2:
		if dummy {
			s = &S{}
		}
		s.nonnilRecv() //want "used as receiver to call `nonnilRecv.*`"
	case 3:
		if s != nil {
			if dummy {
				s.nonnilRecv()
			}
			if dummy {
				if dummy {
					s = nil // DECL_2: s is assigned nil
					if dummy {
						s.nonnilRecv() //want "used as receiver to call `nonnilRecv.*`"
					}
				}
				if dummy {
					s.nonnilRecv() //want "used as receiver to call `nonnilRecv.*`"
				}
			} else {
				if dummy {
					s.nonnilRecv()
				}
				if dummy {
					s = &S{}
				}
				if dummy {
					s.nonnilRecv()
				}
			}
			if dummy {
				s.nonnilRecv() //want "used as receiver to call `nonnilRecv.*`"
			}
		}
		// here - two different flows result in a nilable (DECL_1 and DECL_2)
		s.nonnilRecv() //want "used as receiver to call `nonnilRecv.*`" "used as receiver to call `nonnilRecv.*`"

	case 4:
		s.nonPointerRecv() //want "unassigned variable"

	case 5:
		s.blankPointerRecv(0) //want "unassigned variable"

	case 6:
		s.blankNonPointerRecv(0) //want "unassigned variable"

	case 7:
		print(errObj.Error()) //want "unassigned variable"

	case 8:
		e.errField = errObj
		print(e.errField.Error()) //want "unassigned variable"
	}
}

type myString []*string

// nilable(s[])
func (s *myString) testDeepTypeRecv() {
	x := *s
	_ = *x[0] //want "sliced into"
}

// nilable(s, s[])
func (s *myString) testShallowAndDeepTypeRecv(i int) {
	x := *s   //want "dereferenced"
	_ = *x[0] //want "sliced into"
}
