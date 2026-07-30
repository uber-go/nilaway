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

// CopyTestSuite is a generic test suite that checks the deep-copy method implementation of every struct in
// initStructs, all of which implement interface T. copyFn performs the actual copy for a given element.
//
// Note that copyFn is a plain function value rather than a method constraint on T: different NilAway
// interfaces expose different copy methods (for example, `Copy()` versus `copy()`), so callers supply
// whichever copy function applies.
type CopyTestSuite[T any] struct {
	suite.Suite
	initStructs   []T
	copyFn        func(T) T
	interfaceName string
	packagePath   string
}

// NewCopyTestSuite constructs a CopyTestSuite for interfaceName in packagePath, backed by initStructs and copyFn.
func NewCopyTestSuite[T any](interfaceName, packagePath string, initStructs []T, copyFn func(T) T) *CopyTestSuite[T] {
	return &CopyTestSuite[T]{
		interfaceName: interfaceName,
		packagePath:   packagePath,
		initStructs:   initStructs,
		copyFn:        copyFn,
	}
}

// TestCopy checks that the copy implementation performs a deep copy: copied structs preserve shape and types,
// but nested pointer-bearing objects do not alias the original.
func (s *CopyTestSuite[T]) TestCopy() {
	for _, initStruct := range s.initStructs {
		expectedObjs := GetObjInfo(initStruct)
		copied := s.copyFn(initStruct)
		actualObjs := GetObjInfo(copied)

		for expectedKey, expectedObj := range expectedObjs {
			actualObj, ok := actualObjs[expectedKey]
			s.True(ok, "key `%s` should exist in copied struct object", expectedKey)
			s.Equal(expectedObj.Typ, actualObj.Typ, "key `%s` should have the same type after deep copying", expectedKey)
			s.Equal(expectedObj.NumFields, actualObj.NumFields, "key `%s` should have the same number of fields after deep copying", expectedKey)

			// Go may reuse addresses for pointers to zero-sized structs, so only compare pointer addresses when
			// both snapshots describe structs with at least one field.
			if expectedObj.Addr != "" && actualObj.Addr != "" && expectedObj.NumFields > 0 && actualObj.NumFields > 0 {
				s.NotEqual(expectedObj.Addr, actualObj.Addr, "key `%s` should not have the same pointer value after deep copying", expectedKey)
			}
		}
	}
}

// TestStructsChecked serves as a sanity check to ensure that all the implemented structs are exercised by the suite.
func (s *CopyTestSuite[T]) TestStructsChecked() {
	missedStructs := MissingStructs(s.interfaceName, s.packagePath, s.initStructs)
	s.Equalf(0, len(missedStructs), "the following structs were not tested: [`%s`]", strings.Join(missedStructs, "`, `"))
}
