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
*/
package receivers

type S struct {
	f string
}

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
	_ = s.f //want "used as receiver to call `nonnilRecv.*`" "used as receiver to call `nonnilRecv.*`" "used as receiver to call `nonnilRecv.*`" "used as receiver to call `nonnilRecv.*`"
}

// nonnilRecvAtMerge behaves identically to nonnilRecv, but is reserved for the call sites at
// control-flow merge points in `testCaller` (after branches with different nilness join), so that
// branch-local flows (reported in nonnilRecv) and merged flows are reported on separate lines.
func (s *S) nonnilRecvAtMerge() {
	_ = s.f //want "used as receiver to call `nonnilRecvAtMerge.*`" "used as receiver to call `nonnilRecvAtMerge.*`" "used as receiver to call `nonnilRecvAtMerge.*`"
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

func (_ *S) blankIdentifierPointerRecv(i int) *int {
	return &i
}

func (_ S) blankIdentifierNonPointerRecv(i int) *int {
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
		s.nonnilRecv()
	case 1:
		s = &S{}
		s.nonnilRecv()
	case 2:
		if dummy {
			s = &S{}
		}
		s.nonnilRecv()
	case 3:
		if s != nil {
			if dummy {
				s.nonnilRecv()
			}
			if dummy {
				if dummy {
					s = nil // DECL_2: s is assigned nil
					if dummy {
						s.nonnilRecv()
					}
				}
				if dummy {
					s.nonnilRecv()
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
				s.nonnilRecvAtMerge()
			}
		}
		// here - two different flows result in a nilable (DECL_1 and DECL_2)
		s.nonnilRecvAtMerge()

	case 4:
		s.nonPointerRecv() //want "unassigned variable"

	case 5:
		// A blank pointer receiver is nil-safe under inference because the method does not use it.
		// Explicitly dereference the same receiver before the call to retain this nil-use case.
		_ = *s //want "unassigned variable"
		s.blankPointerRecv(0)

	case 7:
		s.blankNonPointerRecv(0) //want "unassigned variable"

	case 8:
		// As above, retain the nil-use check without changing the blank-receiver method.
		_ = *s //want "unassigned variable"
		s.blankIdentifierPointerRecv(0)

	case 9:
		s.blankIdentifierNonPointerRecv(0) //want "unassigned variable"

	case 10:
		print(errObj.Error()) //want "unassigned variable"

	case 11:
		e.errField = errObj
		print(e.errField.Error()) //want "unassigned variable"
	}
}

type myString []*string

// A nil slice used as the receiver value is reported at the index expression in the method body.
func (s myString) testDeepTypeRecv() {
	x := s
	_ = *x[0] //want "sliced into"
}

func (s *myString) testShallowAndDeepTypeRecv(i int) {
	x := *s //want "dereferenced"
	// Note: deep nilability (the pointed-to slice being nil) is not tracked through pointer
	// receivers under inference, so the index below is treated optimistically (no error).
	_ = *x[0]
}

func testNilableRecvCallers(i int) {
	var s *S
	switch i {
	case 0:
		// The unguarded field access in `nilableRecv` errors in the method body for this nil receiver.
		_ = s.nilableRecv(0)
	case 1:
		s = &S{}
		_ = s.nilableRecv(0) // safe: receiver is nonnil
	case 2:
		var ms *myString
		// The nil receiver is dereferenced unguarded in the method body (`x := *s`), reported there.
		ms.testShallowAndDeepTypeRecv(0)
	case 3:
		var ms myString
		// The nil slice receiver errors at the index expression in the method body.
		ms.testDeepTypeRecv()
	case 4:
		ms := myString{nil}
		ms.testDeepTypeRecv()            // safe: receiver slice is nonnil
		ms.testShallowAndDeepTypeRecv(0) // safe: receiver pointer is nonnil
	}
}

// below tests check for nilable receivers in case of named types

type myInt int

func (m *myInt) nonnilNamedRecv() {
	_ = *m //want "unassigned variable `m` used as receiver" "used as receiver to call" "used as receiver to call" "used as receiver to call"
}

// nonnilNamedRecvAtMerge behaves identically to nonnilNamedRecv, but is reserved for the call
// sites at control-flow merge points in `testNamedTypes`, so that branch-local flows (reported in
// nonnilNamedRecv) and merged flows are reported on separate lines.
func (m *myInt) nonnilNamedRecvAtMerge() {
	_ = *m //want "used as receiver to call" "used as receiver to call" "used as receiver to call"
}

func (m *myInt) nilableNamedRecv() {
	if m != nil {
		_ = *m
	}
}

func testNamedTypes(dummy bool, i int) {
	var m *myInt
	value := myInt(1)

	switch i {
	case 1:
		m.nonnilNamedRecv()
	case 2:
		m.nilableNamedRecv() // safe at call site
	case 3:
		m = &value
		m.nonnilNamedRecv()
	case 4:
		if dummy {
			m = &value
		}
		m.nonnilNamedRecv()
	case 5:
		if m != nil {
			if dummy {
				m.nonnilNamedRecv()
			}
			if dummy {
				if dummy {
					m = nil // DECL_2: m is assigned nil
					if dummy {
						m.nonnilNamedRecv()
					}
				}
				if dummy {
					m.nonnilNamedRecv()
				}
			} else {
				if dummy {
					m.nonnilNamedRecv()
				}
				if dummy {
					m = &value
				}
				if dummy {
					m.nonnilNamedRecv()
				}
			}
			if dummy {
				m.nonnilNamedRecvAtMerge()
			}
		}
		// here - two different flows result in a nilable (DECL_1 and DECL_2)
		m.nonnilNamedRecvAtMerge()
	}
}
