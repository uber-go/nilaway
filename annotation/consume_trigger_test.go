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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ConsumingAnnotationTriggerTestSuite struct {
	EqualsTestSuite
}

func (s *ConsumingAnnotationTriggerTestSuite) SetupTest() {
	s.interfaceName = "ConsumingAnnotationTrigger"

	mockedKey := new(mockKey)
	mockedKey.On("equals", mock.Anything).Return(true)

	// initialize all structs that implement ConsumingAnnotationTrigger
	s.initStructs = []any{
		&TriggerIfNonNil{Ann: mockedKey},
		&TriggerIfDeepNonNil{Ann: mockedKey},
		&ConsumeTriggerTautology{},
		&PtrLoad{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
		&MapAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
		&MapWrittenTo{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
		&SliceAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
		&FldAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
		&UseAsErrorResult{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&FldAssign{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&ArgFldPass{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&GlobalVarAssign{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&ArgPass{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&RecvPass{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&InterfaceResultFromImplementation{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&MethodParamFromInterface{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&UseAsReturn{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&UseAsFldOfReturn{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&SliceAssign{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&ArrayAssign{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&PtrAssign{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&MapAssign{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&DeepAssignPrimitive{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
		&ParamAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&FuncRetAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&VariadicParamAssignDeep{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&FieldAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&GlobalVarAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&ChanAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
		&LocalVarAssignDeep{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
		&ChanSend{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: mockedKey}},
		&FldEscape{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&UseAsNonErrorRetDependentOnErrorRetNilability{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
		&UseAsErrorRetWithNilabilityUnknown{TriggerIfNonNil: &TriggerIfNonNil{Ann: mockedKey}},
	}
}

// TestConsumingAnnotationTriggerEqualsSuite runs the test suite for the `equals` method of all the structs that implement
// the `ConsumingAnnotationTrigger` interface.
func TestConsumingAnnotationTriggerEqualsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ConsumingAnnotationTriggerTestSuite))
}
