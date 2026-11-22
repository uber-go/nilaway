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
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/nilaway/nilawaytest"
)

const _interfaceNameConsumingAnnotationTrigger = "ConsumingAnnotationTrigger"

// initStructsConsumingAnnotationTrigger initializes all structs that implement the ConsumingAnnotationTrigger interface
var initStructsConsumingAnnotationTrigger = []ConsumingAnnotationTrigger{
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
	&ArgPassDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&UseAsReturnDeep{TriggerIfDeepNonNil: &TriggerIfDeepNonNil{Ann: newMockKey()}},
	&StructFieldToContext{TriggerIfNonNil: &TriggerIfNonNil{Ann: newMockKey()}},
}

// TestConsumingAnnotationTriggerEqualsSuite tests the `equals` method of all structs implementing
// the `ConsumingAnnotationTrigger` interface.
func TestConsumingAnnotationTriggerEqualsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, nilawaytest.NewEqualsTestSuite(
		_interfaceNameConsumingAnnotationTrigger,
		".",
		initStructsConsumingAnnotationTrigger,
		func(c1, c2 ConsumingAnnotationTrigger) bool { return c1.equals(c2) },
	))
}

// TestConsumingAnnotationTriggerCopySuite tests the `Copy` method of all structs implementing
// the `ConsumingAnnotationTrigger` interface.
func TestConsumingAnnotationTriggerCopySuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, nilawaytest.NewCopyTestSuite(
		_interfaceNameConsumingAnnotationTrigger,
		".",
		initStructsConsumingAnnotationTrigger,
		func(c ConsumingAnnotationTrigger) ConsumingAnnotationTrigger { return c.Copy() },
	))
}

func TestConsumingAnnotationTriggerCopyAssignmentHistory(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		trigger ConsumingAnnotationTrigger
		prefix  string
	}{
		{"conditional", &TriggerIfNonNil{Ann: newMockKey()}, "nonnil value"},
		{"deep conditional", &TriggerIfDeepNonNil{Ann: newMockKey()}, "deeply nonnil value"},
		{"tautology", &ConsumeTriggerTautology{}, "must be nonnil"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			original := tt.trigger
			original.AddAssignment(Assignment{
				LHSExprStr: "sink", RHSExprStr: "v",
				Position: token.Position{Filename: "flow.go", Line: 8, Column: 2},
			})
			copied := original.Copy()
			require.Equal(t, original.Repr().String(), copied.Repr().String())

			original.AddAssignment(Assignment{
				LHSExprStr: "v", RHSExprStr: "left",
				Position: token.Position{Filename: "flow.go", Line: 4, Column: 3},
			})
			copied.AddAssignment(Assignment{
				LHSExprStr: "v", RHSExprStr: "right",
				Position: token.Position{Filename: "flow.go", Line: 6, Column: 3},
			})

			const header = " via the assignment(s):\n\t\t- "
			const tail = ",\n\t\t- `v` to `sink` at flow.go:8:2"
			require.Equal(t, tt.prefix+header+"`left` to `v` at flow.go:4:3"+tail, original.Repr().String())
			require.Equal(t, tt.prefix+header+"`right` to `v` at flow.go:6:3"+tail, copied.Repr().String())
			require.True(t, original.equals(copied), "assignment history must not affect trigger equality")
		})
	}
}

func TestAssignmentFlowRendering(t *testing.T) {
	t.Parallel()
	var flow assignmentFlow
	require.Equal(t, "", flow.String())

	sink := Assignment{"c", "b", token.Position{Filename: "flow.go", Line: 9, Column: 2}}
	middle := Assignment{"b", "a", token.Position{Filename: "flow.go", Line: 8, Column: 2}}
	source := Assignment{"a", "input", token.Position{Filename: "flow.go", Line: 7, Column: 2}}
	flow.addEntry(sink)
	flow.addEntry(middle)
	flow.addEntry(source)
	flow.addEntry(middle) // A duplicate away from the head must not move to the front.

	require.Equal(t, " via the assignment(s):\n\t\t- `input` to `a` at flow.go:7:2,"+
		"\n\t\t- `a` to `b` at flow.go:8:2,\n\t\t- `b` to `c` at flow.go:9:2", flow.String())
}

func TestAssignmentFlowDeduplicatesExactAssignments(t *testing.T) {
	t.Parallel()
	// Position equality includes the offset, even though the rendered position does not show it.
	pos := token.Position{Filename: "flow.go", Offset: 20, Line: 2, Column: 2}
	otherPos := pos
	otherPos.Offset++
	var flow assignmentFlow
	flow.addEntry(Assignment{"x", "y", pos})
	flow.addEntry(Assignment{"x", "y", otherPos})
	flow.addEntry(Assignment{"z", "y", pos})
	flow.addEntry(Assignment{"x", "z", pos})
	flow.addEntry(Assignment{"x", "y", pos})

	require.Equal(t, 4, strings.Count(flow.String(), "\n\t\t- "))
}
