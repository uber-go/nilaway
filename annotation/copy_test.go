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

// CopyTestSuite is a generic test suite that checks the deep-copy method implementation of every struct in
// initStructs, all of which implement interface T. copyFn performs the actual copy for a given element.
//
// Note that copyFn is a plain function value rather than a method constraint on T: ConsumingAnnotationTrigger
// exposes `Copy() ConsumingAnnotationTrigger` while Key exposes the unexported `copy() Key`.
type CopyTestSuite[T any] struct {
	suite.Suite
	initStructs   []T
	copyFn        func(T) T
	interfaceName string
	packagePath   string
}

// NewCopyTestSuite constructs a CopyTestSuite for interfaceName, backed by initStructs and copyFn.
func NewCopyTestSuite[T any](interfaceName string, initStructs []T, copyFn func(T) T) *CopyTestSuite[T] {
	return &CopyTestSuite[T]{interfaceName: interfaceName, initStructs: initStructs, copyFn: copyFn, packagePath: "."}
}

// This test checks that the `Copy`/`copy` method implementations perform a deep copy, i.e., copies the values but generates
// different pointer addresses for the copied struct and its fields.
// Note that here we cannot use `reflect.DeepEqual` to compare the original and copied structs because reflection
// does not work well with fields with nested struct pointers, giving incorrect results.
// Therefore, we compare the original and copied structs along with their fields for:
// - type
// - number of fields
// - pointer address (if the field is a struct and has at least one field)
func (s *CopyTestSuite[T]) TestCopy() {
	for _, initStruct := range s.initStructs {
		expectedObjs := nilawaytest.GetObjInfo(initStruct)
		copied := s.copyFn(initStruct)
		actualObjs := nilawaytest.GetObjInfo(copied)

		for expectedKey, expectedObj := range expectedObjs {
			actualObj, ok := actualObjs[expectedKey]
			s.True(ok, "key `%s` should exist in copied struct object", expectedKey)
			s.Equal(expectedObj.Typ, actualObj.Typ, "key `%s` should have the same type after deep copying", expectedKey)
			s.Equal(expectedObj.NumFields, actualObj.NumFields, "key `%s` should have the same number of fields after deep copying", expectedKey)

			// Note that Go optimizes the memory allocation of pointers to structs. The pointer address for structs with
			// no fields will be the same. E.g., consider struct `S` with no fields, then `s1 := &S{}, s2 := &S{}`;
			// fmt.Printf("%p %p", s1, s2) will print the same address. Therefore, we only add the pointer address of a struct
			// if it has at least one field. The reason for this being that currently, the use of this helper function is used only in
			// the `CopyTestSuite` to check that the `Copy` method implementations perform a deep copy, i.e., generates different
			// pointer addresses for the copied struct and its fields. We may want to modify this behavior in the future, if needed.
			if expectedObj.Addr != "" && actualObj.Addr != "" && expectedObj.NumFields > 0 && actualObj.NumFields > 0 {
				s.NotEqual(expectedObj.Addr, actualObj.Addr, "key `%s` should not have the same pointer value after deep copying", expectedKey)
			}
		}
	}
}

// Similar to EqualsTestSuite, this test serves as a sanity check to ensure that all the implemented consumer structs
// are tested in this file. The test fails if there are any structs that are found missing from the expected list.
func (s *CopyTestSuite[T]) TestStructsChecked() {
	missedStructs := nilawaytest.MissingStructs(s.interfaceName, s.packagePath, s.initStructs)
	s.Equalf(0, len(missedStructs), "the following structs were not tested: [`%s`]", strings.Join(missedStructs, "`, `"))
}
