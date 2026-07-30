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
	"go.uber.org/nilaway/internal/nilawaytest"
)

// This test file tests the implementation of the `equals` method defined for the interfaces `ConsumingAnnotationTrigger`,
// `ProducingAnnotationTrigger` and `Key`.

// equaler is the constraint satisfied by any interface T in this package that defines an `equals(other T) bool`
// method. Since `equals` is unexported, this constraint must live in the `annotation` package.
type equaler[T any] interface {
	equals(other T) bool
}

// EqualsTestSuite is a generic test suite that checks the `equals` method implementation of every struct in
// initStructs, all of which implement the single interface T.
type EqualsTestSuite[T equaler[T]] struct {
	suite.Suite
	initStructs   []T
	interfaceName string
	packagePath   string
}

// NewEqualsTestSuite constructs an EqualsTestSuite for interfaceName, backed by initStructs.
func NewEqualsTestSuite[T equaler[T]](interfaceName string, initStructs []T) *EqualsTestSuite[T] {
	return &EqualsTestSuite[T]{interfaceName: interfaceName, initStructs: initStructs, packagePath: "."}
}

// This test checks that the `equals` method of all the implemented consumer structs when compared with themselves
// returns true. Although trivial, this test is important to ensure that the type assertion in `equals` method is
// implemented correctly.
func (s *EqualsTestSuite[T]) TestEqualsTrue() {
	msg := "equals() of `%T` should return true when compared with object of same type"

	for _, t := range s.initStructs {
		s.Truef(t.equals(t), msg, t)
	}
}

// This test checks that the `equals` method of all the implemented consumer structs when compared with any other consumer
// struct returns false. This test is important to ensure that the `equals` method is robust to differentiate between
// different consumer struct types.
func (s *EqualsTestSuite[T]) TestEqualsFalse() {
	msg := "equals() of `%T` should return false when compared with object of different type `%T`"

	for _, t1 := range s.initStructs {
		for _, t2 := range s.initStructs {
			// T is an interface type without a `comparable` constraint, so compare through `any` while still
			// comparing the underlying pointer values.
			if any(t1) != any(t2) {
				s.Falsef(t1.equals(t2), msg, t1, t2)
			}
		}
	}
}

// This test serves as a sanity check to ensure that all the implemented consumer structs are tested in this file.
// Ideally, we would have liked to programmatically parse all the consumer structs, instantiate them, and call their
// methods. However, this does not seem to be possible. Therefore, we rely on this not-so-ideal, but practical approach.
// It finds the expected list of consumer structs implementing the interface under test (e.g., `ConsumingAnnotationTrigger`)
// using `nilawaytest.StructsImplementingInterface()`, and finds the actual list of consumer structs that are tested in the
// governing test case. The test fails if there are any structs that are missing from the expected list.
func (s *EqualsTestSuite[T]) TestStructsChecked() {
	missedStructs := nilawaytest.MissingStructs(s.interfaceName, s.packagePath, s.initStructs)
	s.Equalf(0, len(missedStructs), "the following structs were not tested: [`%s`]", strings.Join(missedStructs, "`, `"))
}
