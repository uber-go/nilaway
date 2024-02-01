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

const _interfaceNameConsumingAnnotationTrigger = "ConsumingAnnotationTrigger"

// initStructsConsumingAnnotationTrigger initializes all structs that implement the ConsumingAnnotationTrigger interface
var initStructsConsumingAnnotationTrigger = []any{
	&TriggerIfNonNil{Ann: newMockKey()},
	&TriggerIfDeepNonNil{Ann: newMockKey()},
	&ConsumeTriggerTautology{},
	&PtrLoad{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
	&MapAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
	&MapWrittenTo{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
	&SliceAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
	&FldAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
	&UseAsErrorResult{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&FldAssign{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&ArgFldPass{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&GlobalVarAssign{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&ArgPass{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&RecvPass{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&InterfaceResultFromImplementation{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&MethodParamFromInterface{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&UseAsReturn{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&UseAsFldOfReturn{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&SliceAssign{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&ArrayAssign{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&PtrAssign{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&MapAssign{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&DeepAssignPrimitive{ConsumeTriggerTautology: &ConsumeTriggerTautology{}},
	&ParamAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&FuncRetAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&VariadicParamAssignDeep{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&FieldAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&GlobalVarAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&LocalVarAssignDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&ChanSend{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&FldEscape{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&UseAsNonErrorRetDependentOnErrorRetNilability{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
	&UseAsErrorRetWithNilabilityUnknown{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
}

// ConsumingAnnotationTriggerEqualsTestSuite tests for the `equals` method of all the structs that implement
// the `ConsumingAnnotationTrigger` interface.
type ConsumingAnnotationTriggerEqualsTestSuite struct {
	EqualsTestSuite
}

func (s *ConsumingAnnotationTriggerEqualsTestSuite) SetupTest() {
	s.interfaceName = _interfaceNameConsumingAnnotationTrigger
	s.initStructs = initStructsConsumingAnnotationTrigger
}

func TestConsumingAnnotationTriggerEqualsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ConsumingAnnotationTriggerEqualsTestSuite))
}

// ConsumingAnnotationTriggerCopyTestSuite tests for the `copy` method of all the structs that implement
// the `ConsumingAnnotationTrigger` interface.
type ConsumingAnnotationTriggerCopyTestSuite struct {
	CopyTestSuite
}

func (s *ConsumingAnnotationTriggerCopyTestSuite) SetupTest() {
	s.interfaceName = _interfaceNameConsumingAnnotationTrigger
	s.initStructs = initStructsConsumingAnnotationTrigger
}
func TestConsumingAnnotationTriggerCopySuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ConsumingAnnotationTriggerCopyTestSuite))
}
