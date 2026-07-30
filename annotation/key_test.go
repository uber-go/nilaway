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
	"go.uber.org/nilaway/nilawaytest"
)

const _interfaceNameKey = "Key"

// initStructsKey initializes all structs that implement the Key interface
var initStructsKey = []Key{
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
	&StructFieldContextSite{},
}

// TestKeyEqualsSuite runs the test suite for the `equals` method of all the structs that implement
// the `Key` interface.
func TestKeyEqualsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, nilawaytest.NewEqualsTestSuite(
		_interfaceNameKey,
		".",
		initStructsKey,
		func(k1, k2 Key) bool { return k1.equals(k2) },
	))
}

// TestKeyCopySuite runs the test suite for the `copy` method of all the structs that implement the `Key` interface.
func TestKeyCopySuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, nilawaytest.NewCopyTestSuite(_interfaceNameKey, ".", initStructsKey, func(k Key) Key { return k.copy() }))
}
