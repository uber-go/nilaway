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
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/nilaway/guard"
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

func TestConsumingAnnotationTriggerRepr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		trigger ConsumingAnnotationTrigger
		want    string
	}{
		{name: "nonnil", trigger: &TriggerIfNonNil{}, want: "nonnil value"},
		{name: "deep nonnil", trigger: &TriggerIfDeepNonNil{}, want: "deeply nonnil value"},
		{name: "tautology", trigger: &ConsumeTriggerTautology{}, want: "must be nonnil"},
		{name: "pointer load", trigger: &PtrLoad{ConsumeTriggerTautology: &ConsumeTriggerTautology{}}, want: "dereferenced"},
		{name: "map access", trigger: &MapAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}}, want: "keyed into"},
		{name: "slice access", trigger: &SliceAccess{ConsumeTriggerTautology: &ConsumeTriggerTautology{}}, want: "sliced into"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, tt.trigger.Repr().String())
		})
	}
}

func TestConsumingAnnotationTriggerNeedsGuard(t *testing.T) {
	t.Parallel()

	tests := []ConsumingAnnotationTrigger{
		&TriggerIfNonNil{},
		&TriggerIfDeepNonNil{},
		&ConsumeTriggerTautology{},
	}
	for _, trigger := range tests {
		trigger.SetNeedsGuard(false)
		require.False(t, trigger.NeedsGuard())

		trigger.SetNeedsGuard(true)
		require.True(t, trigger.NeedsGuard())
	}
}

func TestConsumingAnnotationTriggerReprIncludesAssignments(t *testing.T) {
	t.Parallel()

	trigger := &TriggerIfNonNil{}

	trigger.AddAssignment(Assignment{LHSExprStr: "lhs", RHSExprStr: "rhs"})
	require.Equal(t, "nonnil value via the assignment(s):\n\t\t- `rhs` to `lhs` at -", trigger.Repr().String())
}

func TestConsumeTriggerEqualsAndCopy(t *testing.T) {
	t.Parallel()

	expr := &ast.Ident{NamePos: token.Pos(1)}
	trigger := &ConsumeTrigger{
		Annotation:   &ConsumeTriggerTautology{},
		Expr:         expr,
		Guards:       guard.NonceSet{guard.Nonce(1): true},
		GuardMatched: true,
	}

	copyTrigger := trigger.Copy()

	require.True(t, trigger.equals(copyTrigger))
	require.NotSame(t, trigger, copyTrigger)
	require.NotSame(t, trigger.Annotation, copyTrigger.Annotation)

	copyTrigger.Guards.Remove(guard.Nonce(1))
	require.True(t, trigger.Guards.Contains(guard.Nonce(1)))

	copyTrigger.Expr = &ast.Ident{NamePos: token.Pos(2)}
	require.False(t, trigger.equals(copyTrigger))
}

func TestConsumeTriggerPos(t *testing.T) {
	t.Parallel()

	expr := &ast.Ident{NamePos: token.Pos(7)}
	trigger := &ConsumeTrigger{Annotation: &ConsumeTriggerTautology{}, Expr: expr}

	require.Equal(t, expr.Pos(), trigger.Pos())
}

func TestMergeConsumeTriggerSlices(t *testing.T) {
	t.Parallel()

	expr := &ast.Ident{}
	left := &ConsumeTrigger{
		Annotation:   &ConsumeTriggerTautology{},
		Expr:         expr,
		Guards:       guard.NonceSet{guard.Nonce(1): true, guard.Nonce(2): true},
		GuardMatched: true,
	}
	right := &ConsumeTrigger{
		Annotation:   &ConsumeTriggerTautology{},
		Expr:         expr,
		Guards:       guard.NonceSet{guard.Nonce(2): true, guard.Nonce(3): true},
		GuardMatched: false,
	}

	merged := MergeConsumeTriggerSlices([]*ConsumeTrigger{left}, []*ConsumeTrigger{right})
	require.Len(t, merged, 1)
	require.Equal(t, guard.NonceSet{guard.Nonce(2): true}, merged[0].Guards)
	require.False(t, merged[0].GuardMatched)
}

func TestConsumeTriggerSliceAsGuarded(t *testing.T) {
	t.Parallel()

	expr := &ast.Ident{}
	trigger := &ConsumeTrigger{
		Annotation: &ConsumeTriggerTautology{},
		Expr:       expr,
		Guards:     guard.NonceSet{guard.Nonce(1): true},
	}

	guarded := ConsumeTriggerSliceAsGuarded([]*ConsumeTrigger{trigger}, guard.Nonce(2))[0]
	require.NotSame(t, trigger, guarded)
	require.Same(t, expr, guarded.Expr)
	require.Equal(t, guard.NonceSet{guard.Nonce(1): true, guard.Nonce(2): true}, guarded.Guards)
	require.Equal(t, guard.NonceSet{guard.Nonce(1): true}, trigger.Guards)
}

func TestConsumeTriggerSlicesEq(t *testing.T) {
	t.Parallel()

	expr := &ast.Ident{}
	left := &ConsumeTrigger{Annotation: &ConsumeTriggerTautology{}, Expr: expr}
	right := &ConsumeTrigger{Annotation: &ConsumeTriggerTautology{}, Expr: expr}

	require.True(t, ConsumeTriggerSlicesEq(nil, nil))
	require.True(t, ConsumeTriggerSlicesEq([]*ConsumeTrigger{left}, []*ConsumeTrigger{right}))
	require.False(t, ConsumeTriggerSlicesEq([]*ConsumeTrigger{left}, nil))

	right.GuardMatched = true
	require.False(t, ConsumeTriggerSlicesEq([]*ConsumeTrigger{left}, []*ConsumeTrigger{right}))
}
