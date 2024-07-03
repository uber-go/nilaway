//	Copyright (c) 2023 Uber Technologies, Inc.
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
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"go.uber.org/nilaway/util"
	"go.uber.org/nilaway/util/orderedmap"
)

// A ConsumingAnnotationTrigger indicated a possible reason that a nil flow to this site would indicate
// an error
//
// All ConsumingAnnotationTriggers must embed one of the following 3 structs:
// -TriggerIfNonnil
// -TriggerIfDeepNonnil
// -ConsumeTriggerTautology
type ConsumingAnnotationTrigger interface {
	// CheckConsume can be called to determined whether this trigger should be triggered
	// given a particular Annotation map
	// for example - an `ArgPass` trigger triggers iff the corresponding function arg has
	// nonNil type
	CheckConsume(Map) bool
	Prestring() Prestring

	// Kind returns the kind of the trigger.
	Kind() TriggerKind

	// UnderlyingSite returns the underlying site this trigger's nilability depends on. If the
	// trigger always or never fires, the site is nil.
	UnderlyingSite() Key

	customPos() (token.Pos, bool)

	// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
	equals(ConsumingAnnotationTrigger) bool

	// Copy returns a deep copy of this ConsumingAnnotationTrigger
	Copy() ConsumingAnnotationTrigger

	// AddAssignment adds an assignment to the trigger for tracking and printing informative error message.
	// NilAway's `backpropAcrossOneToOneAssignment()` lifts consumer triggers from the RHS of an assignment to the LHS.
	// This implies loss of information about the assignment. This method is used to track such assignments and print
	// a more informative error message.
	AddAssignment(Assignment)

	// NeedsGuard returns true if the trigger needs to be guarded, for example, by a nil check or an ok form.
	NeedsGuard() bool

	// SetNeedsGuard sets the underlying Guard-Neediness of this ConsumerTrigger, if present.
	// Default setting for ConsumerTriggers is that they need a guard. Override this method to set the need for a guard to false.
	SetNeedsGuard(bool)
}

// Prestring is an interface used to encode objects that have compact on-the-wire encodings
// (via gob) but can still be expanded into verbose string representations on demand using
// type information. These are key for compact encoding of InferredAnnotationMaps
type Prestring interface {
	String() string
}

// Assignment is a struct that represents an assignment to an expression
type Assignment struct {
	LHSExprStr string
	RHSExprStr string
	Position   token.Position
}

func (a *Assignment) String() string {
	return fmt.Sprintf("`%s` to `%s` at %s", a.RHSExprStr, a.LHSExprStr, a.Position)
}

// assignmentFlow is a struct that represents a flow of assignments.
// Note that we implement a copy method for this struct, since we want to deep copy the assignments map when we copy
// ConsumerTriggers. However, we don't implement an `equals` method for this struct, since it would incur a performance
// penalty in situations where multiple nilable flows reach a dereference site by creating more full triggers and possibly
// more rounds through backpropagation fix point. Consider the following example:
//
//	func f(m map[int]*int) {
//	  var v *int
//	  var ok1, ok2 bool
//	  if cond {
//	    v, ok1 = m[0] // nilable flow 1, ok1 is false
//	  } else {
//	    v, ok2 = m[1] // nilable flow 2, ok2 is false
//	  }
//	  _, _ = ok1, ok2
//	  _ = *v // nil panic!
//	}
//
// Here `v` can be potentiall nilable from two flows: ok1 or ok2 is false. We would like to print only one error message
// for this situation with one representative flow printed in the error message. However, with an `equals` method, we would
// report multiple error messages, one for each flow, by creating multiple full triggers, thereby affecting performance.
type assignmentFlow struct {
	// We use ordered map for `assignments` to maintain the order of assignments in the flow, and also to avoid
	// duplicates that can get introduced due to fix point convergence in backpropagation.
	assignments *orderedmap.OrderedMap[Assignment, bool]
}

func (a *assignmentFlow) addEntry(entry Assignment) {
	if a.assignments == nil {
		a.assignments = orderedmap.New[Assignment, bool]()
	}
	a.assignments.Store(entry, true)
}

func (a *assignmentFlow) copy() assignmentFlow {
	if a.assignments == nil {
		return assignmentFlow{}
	}
	assignments := orderedmap.New[Assignment, bool]()
	for _, p := range a.assignments.Pairs {
		assignments.Store(p.Key, true)
	}
	return assignmentFlow{assignments: assignments}
}

func (a *assignmentFlow) String() string {
	if a.assignments == nil || len(a.assignments.Pairs) == 0 {
		return ""
	}

	// backprop algorithm populates assignment entries in backward order. Reverse entries to get forward order of
	// assignments, and store in `strs` slice.
	strs := make([]string, 0, len(a.assignments.Pairs))
	for i := len(a.assignments.Pairs) - 1; i >= 0; i-- {
		strs = append(strs, a.assignments.Pairs[i].Key.String())
	}

	// build the informative print string tracking the assignments
	var sb strings.Builder
	sb.WriteString(" via the assignment(s):\n\t\t- ")
	sb.WriteString(strings.Join(strs, ",\n\t\t- "))
	return sb.String()
}

// TriggerIfNonNil is triggered if the contained Annotation is non-nil
type TriggerIfNonNil struct {
	Ann              Key
	IsGuardNotNeeded bool // ConsumeTriggers need guards by default, when applicable. Set this to true when guards are not needed.
	assignmentFlow
}

// Kind returns Conditional.
func (*TriggerIfNonNil) Kind() TriggerKind { return Conditional }

// UnderlyingSite the underlying site this trigger's nilability depends on.
func (t *TriggerIfNonNil) UnderlyingSite() Key { return t.Ann }

// CheckConsume returns true if the underlying annotation is present in the passed map and nonnil
func (t *TriggerIfNonNil) CheckConsume(annMap Map) bool {
	ann, ok := t.Ann.Lookup(annMap)
	return ok && !ann.IsNilable
}

// customPos has the below default implementation for TriggerIfNonNil, in which case ConsumeTrigger.Pos() will return a default value.
// To return non-default position values, this method should be overridden appropriately.
func (*TriggerIfNonNil) customPos() (token.Pos, bool) { return token.NoPos, false }

// NeedsGuard is the default implementation for TriggerIfNonNil. To return non-default value, this method should be overridden.
func (t *TriggerIfNonNil) NeedsGuard() bool { return !t.IsGuardNotNeeded }

// SetNeedsGuard sets the underlying Guard-Neediness of this ConsumerTrigger
func (t *TriggerIfNonNil) SetNeedsGuard(b bool) {
	t.IsGuardNotNeeded = !b
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (t *TriggerIfNonNil) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*TriggerIfNonNil); ok {
		return t.Ann.equals(other.Ann) && t.IsGuardNotNeeded == other.IsGuardNotNeeded
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (t *TriggerIfNonNil) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *t
	copyConsumer.Ann = t.Ann.copy()
	copyConsumer.assignmentFlow = t.assignmentFlow.copy()
	return &copyConsumer
}

// AddAssignment adds an assignment to the trigger.
func (t *TriggerIfNonNil) AddAssignment(e Assignment) {
	t.assignmentFlow.addEntry(e)
}

// Prestring returns this Prestring as a Prestring
func (t *TriggerIfNonNil) Prestring() Prestring {
	return TriggerIfNonNilPrestring{
		AssignmentStr: t.assignmentFlow.String(),
	}
}

// TriggerIfNonNilPrestring is a Prestring storing the needed information to compactly encode a TriggerIfNonNil
type TriggerIfNonNilPrestring struct {
	AssignmentStr string
}

func (t TriggerIfNonNilPrestring) String() string {
	var sb strings.Builder
	sb.WriteString("nonnil value")
	sb.WriteString(t.AssignmentStr)
	return sb.String()
}

// TriggerIfDeepNonNil is triggered if the contained Annotation is deeply non-nil
type TriggerIfDeepNonNil struct {
	Ann              Key
	IsGuardNotNeeded bool // ConsumeTriggers need guards by default, when applicable. Set this to true when guards are not needed.
	assignmentFlow
}

// Kind returns DeepConditional.
func (*TriggerIfDeepNonNil) Kind() TriggerKind { return DeepConditional }

// UnderlyingSite the underlying site this trigger's nilability depends on.
func (t *TriggerIfDeepNonNil) UnderlyingSite() Key { return t.Ann }

// CheckConsume returns true if the underlying annotation is present in the passed map and deeply nonnil
func (t *TriggerIfDeepNonNil) CheckConsume(annMap Map) bool {
	ann, ok := t.Ann.Lookup(annMap)
	return ok && !ann.IsDeepNilable
}

// customPos has the below default implementation for TriggerIfDeepNonNil, in which case ConsumeTrigger.Pos() will return a default value.
// To return non-default position values, this method should be overridden appropriately.
func (*TriggerIfDeepNonNil) customPos() (token.Pos, bool) { return token.NoPos, false }

// NeedsGuard default implementation for TriggerIfDeepNonNil. To return non-default value, this method should be overridden.
func (t *TriggerIfDeepNonNil) NeedsGuard() bool { return !t.IsGuardNotNeeded }

// SetNeedsGuard sets the underlying Guard-Neediness of this ConsumerTrigger
func (t *TriggerIfDeepNonNil) SetNeedsGuard(b bool) {
	t.IsGuardNotNeeded = !b
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (t *TriggerIfDeepNonNil) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*TriggerIfDeepNonNil); ok {
		return t.Ann.equals(other.Ann) && t.IsGuardNotNeeded == other.IsGuardNotNeeded
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (t *TriggerIfDeepNonNil) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *t
	copyConsumer.Ann = t.Ann.copy()
	copyConsumer.assignmentFlow = t.assignmentFlow.copy()
	return &copyConsumer
}

// AddAssignment adds an assignment to the trigger.
func (t *TriggerIfDeepNonNil) AddAssignment(e Assignment) {
	t.assignmentFlow.addEntry(e)
}

// Prestring returns this Prestring as a Prestring
func (t *TriggerIfDeepNonNil) Prestring() Prestring {
	return TriggerIfDeepNonNilPrestring{
		AssignmentStr: t.assignmentFlow.String(),
	}
}

// TriggerIfDeepNonNilPrestring is a Prestring storing the needed information to compactly encode a TriggerIfDeepNonNil
type TriggerIfDeepNonNilPrestring struct {
	AssignmentStr string
}

func (t TriggerIfDeepNonNilPrestring) String() string {
	var sb strings.Builder
	sb.WriteString("deeply nonnil value")
	sb.WriteString(t.AssignmentStr)
	return sb.String()
}

// ConsumeTriggerTautology is used at consumption sites were consuming nil is always an error
type ConsumeTriggerTautology struct {
	IsGuardNotNeeded bool // ConsumeTriggers need guards by default, when applicable. Set this to true when guards are not needed.
	assignmentFlow
}

// Kind returns Always.
func (*ConsumeTriggerTautology) Kind() TriggerKind { return Always }

// UnderlyingSite always returns nil.
func (*ConsumeTriggerTautology) UnderlyingSite() Key { return nil }

// CheckConsume returns true
func (*ConsumeTriggerTautology) CheckConsume(Map) bool { return true }

// customPos has the below default implementation for ConsumeTriggerTautology, in which case ConsumeTrigger.Pos() will return a default value.
// To return non-default position values, this method should be overridden appropriately.
func (*ConsumeTriggerTautology) customPos() (token.Pos, bool) { return token.NoPos, false }

// NeedsGuard default implementation for ConsumeTriggerTautology. To return non-default value, this method should be overridden.
func (c *ConsumeTriggerTautology) NeedsGuard() bool { return !c.IsGuardNotNeeded }

// SetNeedsGuard sets the underlying Guard-Neediness of this ConsumerTrigger
func (c *ConsumeTriggerTautology) SetNeedsGuard(b bool) {
	c.IsGuardNotNeeded = !b
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (c *ConsumeTriggerTautology) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*ConsumeTriggerTautology); ok {
		return c.IsGuardNotNeeded == other.IsGuardNotNeeded
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (c *ConsumeTriggerTautology) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *c
	copyConsumer.assignmentFlow = c.assignmentFlow.copy()
	return &copyConsumer
}

// AddAssignment adds an assignment to the trigger.
func (c *ConsumeTriggerTautology) AddAssignment(e Assignment) {
	c.assignmentFlow.addEntry(e)
}

// Prestring returns this Prestring as a Prestring
func (c *ConsumeTriggerTautology) Prestring() Prestring {
	return ConsumeTriggerTautologyPrestring{
		AssignmentStr: c.assignmentFlow.String(),
	}
}

// ConsumeTriggerTautologyPrestring is a Prestring storing the needed information to compactly encode a ConsumeTriggerTautology
type ConsumeTriggerTautologyPrestring struct {
	AssignmentStr string
}

func (c ConsumeTriggerTautologyPrestring) String() string {
	var sb strings.Builder
	sb.WriteString("must be nonnil")
	sb.WriteString(c.AssignmentStr)
	return sb.String()
}

// PtrLoad is when a value flows to a point where it is loaded as a pointer
type PtrLoad struct {
	*ConsumeTriggerTautology
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (p *PtrLoad) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*PtrLoad); ok {
		return p.ConsumeTriggerTautology.equals(other.ConsumeTriggerTautology)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (p *PtrLoad) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *p
	copyConsumer.ConsumeTriggerTautology = p.ConsumeTriggerTautology.Copy().(*ConsumeTriggerTautology)
	return &copyConsumer
}

// Prestring returns this PtrLoad as a Prestring
func (p *PtrLoad) Prestring() Prestring {
	return PtrLoadPrestring{
		AssignmentStr: p.assignmentFlow.String(),
	}
}

// PtrLoadPrestring is a Prestring storing the needed information to compactly encode a PtrLoad
type PtrLoadPrestring struct {
	AssignmentStr string
}

func (p PtrLoadPrestring) String() string {
	var sb strings.Builder
	sb.WriteString("dereferenced")
	sb.WriteString(p.AssignmentStr)
	return sb.String()
}

// MapAccess is when a map value flows to a point where it is indexed, and thus must be non-nil
//
// note: this trigger is produced only if config.ErrorOnNilableMapRead == true
type MapAccess struct {
	*ConsumeTriggerTautology
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (i *MapAccess) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*MapAccess); ok {
		return i.ConsumeTriggerTautology.equals(other.ConsumeTriggerTautology)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (i *MapAccess) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *i
	copyConsumer.ConsumeTriggerTautology = i.ConsumeTriggerTautology.Copy().(*ConsumeTriggerTautology)
	return &copyConsumer
}

// Prestring returns this MapAccess as a Prestring
func (i *MapAccess) Prestring() Prestring {
	return MapAccessPrestring{
		AssignmentStr: i.assignmentFlow.String(),
	}
}

// MapAccessPrestring is a Prestring storing the needed information to compactly encode a MapAccess
type MapAccessPrestring struct {
	AssignmentStr string
}

func (i MapAccessPrestring) String() string {
	var sb strings.Builder
	sb.WriteString("keyed into")
	sb.WriteString(i.AssignmentStr)
	return sb.String()
}

// MapWrittenTo is when a map value flows to a point where one of its indices is written to, and thus
// must be non-nil
type MapWrittenTo struct {
	*ConsumeTriggerTautology
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (m *MapWrittenTo) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*MapWrittenTo); ok {
		return m.ConsumeTriggerTautology.equals(other.ConsumeTriggerTautology)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (m *MapWrittenTo) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *m
	copyConsumer.ConsumeTriggerTautology = m.ConsumeTriggerTautology.Copy().(*ConsumeTriggerTautology)
	return &copyConsumer
}

// Prestring returns this MapWrittenTo as a Prestring
func (m *MapWrittenTo) Prestring() Prestring {
	return MapWrittenToPrestring{
		AssignmentStr: m.assignmentFlow.String(),
	}
}

// MapWrittenToPrestring is a Prestring storing the needed information to compactly encode a MapWrittenTo
type MapWrittenToPrestring struct {
	AssignmentStr string
}

func (m MapWrittenToPrestring) String() string {
	var sb strings.Builder
	sb.WriteString("written to at an index")
	sb.WriteString(m.AssignmentStr)
	return sb.String()
}

// SliceAccess is when a slice value flows to a point where it is sliced, and thus must be non-nil
type SliceAccess struct {
	*ConsumeTriggerTautology
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (s *SliceAccess) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*SliceAccess); ok {
		return s.ConsumeTriggerTautology.equals(other.ConsumeTriggerTautology)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (s *SliceAccess) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *s
	copyConsumer.ConsumeTriggerTautology = s.ConsumeTriggerTautology.Copy().(*ConsumeTriggerTautology)
	return &copyConsumer
}

// Prestring returns this SliceAccess as a Prestring
func (s *SliceAccess) Prestring() Prestring {
	return SliceAccessPrestring{
		AssignmentStr: s.assignmentFlow.String(),
	}
}

// SliceAccessPrestring is a Prestring storing the needed information to compactly encode a SliceAccess
type SliceAccessPrestring struct {
	AssignmentStr string
}

func (s SliceAccessPrestring) String() string {
	var sb strings.Builder
	sb.WriteString("sliced into")
	sb.WriteString(s.AssignmentStr)
	return sb.String()
}

// FldAccess is when a value flows to a point where a field of it is accessed, and so it must be non-nil
type FldAccess struct {
	*ConsumeTriggerTautology

	Sel types.Object
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *FldAccess) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*FldAccess); ok {
		return f.ConsumeTriggerTautology.equals(other.ConsumeTriggerTautology) && f.Sel == other.Sel
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *FldAccess) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.ConsumeTriggerTautology = f.ConsumeTriggerTautology.Copy().(*ConsumeTriggerTautology)
	return &copyConsumer
}

// Prestring returns this FldAccess as a Prestring
func (f *FldAccess) Prestring() Prestring {
	fieldName, methodName := "", ""
	switch t := f.Sel.(type) {
	case *types.Var:
		fieldName = t.Name()
	case *types.Func:
		methodName = t.Name()
	default:
		panic(fmt.Sprintf("unexpected Sel type %T in FldAccess", t))
	}

	return FldAccessPrestring{
		FieldName:     fieldName,
		MethodName:    methodName,
		AssignmentStr: f.assignmentFlow.String(),
	}
}

// FldAccessPrestring is a Prestring storing the needed information to compactly encode a FldAccess
type FldAccessPrestring struct {
	FieldName     string
	MethodName    string
	AssignmentStr string
}

func (f FldAccessPrestring) String() string {
	var sb strings.Builder
	if f.MethodName != "" {
		sb.WriteString(fmt.Sprintf("called `%s()`", f.MethodName))
	} else {
		sb.WriteString(fmt.Sprintf("accessed field `%s`", f.FieldName))
	}
	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// UseAsErrorResult is when a value flows to the error result of a function, where it is expected to be non-nil
type UseAsErrorResult struct {
	*TriggerIfNonNil

	RetStmt       *ast.ReturnStmt
	IsNamedReturn bool
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (u *UseAsErrorResult) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*UseAsErrorResult); ok {
		return u.TriggerIfNonNil.equals(other.TriggerIfNonNil) &&
			u.RetStmt == other.RetStmt &&
			u.IsNamedReturn == other.IsNamedReturn
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (u *UseAsErrorResult) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *u
	copyConsumer.TriggerIfNonNil = u.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this UseAsErrorResult as a Prestring
func (u *UseAsErrorResult) Prestring() Prestring {
	retAnn := u.Ann.(*RetAnnotationKey)
	return UseAsErrorResultPrestring{
		Pos:              retAnn.RetNum,
		ReturningFuncStr: retAnn.FuncDecl.Name(),
		IsNamedReturn:    u.IsNamedReturn,
		RetName:          retAnn.FuncDecl.Type().(*types.Signature).Results().At(retAnn.RetNum).Name(),
		AssignmentStr:    u.assignmentFlow.String(),
	}
}

// UseAsErrorResultPrestring is a Prestring storing the needed information to compactly encode a UseAsErrorResult
type UseAsErrorResultPrestring struct {
	Pos              int
	ReturningFuncStr string
	IsNamedReturn    bool
	RetName          string
	AssignmentStr    string
}

func (u UseAsErrorResultPrestring) String() string {
	var sb strings.Builder
	if u.IsNamedReturn {
		sb.WriteString(fmt.Sprintf("returned as named error result `%s` of `%s()`", u.RetName, u.ReturningFuncStr))
	} else {
		sb.WriteString(fmt.Sprintf("returned as error result %d of `%s()`", u.Pos, u.ReturningFuncStr))
	}
	sb.WriteString(u.AssignmentStr)
	return sb.String()
}

// overriding position value to point to the raw return statement, which is the source of the potential error
func (u *UseAsErrorResult) customPos() (token.Pos, bool) {
	if u.IsNamedReturn {
		return u.RetStmt.Pos(), true
	}
	return 0, false
}

// FldAssign is when a value flows to a point where it is assigned into a field
type FldAssign struct {
	*TriggerIfNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *FldAssign) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*FldAssign); ok {
		return f.TriggerIfNonNil.equals(other.TriggerIfNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *FldAssign) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.TriggerIfNonNil = f.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this FldAssign as a Prestring
func (f *FldAssign) Prestring() Prestring {
	fldAnn := f.Ann.(*FieldAnnotationKey)
	return FldAssignPrestring{
		FieldName:     fldAnn.FieldDecl.Name(),
		AssignmentStr: f.assignmentFlow.String(),
	}
}

// FldAssignPrestring is a Prestring storing the needed information to compactly encode a FldAssign
type FldAssignPrestring struct {
	FieldName     string
	AssignmentStr string
}

func (f FldAssignPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned into field `%s`", f.FieldName))
	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// ArgFldPass is when a struct field value (A.f) flows to a point where it is passed to a function with a param of
// the same struct type (A)
type ArgFldPass struct {
	*TriggerIfNonNil
	IsPassed bool
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *ArgFldPass) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*ArgFldPass); ok {
		return f.TriggerIfNonNil.equals(other.TriggerIfNonNil) && f.IsPassed == other.IsPassed
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *ArgFldPass) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.TriggerIfNonNil = f.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this ArgFldPass as a Prestring
func (f *ArgFldPass) Prestring() Prestring {
	ann := f.Ann.(*ParamFieldAnnotationKey)
	recvName := ""
	if ann.IsReceiver() {
		recvName = ann.FuncDecl.Type().(*types.Signature).Recv().Name()
	}

	return ArgFldPassPrestring{
		FieldName:     ann.FieldDecl.Name(),
		FuncName:      ann.FuncDecl.Name(),
		ParamNum:      ann.ParamNum,
		RecvName:      recvName,
		IsPassed:      f.IsPassed,
		AssignmentStr: f.assignmentFlow.String(),
	}
}

// ArgFldPassPrestring is a Prestring storing the needed information to compactly encode a ArgFldPass
type ArgFldPassPrestring struct {
	FieldName     string
	FuncName      string
	ParamNum      int
	RecvName      string
	IsPassed      bool
	AssignmentStr string
}

func (f ArgFldPassPrestring) String() string {
	var sb strings.Builder
	prefix := ""
	if f.IsPassed {
		prefix = "assigned to "
	}

	if len(f.RecvName) > 0 {
		sb.WriteString(fmt.Sprintf("%sfield `%s` of method receiver `%s`", prefix, f.FieldName, f.RecvName))
	} else {
		sb.WriteString(fmt.Sprintf("%sfield `%s` of argument %d to `%s()`", prefix, f.FieldName, f.ParamNum, f.FuncName))
	}

	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// GlobalVarAssign is when a value flows to a point where it is assigned into a global variable
type GlobalVarAssign struct {
	*TriggerIfNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (g *GlobalVarAssign) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*GlobalVarAssign); ok {
		return g.TriggerIfNonNil.equals(other.TriggerIfNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (g *GlobalVarAssign) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *g
	copyConsumer.TriggerIfNonNil = g.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this GlobalVarAssign as a Prestring
func (g *GlobalVarAssign) Prestring() Prestring {
	varAnn := g.Ann.(*GlobalVarAnnotationKey)
	return GlobalVarAssignPrestring{
		VarName:       varAnn.VarDecl.Name(),
		AssignmentStr: g.assignmentFlow.String(),
	}
}

// GlobalVarAssignPrestring is a Prestring storing the needed information to compactly encode a GlobalVarAssign
type GlobalVarAssignPrestring struct {
	VarName       string
	AssignmentStr string
}

func (g GlobalVarAssignPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned into global variable `%s`", g.VarName))
	sb.WriteString(g.AssignmentStr)
	return sb.String()
}

// ArgPass is when a value flows to a point where it is passed as an argument to a function. This
// consumer trigger can be used on top of two different sites: ParamAnnotationKey &
// CallSiteParamAnnotationKey. ParamAnnotationKey is the parameter site in the function
// declaration; CallSiteParamAnnotationKey is the argument site in the call expression.
// CallSiteParamAnnotationKey is specifically used for functions with contracts since we need to
// duplicate the sites for context sensitivity.
type ArgPass struct {
	*TriggerIfNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (a *ArgPass) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*ArgPass); ok {
		return a.TriggerIfNonNil.equals(other.TriggerIfNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (a *ArgPass) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *a
	copyConsumer.TriggerIfNonNil = a.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this ArgPass as a Prestring
func (a *ArgPass) Prestring() Prestring {
	switch key := a.Ann.(type) {
	case *ParamAnnotationKey:
		return ArgPassPrestring{
			ParamName:     key.MinimalString(),
			FuncName:      key.FuncDecl.Name(),
			Location:      "",
			AssignmentStr: a.assignmentFlow.String(),
		}
	case *CallSiteParamAnnotationKey:
		return ArgPassPrestring{
			ParamName:     key.MinimalString(),
			FuncName:      key.FuncDecl.Name(),
			Location:      key.Location.String(),
			AssignmentStr: a.assignmentFlow.String(),
		}
	default:
		panic(fmt.Sprintf(
			"Expected ParamAnnotationKey or CallSiteParamAnnotationKey but got: %T", key))
	}
}

// ArgPassPrestring is a Prestring storing the needed information to compactly encode a ArgPass
type ArgPassPrestring struct {
	ParamName string
	FuncName  string
	// Location points to the code location of the argument pass at the call site for a ArgPass
	// enclosing CallSiteParamAnnotationKey; Location is empty for a ArgPass enclosing ParamAnnotationKey.
	Location      string
	AssignmentStr string
}

func (a ArgPassPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("passed as %s to `%s()`", a.ParamName, a.FuncName))
	if a.Location != "" {
		sb.WriteString(fmt.Sprintf(" at %s", a.Location))
	}
	sb.WriteString(a.AssignmentStr)
	return sb.String()
}

// ArgPassDeep is when a value deeply flows to a point where it is passed as an argument to a function
type ArgPassDeep struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (a *ArgPassDeep) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*ArgPassDeep); ok {
		return a.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (a *ArgPassDeep) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *a
	copyConsumer.TriggerIfDeepNonNil = a.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this ArgPassDeep as a Prestring
func (a *ArgPassDeep) Prestring() Prestring {
	switch key := a.Ann.(type) {
	case *ParamAnnotationKey:
		return ArgPassPrestring{
			ParamName:     key.MinimalString(),
			FuncName:      key.FuncDecl.Name(),
			Location:      "",
			AssignmentStr: a.assignmentFlow.String(),
		}
	case *CallSiteParamAnnotationKey:
		return ArgPassPrestring{
			ParamName:     key.MinimalString(),
			FuncName:      key.FuncDecl.Name(),
			Location:      key.Location.String(),
			AssignmentStr: a.assignmentFlow.String(),
		}
	default:
		panic(fmt.Sprintf(
			"Expected ParamAnnotationKey or CallSiteParamAnnotationKey but got: %T", key))
	}
}

// ArgPassDeepPrestring is a Prestring storing the needed information to compactly encode a ArgPassDeep
type ArgPassDeepPrestring struct {
	ParamName string
	FuncName  string
	// Location points to the code location of the argument pass at the call site for a ArgPass
	// enclosing CallSiteParamAnnotationKey; Location is empty for a ArgPass enclosing ParamAnnotationKey.
	Location      string
	AssignmentStr string
}

func (a ArgPassDeepPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("passed deeply as %s to `%s()`", a.ParamName, a.FuncName))
	if a.Location != "" {
		sb.WriteString(fmt.Sprintf(" at %s", a.Location))
	}
	sb.WriteString(a.AssignmentStr)
	return sb.String()
}

// RecvPass is when a receiver value flows to a point where it is used to invoke a method.
// E.g., `s.foo()`, here `s` is a receiver and forms the RecvPass Consumer
type RecvPass struct {
	*TriggerIfNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (a *RecvPass) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*RecvPass); ok {
		return a.TriggerIfNonNil.equals(other.TriggerIfNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (a *RecvPass) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *a
	copyConsumer.TriggerIfNonNil = a.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this RecvPass as a Prestring
func (a *RecvPass) Prestring() Prestring {
	recvAnn := a.Ann.(*RecvAnnotationKey)
	return RecvPassPrestring{
		FuncName:      recvAnn.FuncDecl.Name(),
		AssignmentStr: a.assignmentFlow.String(),
	}
}

// RecvPassPrestring is a Prestring storing the needed information to compactly encode a RecvPass
type RecvPassPrestring struct {
	FuncName      string
	AssignmentStr string
}

func (a RecvPassPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("used as receiver to call `%s()`", a.FuncName))
	sb.WriteString(a.AssignmentStr)
	return sb.String()
}

// InterfaceResultFromImplementation is when a result is determined to flow from a concrete method to an interface method via implementation
type InterfaceResultFromImplementation struct {
	*TriggerIfNonNil
	AffiliationPair
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (i *InterfaceResultFromImplementation) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*InterfaceResultFromImplementation); ok {
		return i.TriggerIfNonNil.equals(other.TriggerIfNonNil) &&
			i.AffiliationPair.InterfaceMethod == other.AffiliationPair.InterfaceMethod &&
			i.AffiliationPair.ImplementingMethod == other.AffiliationPair.ImplementingMethod
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (i *InterfaceResultFromImplementation) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *i
	copyConsumer.TriggerIfNonNil = i.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this InterfaceResultFromImplementation as a Prestring
func (i *InterfaceResultFromImplementation) Prestring() Prestring {
	retAnn := i.Ann.(*RetAnnotationKey)
	return InterfaceResultFromImplementationPrestring{
		retAnn.RetNum,
		util.PartiallyQualifiedFuncName(retAnn.FuncDecl),
		util.PartiallyQualifiedFuncName(i.ImplementingMethod),
		i.assignmentFlow.String(),
	}
}

// InterfaceResultFromImplementationPrestring is a Prestring storing the needed information to compactly encode a InterfaceResultFromImplementation
type InterfaceResultFromImplementationPrestring struct {
	RetNum        int
	IntName       string
	ImplName      string
	AssignmentStr string
}

func (i InterfaceResultFromImplementationPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("returned as result %d from interface method `%s()` (implemented by `%s()`)",
		i.RetNum, i.IntName, i.ImplName))
	sb.WriteString(i.AssignmentStr)
	return sb.String()
}

// MethodParamFromInterface is when a param flows from an interface method to a concrete method via implementation
type MethodParamFromInterface struct {
	*TriggerIfNonNil
	AffiliationPair
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (m *MethodParamFromInterface) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*MethodParamFromInterface); ok {
		return m.TriggerIfNonNil.equals(other.TriggerIfNonNil) &&
			m.AffiliationPair.InterfaceMethod == other.AffiliationPair.InterfaceMethod &&
			m.AffiliationPair.ImplementingMethod == other.AffiliationPair.ImplementingMethod
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (m *MethodParamFromInterface) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *m
	copyConsumer.TriggerIfNonNil = m.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this MethodParamFromInterface as a Prestring
func (m *MethodParamFromInterface) Prestring() Prestring {
	paramAnn := m.Ann.(*ParamAnnotationKey)
	return MethodParamFromInterfacePrestring{
		paramAnn.ParamNameString(),
		util.PartiallyQualifiedFuncName(paramAnn.FuncDecl),
		util.PartiallyQualifiedFuncName(m.InterfaceMethod),
		m.assignmentFlow.String(),
	}
}

// MethodParamFromInterfacePrestring is a Prestring storing the needed information to compactly encode a MethodParamFromInterface
type MethodParamFromInterfacePrestring struct {
	ParamName     string
	ImplName      string
	IntName       string
	AssignmentStr string
}

func (m MethodParamFromInterfacePrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("passed as parameter `%s` to `%s()` (implementing `%s()`)",
		m.ParamName, m.ImplName, m.IntName))
	sb.WriteString(m.AssignmentStr)
	return sb.String()
}

// DuplicateReturnConsumer duplicates a given consume trigger, assuming the given consumer trigger
// is for a UseAsReturn annotation.
func DuplicateReturnConsumer(t *ConsumeTrigger, location token.Position) *ConsumeTrigger {
	ann := t.Annotation.(*UseAsReturn)
	key := ann.TriggerIfNonNil.Ann.(*RetAnnotationKey)
	return &ConsumeTrigger{
		Annotation: &UseAsReturn{
			TriggerIfNonNil: &TriggerIfNonNil{
				Ann: NewCallSiteRetKey(key.FuncDecl, key.RetNum, location)},
			IsNamedReturn: ann.IsNamedReturn,
			RetStmt:       ann.RetStmt,
		},
		Expr:         t.Expr,
		Guards:       t.Guards.Copy(), // TODO: probably, we might not need a deep copy all the time
		GuardMatched: t.GuardMatched,
	}
}

// UseAsReturn is when a value flows to a point where it is returned from a function.
// This consumer trigger can be used on top of two different sites: RetAnnotationKey &
// CallSiteRetAnnotationKey. RetAnnotationKey is the parameter site in the function declaration;
// CallSiteRetAnnotationKey is the argument site in the call expression. CallSiteRetAnnotationKey is specifically
// used for functions with contracts since we need to duplicate the sites for context sensitivity.
type UseAsReturn struct {
	*TriggerIfNonNil
	IsNamedReturn        bool
	IsTrackingAlwaysSafe bool
	RetStmt              *ast.ReturnStmt
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (u *UseAsReturn) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*UseAsReturn); ok {
		return u.TriggerIfNonNil.equals(other.TriggerIfNonNil) &&
			u.IsNamedReturn == other.IsNamedReturn &&
			u.IsTrackingAlwaysSafe == other.IsTrackingAlwaysSafe &&
			u.RetStmt == other.RetStmt
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (u *UseAsReturn) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *u
	copyConsumer.TriggerIfNonNil = u.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this UseAsReturn as a Prestring
func (u *UseAsReturn) Prestring() Prestring {
	switch key := u.Ann.(type) {
	case *RetAnnotationKey:
		return UseAsReturnPrestring{
			key.FuncDecl.Name(),
			key.RetNum,
			u.IsNamedReturn,
			key.FuncDecl.Type().(*types.Signature).Results().At(key.RetNum).Name(),
			"",
			u.assignmentFlow.String(),
		}
	case *CallSiteRetAnnotationKey:
		return UseAsReturnPrestring{
			key.FuncDecl.Name(),
			key.RetNum,
			u.IsNamedReturn,
			key.FuncDecl.Type().(*types.Signature).Results().At(key.RetNum).Name(),
			key.Location.String(),
			u.assignmentFlow.String(),
		}
	default:
		panic(fmt.Sprintf("Expected RetAnnotationKey or CallSiteRetAnnotationKey but got: %T", key))
	}
}

// UseAsReturnPrestring is a Prestring storing the needed information to compactly encode a UseAsReturn
type UseAsReturnPrestring struct {
	FuncName      string
	RetNum        int
	IsNamedReturn bool
	RetName       string
	// Location is empty for a UseAsReturn enclosing RetAnnotationKey. Location points to the
	// location of the result at the call site for a UseAsReturn enclosing
	// CallSiteRetAnnotationKey.
	Location      string
	AssignmentStr string
}

func (u UseAsReturnPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("returned from `%s()`", u.FuncName))
	if u.IsNamedReturn {
		sb.WriteString(fmt.Sprintf(" via named return `%s`", u.RetName))
	} else {
		sb.WriteString(fmt.Sprintf(" in position %d", u.RetNum))
	}
	if u.Location != "" {
		sb.WriteString(fmt.Sprintf(" at %s", u.Location))
	}
	sb.WriteString(u.AssignmentStr)
	return sb.String()
}

// overriding position value to point to the raw return statement, which is the source of the potential error
func (u *UseAsReturn) customPos() (token.Pos, bool) {
	if u.IsNamedReturn {
		return u.RetStmt.Pos(), true
	}
	return 0, false
}

// UseAsReturnDeep is when a deep value flows to a point where it is returned from a function.
type UseAsReturnDeep struct {
	*TriggerIfDeepNonNil
	IsNamedReturn bool
	RetStmt       *ast.ReturnStmt
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (u *UseAsReturnDeep) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*UseAsReturnDeep); ok {
		return u.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil) &&
			u.IsNamedReturn == other.IsNamedReturn &&
			u.RetStmt == other.RetStmt
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (u *UseAsReturnDeep) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *u
	copyConsumer.TriggerIfDeepNonNil = u.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this UseAsReturn as a Prestring
func (u *UseAsReturnDeep) Prestring() Prestring {
	key := u.Ann.(*RetAnnotationKey)
	return UseAsReturnDeepPrestring{
		key.FuncDecl.Name(),
		key.RetNum,
		key.FuncDecl.Type().(*types.Signature).Results().At(key.RetNum).Name(),
		u.assignmentFlow.String(),
	}
}

// UseAsReturnDeepPrestring is a Prestring storing the needed information to compactly encode a UseAsReturnDeep
type UseAsReturnDeepPrestring struct {
	FuncName      string
	RetNum        int
	RetName       string
	AssignmentStr string
}

func (u UseAsReturnDeepPrestring) String() string {
	var sb strings.Builder
	via := ""
	if u.RetName != "" && u.RetName != "_" {
		via = fmt.Sprintf(" via named return `%s`", u.RetName)
	}
	sb.WriteString(fmt.Sprintf("returned deeply from `%s()`%s in position %d", u.FuncName, via, u.RetNum))
	sb.WriteString(u.AssignmentStr)
	return sb.String()
}

// overriding position value to point to the raw return statement, which is the source of the potential error
func (u UseAsReturnDeep) customPos() (token.Pos, bool) {
	if u.IsNamedReturn {
		return u.RetStmt.Pos(), true
	}
	return 0, false
}

// UseAsFldOfReturn is when a struct field value (A.f) flows to a point where it is returned from a function with the
// return expression of the same struct type (A)
type UseAsFldOfReturn struct {
	*TriggerIfNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (u *UseAsFldOfReturn) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*UseAsFldOfReturn); ok {
		return u.TriggerIfNonNil.equals(other.TriggerIfNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (u *UseAsFldOfReturn) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *u
	copyConsumer.TriggerIfNonNil = u.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this UseAsFldOfReturn as a Prestring
func (u *UseAsFldOfReturn) Prestring() Prestring {
	retAnn := u.Ann.(*RetFieldAnnotationKey)
	return UseAsFldOfReturnPrestring{
		retAnn.FuncDecl.Name(),
		retAnn.FieldDecl.Name(),
		retAnn.RetNum,
		u.assignmentFlow.String(),
	}
}

// UseAsFldOfReturnPrestring is a Prestring storing the needed information to compactly encode a UseAsFldOfReturn
type UseAsFldOfReturnPrestring struct {
	FuncName      string
	FieldName     string
	RetNum        int
	AssignmentStr string
}

func (u UseAsFldOfReturnPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("field `%s` returned by result %d of `%s()`", u.FieldName, u.RetNum, u.FuncName))
	sb.WriteString(u.AssignmentStr)
	return sb.String()
}

// GetRetFldConsumer returns the UseAsFldOfReturn consume trigger with given retKey and expr
func GetRetFldConsumer(retKey Key, expr ast.Expr) *ConsumeTrigger {
	return &ConsumeTrigger{
		Annotation: &UseAsFldOfReturn{
			TriggerIfNonNil: &TriggerIfNonNil{
				Ann: retKey}},
		Expr:   expr,
		Guards: util.NoGuards(),
	}
}

// GetEscapeFldConsumer returns the FldEscape consume trigger with given escKey and selExpr
func GetEscapeFldConsumer(escKey Key, selExpr ast.Expr) *ConsumeTrigger {
	return &ConsumeTrigger{
		Annotation: &FldEscape{
			TriggerIfNonNil: &TriggerIfNonNil{
				Ann: escKey,
			}},
		Expr:   selExpr,
		Guards: util.NoGuards(),
	}
}

// GetParamFldConsumer returns the ArgFldPass consume trigger with given paramKey and expr
func GetParamFldConsumer(paramKey Key, expr ast.Expr) *ConsumeTrigger {
	return &ConsumeTrigger{
		Annotation: &ArgFldPass{
			TriggerIfNonNil: &TriggerIfNonNil{
				Ann: paramKey},
			IsPassed: true,
		},
		Expr:   expr,
		Guards: util.NoGuards(),
	}
}

// SliceAssign is when a value flows to a point where it is assigned into a slice
type SliceAssign struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *SliceAssign) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*SliceAssign); ok {
		return f.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *SliceAssign) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.TriggerIfDeepNonNil = f.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this SliceAssign as a Prestring
func (f *SliceAssign) Prestring() Prestring {
	fldAnn := f.Ann.(*TypeNameAnnotationKey)
	return SliceAssignPrestring{
		fldAnn.TypeDecl.Name(),
		f.assignmentFlow.String(),
	}
}

// SliceAssignPrestring is a Prestring storing the needed information to compactly encode a SliceAssign
type SliceAssignPrestring struct {
	TypeName      string
	AssignmentStr string
}

func (f SliceAssignPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned into a slice of deeply nonnil type `%s`", f.TypeName))
	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// ArrayAssign is when a value flows to a point where it is assigned into an array
type ArrayAssign struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (a *ArrayAssign) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*ArrayAssign); ok {
		return a.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (a *ArrayAssign) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *a
	copyConsumer.TriggerIfDeepNonNil = a.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this ArrayAssign as a Prestring
func (a *ArrayAssign) Prestring() Prestring {
	fldAnn := a.Ann.(*TypeNameAnnotationKey)
	return ArrayAssignPrestring{
		fldAnn.TypeDecl.Name(),
		a.assignmentFlow.String(),
	}
}

// ArrayAssignPrestring is a Prestring storing the needed information to compactly encode a SliceAssign
type ArrayAssignPrestring struct {
	TypeName      string
	AssignmentStr string
}

func (a ArrayAssignPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned into an array of deeply nonnil type `%s`", a.TypeName))
	sb.WriteString(a.AssignmentStr)
	return sb.String()
}

// PtrAssign is when a value flows to a point where it is assigned into a pointer
type PtrAssign struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *PtrAssign) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*PtrAssign); ok {
		return f.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *PtrAssign) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.TriggerIfDeepNonNil = f.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this PtrAssign as a Prestring
func (f *PtrAssign) Prestring() Prestring {
	fldAnn := f.Ann.(*TypeNameAnnotationKey)
	return PtrAssignPrestring{
		fldAnn.TypeDecl.Name(),
		f.assignmentFlow.String(),
	}
}

// PtrAssignPrestring is a Prestring storing the needed information to compactly encode a PtrAssign
type PtrAssignPrestring struct {
	TypeName      string
	AssignmentStr string
}

func (f PtrAssignPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned into a pointer of deeply nonnil type `%s`", f.TypeName))
	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// MapAssign is when a value flows to a point where it is assigned into an annotated map
type MapAssign struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *MapAssign) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*MapAssign); ok {
		return f.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *MapAssign) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.TriggerIfDeepNonNil = f.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this MapAssign as a Prestring
func (f *MapAssign) Prestring() Prestring {
	fldAnn := f.Ann.(*TypeNameAnnotationKey)
	return MapAssignPrestring{
		fldAnn.TypeDecl.Name(),
		f.assignmentFlow.String(),
	}
}

// MapAssignPrestring is a Prestring storing the needed information to compactly encode a MapAssign
type MapAssignPrestring struct {
	TypeName      string
	AssignmentStr string
}

func (f MapAssignPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned into a map of deeply nonnil type `%s`", f.TypeName))
	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// DeepAssignPrimitive is when a value flows to a point where it is assigned
// deeply into an unnannotated object
type DeepAssignPrimitive struct {
	*ConsumeTriggerTautology
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (d *DeepAssignPrimitive) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*DeepAssignPrimitive); ok {
		return d.ConsumeTriggerTautology.equals(other.ConsumeTriggerTautology)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (d *DeepAssignPrimitive) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *d
	copyConsumer.ConsumeTriggerTautology = d.ConsumeTriggerTautology.Copy().(*ConsumeTriggerTautology)
	return &copyConsumer
}

// Prestring returns this Prestring as a Prestring
func (d *DeepAssignPrimitive) Prestring() Prestring {
	return DeepAssignPrimitivePrestring{
		AssignmentStr: d.assignmentFlow.String(),
	}
}

// DeepAssignPrimitivePrestring is a Prestring storing the needed information to compactly encode a DeepAssignPrimitive
type DeepAssignPrimitivePrestring struct {
	AssignmentStr string
}

func (d DeepAssignPrimitivePrestring) String() string {
	var sb strings.Builder
	sb.WriteString("assigned into a deep type expecting nonnil element type")
	sb.WriteString(d.AssignmentStr)
	return sb.String()
}

// ParamAssignDeep is when a value flows to a point where it is assigned deeply into a function parameter
type ParamAssignDeep struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (p *ParamAssignDeep) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*ParamAssignDeep); ok {
		return p.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (p *ParamAssignDeep) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *p
	copyConsumer.TriggerIfDeepNonNil = p.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this ParamAssignDeep as a Prestring
func (p *ParamAssignDeep) Prestring() Prestring {
	return ParamAssignDeepPrestring{
		p.Ann.(*ParamAnnotationKey).MinimalString(),
		p.assignmentFlow.String(),
	}
}

// ParamAssignDeepPrestring is a Prestring storing the needed information to compactly encode a ParamAssignDeep
type ParamAssignDeepPrestring struct {
	ParamName     string
	AssignmentStr string
}

func (p ParamAssignDeepPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned deeply into parameter %s", p.ParamName))
	sb.WriteString(p.AssignmentStr)
	return sb.String()
}

// FuncRetAssignDeep is when a value flows to a point where it is assigned deeply into a function return
type FuncRetAssignDeep struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *FuncRetAssignDeep) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*FuncRetAssignDeep); ok {
		return f.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *FuncRetAssignDeep) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.TriggerIfDeepNonNil = f.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this FuncRetAssignDeep as a Prestring
func (f *FuncRetAssignDeep) Prestring() Prestring {
	retAnn := f.Ann.(*RetAnnotationKey)
	return FuncRetAssignDeepPrestring{
		retAnn.FuncDecl.Name(),
		retAnn.RetNum,
		f.assignmentFlow.String(),
	}
}

// FuncRetAssignDeepPrestring is a Prestring storing the needed information to compactly encode a FuncRetAssignDeep
type FuncRetAssignDeepPrestring struct {
	FuncName      string
	RetNum        int
	AssignmentStr string
}

func (f FuncRetAssignDeepPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned deeply into the result %d of `%s()`", f.RetNum, f.FuncName))
	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// VariadicParamAssignDeep is when a value flows to a point where it is assigned deeply into a variadic
// function parameter
type VariadicParamAssignDeep struct {
	*TriggerIfNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (v *VariadicParamAssignDeep) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*VariadicParamAssignDeep); ok {
		return v.TriggerIfNonNil.equals(other.TriggerIfNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (v *VariadicParamAssignDeep) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *v
	copyConsumer.TriggerIfNonNil = v.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this VariadicParamAssignDeep as a Prestring
func (v *VariadicParamAssignDeep) Prestring() Prestring {
	paramAnn := v.Ann.(*ParamAnnotationKey)
	return VariadicParamAssignDeepPrestring{
		ParamName:     paramAnn.MinimalString(),
		AssignmentStr: v.assignmentFlow.String(),
	}
}

// VariadicParamAssignDeepPrestring is a Prestring storing the needed information to compactly encode a VariadicParamAssignDeep
type VariadicParamAssignDeepPrestring struct {
	ParamName     string
	AssignmentStr string
}

func (v VariadicParamAssignDeepPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned deeply into variadic parameter `%s`", v.ParamName))
	sb.WriteString(v.AssignmentStr)
	return sb.String()
}

// FieldAssignDeep is when a value flows to a point where it is assigned deeply into a field
type FieldAssignDeep struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *FieldAssignDeep) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*FieldAssignDeep); ok {
		return f.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *FieldAssignDeep) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.TriggerIfDeepNonNil = f.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this FieldAssignDeep as a Prestring
func (f *FieldAssignDeep) Prestring() Prestring {
	fldAnn := f.Ann.(*FieldAnnotationKey)
	return FieldAssignDeepPrestring{
		fldAnn.FieldDecl.Name(),
		f.assignmentFlow.String(),
	}
}

// FieldAssignDeepPrestring is a Prestring storing the needed information to compactly encode a FieldAssignDeep
type FieldAssignDeepPrestring struct {
	FldName       string
	AssignmentStr string
}

func (f FieldAssignDeepPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned deeply into field `%s`", f.FldName))
	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// GlobalVarAssignDeep is when a value flows to a point where it is assigned deeply into a global variable
type GlobalVarAssignDeep struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (g *GlobalVarAssignDeep) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*GlobalVarAssignDeep); ok {
		return g.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (g *GlobalVarAssignDeep) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *g
	copyConsumer.TriggerIfDeepNonNil = g.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this GlobalVarAssignDeep as a Prestring
func (g *GlobalVarAssignDeep) Prestring() Prestring {
	varAnn := g.Ann.(*GlobalVarAnnotationKey)
	return GlobalVarAssignDeepPrestring{
		varAnn.VarDecl.Name(),
		g.assignmentFlow.String(),
	}
}

// GlobalVarAssignDeepPrestring is a Prestring storing the needed information to compactly encode a GlobalVarAssignDeep
type GlobalVarAssignDeepPrestring struct {
	VarName       string
	AssignmentStr string
}

func (g GlobalVarAssignDeepPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned deeply into global variable `%s`", g.VarName))
	sb.WriteString(g.AssignmentStr)
	return sb.String()
}

// LocalVarAssignDeep is when a value flows to a point where it is assigned deeply into a local variable of deeply nonnil type
type LocalVarAssignDeep struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (l *LocalVarAssignDeep) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*LocalVarAssignDeep); ok {
		return l.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (l *LocalVarAssignDeep) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *l
	copyConsumer.TriggerIfDeepNonNil = l.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this LocalVarAssignDeep as a Prestring
func (l *LocalVarAssignDeep) Prestring() Prestring {
	return LocalVarAssignDeepPrestring{
		VarName:       l.Ann.(*LocalVarAnnotationKey).VarDecl.Name(),
		AssignmentStr: l.assignmentFlow.String(),
	}
}

// LocalVarAssignDeepPrestring is a Prestring storing the needed information to compactly encode a LocalVarAssignDeep
type LocalVarAssignDeepPrestring struct {
	VarName       string
	AssignmentStr string
}

func (l LocalVarAssignDeepPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("assigned deeply into local variable `%s`", l.VarName))
	sb.WriteString(l.AssignmentStr)
	return sb.String()
}

// ChanSend is when a value flows to a point where it is sent to a channel
type ChanSend struct {
	*TriggerIfDeepNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (c *ChanSend) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*ChanSend); ok {
		return c.TriggerIfDeepNonNil.equals(other.TriggerIfDeepNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (c *ChanSend) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *c
	copyConsumer.TriggerIfDeepNonNil = c.TriggerIfDeepNonNil.Copy().(*TriggerIfDeepNonNil)
	return &copyConsumer
}

// Prestring returns this ChanSend as a Prestring
func (c *ChanSend) Prestring() Prestring {
	typeAnn := c.Ann.(*TypeNameAnnotationKey)
	return ChanSendPrestring{
		typeAnn.TypeDecl.Name(),
		c.assignmentFlow.String(),
	}
}

// ChanSendPrestring is a Prestring storing the needed information to compactly encode a ChanSend
type ChanSendPrestring struct {
	TypeName      string
	AssignmentStr string
}

func (c ChanSendPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("sent to channel of deeply nonnil type `%s`", c.TypeName))
	sb.WriteString(c.AssignmentStr)
	return sb.String()
}

// FldEscape is when a nilable value flows through a field of a struct that escapes.
// The consumer is added for the fields at sites of escape.
// There are 2 cases, that we currently consider as escaping:
// 1. If a struct is returned from the function where the field has nilable value,
// e.g, If aptr is pointer in struct A, then  `return &A{}` causes the field aptr to escape
// 2. If a struct is parameter of a function and the field is not initialized
// e.g., if we have fun(&A{}) then the field aptr is considered escaped
// TODO: Add struct assignment as another possible cause of field escape
type FldEscape struct {
	*TriggerIfNonNil
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (f *FldEscape) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*FldEscape); ok {
		return f.TriggerIfNonNil.equals(other.TriggerIfNonNil)
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (f *FldEscape) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *f
	copyConsumer.TriggerIfNonNil = f.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this FldEscape as a Prestring
func (f *FldEscape) Prestring() Prestring {
	ann := f.Ann.(*EscapeFieldAnnotationKey)
	return FldEscapePrestring{
		FieldName:     ann.FieldDecl.Name(),
		AssignmentStr: f.assignmentFlow.String(),
	}
}

// FldEscapePrestring is a Prestring storing the needed information to compactly encode a FldEscape
type FldEscapePrestring struct {
	FieldName     string
	AssignmentStr string
}

func (f FldEscapePrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("field `%s` escaped out of our analysis scope (presumed nilable)", f.FieldName))
	sb.WriteString(f.AssignmentStr)
	return sb.String()
}

// UseAsNonErrorRetDependentOnErrorRetNilability is when a value flows to a point where it is returned from an error returning function
type UseAsNonErrorRetDependentOnErrorRetNilability struct {
	*TriggerIfNonNil

	IsNamedReturn bool
	RetStmt       *ast.ReturnStmt
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (u *UseAsNonErrorRetDependentOnErrorRetNilability) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*UseAsNonErrorRetDependentOnErrorRetNilability); ok {
		return u.TriggerIfNonNil.equals(other.TriggerIfNonNil) &&
			u.IsNamedReturn == other.IsNamedReturn &&
			u.RetStmt == other.RetStmt
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (u *UseAsNonErrorRetDependentOnErrorRetNilability) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *u
	copyConsumer.TriggerIfNonNil = u.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this UseAsNonErrorRetDependentOnErrorRetNilability as a Prestring
func (u *UseAsNonErrorRetDependentOnErrorRetNilability) Prestring() Prestring {
	retAnn := u.Ann.(*RetAnnotationKey)
	return UseAsNonErrorRetDependentOnErrorRetNilabilityPrestring{
		retAnn.FuncDecl.Name(),
		retAnn.RetNum,
		retAnn.FuncDecl.Type().(*types.Signature).Results().At(retAnn.RetNum).Name(),
		retAnn.FuncDecl.Type().(*types.Signature).Results().Len() - 1,
		u.IsNamedReturn,
		u.assignmentFlow.String(),
	}
}

// UseAsNonErrorRetDependentOnErrorRetNilabilityPrestring is a Prestring storing the needed information to compactly encode a UseAsNonErrorRetDependentOnErrorRetNilability
type UseAsNonErrorRetDependentOnErrorRetNilabilityPrestring struct {
	FuncName      string
	RetNum        int
	RetName       string
	ErrRetNum     int
	IsNamedReturn bool
	AssignmentStr string
}

func (u UseAsNonErrorRetDependentOnErrorRetNilabilityPrestring) String() string {
	via := ""
	if u.IsNamedReturn {
		via = fmt.Sprintf(" via named return `%s`", u.RetName)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("returned from `%s()`%s in position %d when the error return in position %d is not guaranteed to be non-nil through all paths",
		u.FuncName, via, u.RetNum, u.ErrRetNum))
	sb.WriteString(u.AssignmentStr)
	return sb.String()
}

// overriding position value to point to the raw return statement, which is the source of the potential error
func (u *UseAsNonErrorRetDependentOnErrorRetNilability) customPos() (token.Pos, bool) {
	if u.IsNamedReturn {
		return u.RetStmt.Pos(), true
	}
	return 0, false
}

// UseAsErrorRetWithNilabilityUnknown is when a value flows to a point where it is returned from an error returning function
type UseAsErrorRetWithNilabilityUnknown struct {
	*TriggerIfNonNil

	IsNamedReturn bool
	RetStmt       *ast.ReturnStmt
}

// equals returns true if the passed ConsumingAnnotationTrigger is equal to this one
func (u *UseAsErrorRetWithNilabilityUnknown) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*UseAsErrorRetWithNilabilityUnknown); ok {
		return u.TriggerIfNonNil.equals(other.TriggerIfNonNil) &&
			u.IsNamedReturn == other.IsNamedReturn &&
			u.RetStmt == other.RetStmt
	}
	return false
}

// Copy returns a deep copy of this ConsumingAnnotationTrigger
func (u *UseAsErrorRetWithNilabilityUnknown) Copy() ConsumingAnnotationTrigger {
	copyConsumer := *u
	copyConsumer.TriggerIfNonNil = u.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &copyConsumer
}

// Prestring returns this UseAsErrorRetWithNilabilityUnknown as a Prestring
func (u *UseAsErrorRetWithNilabilityUnknown) Prestring() Prestring {
	retAnn := u.Ann.(*RetAnnotationKey)
	return UseAsErrorRetWithNilabilityUnknownPrestring{
		retAnn.FuncDecl.Name(),
		retAnn.RetNum,
		u.IsNamedReturn,
		retAnn.FuncDecl.Type().(*types.Signature).Results().At(retAnn.RetNum).Name(),
		u.assignmentFlow.String(),
	}
}

// UseAsErrorRetWithNilabilityUnknownPrestring is a Prestring storing the needed information to compactly encode a UseAsErrorRetWithNilabilityUnknown
type UseAsErrorRetWithNilabilityUnknownPrestring struct {
	FuncName      string
	RetNum        int
	IsNamedReturn bool
	RetName       string
	AssignmentStr string
}

func (u UseAsErrorRetWithNilabilityUnknownPrestring) String() string {
	var sb strings.Builder
	if u.IsNamedReturn {
		sb.WriteString(fmt.Sprintf("found in at least one path of `%s()` for named return `%s` in position %d", u.FuncName, u.RetName, u.RetNum))
	} else {
		sb.WriteString(fmt.Sprintf("found in at least one path of `%s()` for return in position %d", u.FuncName, u.RetNum))
	}
	sb.WriteString(u.AssignmentStr)
	return sb.String()
}

// overriding position value to point to the raw return statement, which is the source of the potential error
func (u *UseAsErrorRetWithNilabilityUnknown) customPos() (token.Pos, bool) {
	if u.IsNamedReturn {
		return u.RetStmt.Pos(), true
	}
	return 0, false
}

// don't modify the ConsumeTrigger and ProduceTrigger objects after construction! Pointers
// to them are duplicated

// A ConsumeTrigger represents a point at which a value is consumed that may be required to be
// non-nil by some Annotation (ConsumingAnnotationTrigger). If Parent is not a RootAssertionNode,
// then that AssertionNode represents the expression that will flow into this consumption point.
// If Parent is a RootAssertionNode, then it will be paired with a ProduceTrigger
//
// Expr should be the expression being consumed, not the expression doing the consumption.
// For example, if the field access x.f requires x to be non-nil, then x should be the
// expression embedded in the ConsumeTrigger not x.f.
//
// The set Guards indicates whether this consumption takes places in a context in which
// it is known to be _guarded_ by one or more conditional checks that refine its behavior.
// This is not _all_ conditional checks this is a very small subset of them.
// Consume triggers become guarded via backpropagation across a check that
// `propagateRichChecks` identified with a `RichCheckEffect`. This pass will
// embed a call to `ConsumeTriggerSliceAsGuarded` that will modify all consume
// triggers for the value targeted by the check as guarded by the guard nonces of the
// flowed `RichCheckEffect`.
//
// Like a nil check, guarding is used to indicate information
// refinement local to one branch. The presence of a guard is overwritten by the absence of a guard
// on a given ConsumeTrigger - see MergeConsumeTriggerSlices. Beyond RichCheckEffects,
// Guards consume triggers can be introduced by other sites that are known to
// obey compatible semantics - such as passing the results of one error-returning function
// directly to a return of another.
//
// ConsumeTriggers arise at consumption sites that may guarded by a meaningful conditional check,
// adding that guard as a unique nonce to the set Guards of the trigger. The guard is added when the
// trigger is propagated across the check, so that when it reaches the statement that relies on the
// guard, the statement can see that the check was performed around the site of the consumption. This
// allows the statement to switch to more permissive semantics.
//
// GuardMatched is a boolean used to indicate that this ConsumeTrigger, by the current point in
// backpropagation, passed through a conditional that granted it a guard, and that that guard was
// determined to match the guard expected by a statement such as `v, ok := m[k]`. Since there could have
// been multiple paths in the CFG between the current point in backpropagation and the site at which the
// trigger arose, GuardMatched is true only if a guard arose and was matched along every path. This
// allows the trigger to maintain its more permissive semantics in later stages of backpropagation.
//
// For some productions, such as reading an index of a map, there is no way for them to generate
// nonnil without such a guarding along every path to their point of consumption, so if GuardMatched
// is not true then they will be replaced (by `checkGuardOnFullTrigger`) with an always-produce-nil
// producer. More explanation of this mechanism is provided in the documentation for
// `RootAssertionNode.AddGuardMatch`
//
// nonnil(Guards)
type ConsumeTrigger struct {
	Annotation   ConsumingAnnotationTrigger
	Expr         ast.Expr
	Guards       util.GuardNonceSet
	GuardMatched bool
}

// equals compares two ConsumeTrigger pointers for equality
func (c *ConsumeTrigger) equals(c2 *ConsumeTrigger) bool {
	return c.Annotation.equals(c2.Annotation) &&
		c.Expr == c2.Expr &&
		c.Guards.Eq(c2.Guards) &&
		c.GuardMatched == c2.GuardMatched

}

// Copy returns a deep copy of the ConsumeTrigger
func (c *ConsumeTrigger) Copy() *ConsumeTrigger {
	copyTrigger := *c
	copyTrigger.Annotation = c.Annotation.Copy()
	copyTrigger.Guards = c.Guards.Copy()
	return &copyTrigger
}

// Pos returns the source position (e.g., line) of the consumer's expression. In special cases, such as named return, it
// returns the position of the stored return AST node
func (c *ConsumeTrigger) Pos() token.Pos {
	if pos, ok := c.Annotation.customPos(); ok {
		return pos
	}
	return c.Expr.Pos()
}

// MergeConsumeTriggerSlices merges two slices of `ConsumeTrigger`s
// its semantics are slightly unexpected only in its treatment of guarding:
// it intersects guard sets
func MergeConsumeTriggerSlices(left, right []*ConsumeTrigger) []*ConsumeTrigger {
	var out []*ConsumeTrigger

	addToOut := func(trigger *ConsumeTrigger) {
		for i, outTrigger := range out {
			if outTrigger.Annotation.equals(trigger.Annotation) &&
				outTrigger.Expr == trigger.Expr {
				// intersect guard sets - if a guard isn't present in both branches it can't
				// be considered present before the branch
				out[i] = &ConsumeTrigger{
					Annotation:   outTrigger.Annotation.Copy(),
					Expr:         outTrigger.Expr,
					Guards:       outTrigger.Guards.Intersection(trigger.Guards),
					GuardMatched: outTrigger.GuardMatched && trigger.GuardMatched,
				}
				return
			}
		}
		out = append(out, trigger)
	}

	for _, l := range left {
		addToOut(l)
	}

	for _, r := range right {
		addToOut(r)
	}

	return out
}

// ConsumeTriggerSliceAsGuarded takes a slice of consume triggers,
// and returns a new slice identical except that each trigger is guarded
func ConsumeTriggerSliceAsGuarded(slice []*ConsumeTrigger, guards ...util.GuardNonce) []*ConsumeTrigger {
	var out []*ConsumeTrigger
	for _, trigger := range slice {
		out = append(out, &ConsumeTrigger{
			Annotation:   trigger.Annotation.Copy(),
			Expr:         trigger.Expr,
			Guards:       trigger.Guards.Copy().Add(guards...),
			GuardMatched: trigger.GuardMatched,
		})
	}
	return out
}

// ConsumeTriggerSlicesEq returns true if the two passed slices of ConsumeTrigger contain the same elements
// precondition: no duplications
func ConsumeTriggerSlicesEq(left, right []*ConsumeTrigger) bool {
	if len(left) != len(right) {
		return false
	}
lsearch:
	for _, l := range left {
		for _, r := range right {
			if l.equals(r) {
				continue lsearch
			}
		}
		return false
	}
	return true
}
