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
	"testing"

	"github.com/stretchr/testify/suite"
)

const _interfaceNameKey = "Key"

// initStructsKey initializes all structs that implement the Key interface
var initStructsKey = []any{
	&FieldAnnotationKey{},
	&CallSiteParamAnnotationKey{},
	&ParamAnnotationKey{},
	&CallSiteRetAnnotationKey{},
	&RetAnnotationKey{},
	&TypeNameAnnotationKey{},
	&GlobalVarAnnotationKey{},
	&RecvAnnotationKey{},
	&RetFieldAnnotationKey{},
	&EscapeFieldAnnotationKey{},
	&ParamFieldAnnotationKey{},
	&LocalVarAnnotationKey{},
}

// TestKeyEqualsSuite runs the test suite for the `equals` method of all the structs that implement
// the `Key` interface.
type KeyEqualsTestSuite struct {
	EqualsTestSuite
}

func (s *KeyEqualsTestSuite) SetupTest() {
	s.interfaceName = _interfaceNameKey
	s.initStructs = initStructsKey
}

func TestKeyEqualsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(KeyEqualsTestSuite))
}

// TestKeyCopySuite runs the test suite for the `copy` method of all the structs that implement the `Key` interface.
type KeyCopyTestSuite struct {
	CopyTestSuite
}

func (s *KeyCopyTestSuite) SetupTest() {
	s.interfaceName = _interfaceNameKey
	s.initStructs = initStructsKey
}

func TestKeyCopySuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(KeyCopyTestSuite))
}
