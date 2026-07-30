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

package nilawaytest

import (
	"strings"

	"github.com/stretchr/testify/suite"
)

// EqualsTestSuite is a generic test suite that checks the equality implementation of every struct in
// initStructs, all of which implement the single interface T. equalsFn performs the actual equality
// comparison for a pair of values.
type EqualsTestSuite[T any] struct {
	suite.Suite
	initStructs   []T
	equalsFn      func(T, T) bool
	interfaceName string
	packagePath   string
}

// NewEqualsTestSuite constructs an EqualsTestSuite for interfaceName in packagePath, backed by
// initStructs and equalsFn.
func NewEqualsTestSuite[T any](interfaceName, packagePath string, initStructs []T, equalsFn func(T, T) bool) *EqualsTestSuite[T] {
	return &EqualsTestSuite[T]{
		interfaceName: interfaceName,
		packagePath:   packagePath,
		initStructs:   initStructs,
		equalsFn:      equalsFn,
	}
}

// TestEqualsTrue checks that each value compares equal to itself.
func (s *EqualsTestSuite[T]) TestEqualsTrue() {
	msg := "equals() of `%T` should return true when compared with object of same type"

	for _, t := range s.initStructs {
		s.Truef(s.equalsFn(t, t), msg, t)
	}
}

// TestEqualsFalse checks that values of different concrete types compare unequal.
func (s *EqualsTestSuite[T]) TestEqualsFalse() {
	msg := "equals() of `%T` should return false when compared with object of different type `%T`"

	for _, t1 := range s.initStructs {
		for _, t2 := range s.initStructs {
			// T is typically an interface type without a `comparable` constraint, so compare through `any`
			// while still comparing the underlying pointer values.
			if any(t1) != any(t2) {
				s.Falsef(s.equalsFn(t1, t2), msg, t1, t2)
			}
		}
	}
}

// TestStructsChecked serves as a sanity check to ensure that all the implemented structs are exercised by the suite.
func (s *EqualsTestSuite[T]) TestStructsChecked() {
	missedStructs := MissingStructs(s.interfaceName, s.packagePath, s.initStructs)
	s.Equalf(0, len(missedStructs), "the following structs were not tested: [`%s`]", strings.Join(missedStructs, "`, `"))
}
