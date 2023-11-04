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

package annotation

import (
	"strings"

	"github.com/stretchr/testify/suite"
)

// This test file tests the implementation of the `equals` method defined for the interfaces `ConsumingAnnotationTrigger`,
// `ProducingAnnotationTrigger` and `Key`.

type EqualsTestSuite struct {
	suite.Suite
	initStructs   []any
	interfaceName string
	packagePath   string
}

// This test checks that the `equals` method of all the implemented consumer structs when compared with themselves
// returns true. Although trivial, this test is important to ensure that the type assertion in `equals` method is
// implemented correctly.
func (s *EqualsTestSuite) TestEqualsTrue() {
	msg := "equals() of `%T` should return true when compared with object of same type"

	for _, initStruct := range s.initStructs {
		switch t := initStruct.(type) {
		case ConsumingAnnotationTrigger:
			s.Truef(t.equals(t), msg, t)
		case ProducingAnnotationTrigger:
			s.Truef(t.equals(t), msg, t)
		case Key:
			s.Truef(t.equals(t), msg, t)
		default:
			s.Failf("unknown type", "unknown type `%T`", t)
		}
	}
}

// This test checks that the `equals` method of all the implemented consumer structs when compared with any other consumer
// struct returns false. This test is important to ensure that the `equals` method is robust to differentiate between
// different consumer struct types.
func (s *EqualsTestSuite) TestEqualsFalse() {
	msg := "equals() of `%T` should return false when compared with object of different type `%T`"

	for _, s1 := range s.initStructs {
		for _, s2 := range s.initStructs {
			if s1 != s2 {
				switch t1 := s1.(type) {
				case ConsumingAnnotationTrigger:
					if t2, ok := s2.(ConsumingAnnotationTrigger); ok {
						s.Falsef(t1.equals(t2), msg, t1, t2)
					}
				case ProducingAnnotationTrigger:
					if t2, ok := s2.(ProducingAnnotationTrigger); ok {
						s.Falsef(t1.equals(t2), msg, t1, t2)
					}
				case Key:
					if t2, ok := s2.(Key); ok {
						s.Falsef(t1.equals(t2), msg, t1, t2)
					}
				default:
					s.Failf("unknown type", "unknown type `%T`", t1)
				}
			}
		}
	}
}

// This test serves as a sanity check to ensure that all the implemented consumer structs are tested in this file.
// Ideally, we would have liked to  programmatically parse all the consumer structs, instantiate them, and call their
// methods. However, this does not seem to be possible. Therefore, we rely on this not-so-ideal, but practical approach.
// It finds the expected list of consumer structs implementing the interface under test (e.g., `ConsumingAnnotationTrigger`)
// using `structsImplementingInterface()`, and finds the actual list of consumer structs that are tested in the
// governing test case. The test fails if there are any structs that are missing from the expected list.
func (s *EqualsTestSuite) TestStructsChecked() {
	missedStructs := structsCheckedTestHelper(s.interfaceName, s.packagePath, s.initStructs)
	s.Equalf(0, len(missedStructs), "the following structs were not tested: [`%s`]", strings.Join(missedStructs, "`, `"))
}
