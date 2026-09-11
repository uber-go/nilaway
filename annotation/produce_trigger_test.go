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
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/nilaway/nilawaytest"
)

const _interfaceNameProducingAnnotationTrigger = "ProducingAnnotationTrigger"

// initStructsProducingAnnotationTrigger initializes all structs that implement the ProducingAnnotationTrigger interface.
func initStructsProducingAnnotationTrigger() []ProducingAnnotationTrigger {
	mockedKey := new(mockKey)
	mockedKey.On("equals", mock.Anything).Return(true)

	mockedProducingAnnotationTrigger := new(mockProducingAnnotationTrigger)
	mockedProducingAnnotationTrigger.On("equals", mock.Anything).Return(true)

	return []ProducingAnnotationTrigger{
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
		&LocalVarReadDeep{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&GlobalVarReadDeep{TriggerIfDeepNilable: &TriggerIfDeepNilable{Ann: mockedKey}},
		&GuardMissing{ProduceTriggerTautology: &ProduceTriggerTautology{}, OldAnnotation: mockedProducingAnnotationTrigger},
		&StructFieldNil{ProduceTriggerTautology: &ProduceTriggerTautology{}},
		&StructFieldFromContext{TriggerIfNilable: &TriggerIfNilable{Ann: mockedKey}},
	}
}

// TestProducingAnnotationTriggerEqualsSuite runs the test suite for the `equals` method of all the structs that implement
// the `ProducingAnnotationTrigger` interface.
func TestProducingAnnotationTriggerEqualsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, nilawaytest.NewEqualsTestSuite(
		_interfaceNameProducingAnnotationTrigger,
		".",
		initStructsProducingAnnotationTrigger(),
		func(p1, p2 ProducingAnnotationTrigger) bool { return p1.equals(p2) },
	))
}

func TestProducingAnnotationTriggerRepr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		trigger ProducingAnnotationTrigger
		want    string
	}{
		{name: "nilable", trigger: &TriggerIfNilable{}, want: "nilable value"},
		{name: "deep nilable", trigger: &TriggerIfDeepNilable{}, want: "deeply nilable value"},
		{name: "tautology", trigger: &ProduceTriggerTautology{}, want: "nilable value"},
		{name: "never", trigger: &ProduceTriggerNever{}, want: "is not nilable"},
		{name: "positive nil check", trigger: &PositiveNilCheck{ProduceTriggerTautology: &ProduceTriggerTautology{}}, want: "determined nil via conditional check"},
		{name: "negative nil check", trigger: &NegativeNilCheck{ProduceTriggerNever: &ProduceTriggerNever{}}, want: "determined nonnil via conditional check"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.trigger.Repr().String())
		})
	}
}

func TestProducingAnnotationTriggerNeedsGuard(t *testing.T) {
	t.Parallel()

	tests := []ProducingAnnotationTrigger{
		&TriggerIfNilable{},
		&TriggerIfDeepNilable{},
		&ProduceTriggerTautology{},
		&ProduceTriggerNever{},
	}
	for _, trigger := range tests {
		trigger.SetNeedsGuard(true)
		require.True(t, trigger.NeedsGuardMatch())

		trigger.SetNeedsGuard(false)
		require.False(t, trigger.NeedsGuardMatch())
	}
}
