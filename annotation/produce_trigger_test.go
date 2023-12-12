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

type ProducingAnnotationTriggerTestSuite struct {
	EqualsTestSuite
}

func (s *ProducingAnnotationTriggerTestSuite) SetupTest() {
	s.interfaceName = "ProducingAnnotationTrigger"

	mockedKey := new(mockKey)
	mockedKey.On("equals", mock.Anything).Return(true)

	mockedProducingAnnotationTrigger := new(mockProducingAnnotationTrigger)
	mockedProducingAnnotationTrigger.On("equals", mock.Anything).Return(true)

	// initialize all structs that implement ProducingAnnotationTrigger
	s.initStructs = []any{
		&TriggerIfNilable{Ann: mockedKey},
		&TriggerIfDeepNilable{Ann: mockedKey},
		&ProduceTriggerTautology{},
		&ProduceTriggerNever{},
		&ExprOkCheck{ProduceTriggerNever: &ProduceTriggerNever{}},
		&RangeIndexAssignment{ProduceTriggerNever: &ProduceTriggerNever{}},
		&PositiveNilCheck{ProduceTriggerTautology: &ProduceTriggerTautology{}},
		&NegativeNilCheck{ProduceTriggerNever: &ProduceTriggerNever{}},
		&OkReadReflCheck{ProduceTriggerNever: &ProduceTriggerNever{}},
		&RangeOver{ProduceTriggerNever: &ProduceTriggerNever{}},
		&ConstNil{ProduceTriggerTautology: &ProduceTriggerTautology{}},
		&UnassignedFld{ProduceTriggerTautology: &ProduceTriggerTautology{}},
		&NoVarAssign{ProduceTriggerTautology: &ProduceTriggerTautology{}},
		&BlankVarReturn{ProduceTriggerTautology: &ProduceTriggerTautology{}},
		&FuncParam{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&MethodRecv{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&MethodRecvDeep{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&VariadicFuncParam{ProduceTriggerTautology: &ProduceTriggerTautology{}},
		&TrustedFuncNilable{ProduceTriggerTautology: &ProduceTriggerTautology{}},
		&TrustedFuncNonnil{ProduceTriggerNever: &ProduceTriggerNever{}},
		&FldRead{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&ParamFldRead{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&FldReturn{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&FuncReturn{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&MethodReturn{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&MethodResultReachesInterface{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&InterfaceParamReachesImplementation{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&GlobalVarRead{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&MapRead{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&ArrayRead{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&SliceRead{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&PtrRead{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&ChanRecv{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&FuncParamDeep{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&VariadicFuncParamDeep{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
		&FuncReturnDeep{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&FldReadDeep{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&LocalVarReadDeep{ProduceTriggerNever: &ProduceTriggerNever{}},
		&GlobalVarReadDeep{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&GuardMissing{ProduceTriggerTautology: &ProduceTriggerTautology{}, OldAnnotation: mockedProducingAnnotationTrigger},
	}
}

// TestProducingAnnotationTriggerEqualsSuite runs the test suite for the `equals` method of all the structs that implement
// the `ProducingAnnotationTrigger` interface.
func TestProducingAnnotationTriggerEqualsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ProducingAnnotationTriggerTestSuite))
}
