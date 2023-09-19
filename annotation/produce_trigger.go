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
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"go.uber.org/nilaway/util"
)

// A ProducingAnnotationTrigger is a possible reason that a nil value might be produced
//
// All ProducingAnnotationTriggers must embed one of the following 4 structs:
// -TriggerIfNilable
// -TriggerIfDeepNilable
// -ProduceTriggerTautology
// -ProduceTriggerNever
//
// This is because there are interfaces, such as AdmitsPrimitive, that are implemented only for those
// structs, and to which a ProducingAnnotationTrigger must be able to be case
type ProducingAnnotationTrigger interface {
	// CheckProduce can be called to determined whether this trigger should be triggered
	// given a particular Annotation map
	// for example - a `FuncReturn` trigger triggers iff the corresponding function has
	// nilable return type
	CheckProduce(Map) bool

	// NeedsGuardMatch returns whether this production is contingent on being
	// paired with a guarded consumer.
	// In other words, this production is only given the freedom to produce
	// a non-nil value in the case that it is matched with a guarded consumer.
	// otherwise, it is replaced with annotation.GuardMissing
	NeedsGuardMatch() bool

	// SetNeedsGuard sets the underlying Guard-Neediness of this ProduceTrigger, if present
	// This should be very sparingly used, and only with utter conviction of correctness
	SetNeedsGuard(bool)

	Prestring() Prestring

	// Kind returns the kind of the trigger.
	Kind() TriggerKind

	// UnderlyingSite returns the underlying site this trigger's nilability depends on. If the
	// trigger always or never fires, the site is nil.
	UnderlyingSite() Key

	// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
	equals(ProducingAnnotationTrigger) bool
}

// TriggerIfNilable is a general trigger indicating that the bad case occurs when a certain Annotation
// key is nilable
type TriggerIfNilable struct {
	Ann        Key
	NeedsGuard bool
}

// Prestring returns this Prestring as a Prestring
func (*TriggerIfNilable) Prestring() Prestring {
	return TriggerIfNilablePrestring{}
}

// TriggerIfNilablePrestring is a Prestring storing the needed information to compactly encode a TriggerIfNilable
type TriggerIfNilablePrestring struct{}

func (TriggerIfNilablePrestring) String() string {
	return "nilable value"
}

// CheckProduce returns true if the underlying annotation is present in the passed map and nilable
func (t *TriggerIfNilable) CheckProduce(annMap Map) bool {
	ann, ok := t.Ann.Lookup(annMap)
	return ok && ann.IsNilable
}

// NeedsGuardMatch for a `TriggerIfNilable` is default false, as guarding
// applies mostly to deep reads, but this behavior is overriden
// for `VariadicFuncParamDeep`s, which have the semantics of
// deep reads despite consulting shallow annotations
func (t *TriggerIfNilable) NeedsGuardMatch() bool { return t.NeedsGuard }

// SetNeedsGuard for a `TriggerIfNilable` is, by default, a noop, as guarding
// applies mostly to deep reads, but this behavior is overriden
// for `VariadicFuncParamDeep`s, which have the semantics of
// deep reads despite consulting shallow annotations
func (t *TriggerIfNilable) SetNeedsGuard(b bool) { t.NeedsGuard = b }

// Kind returns Conditional.
func (t *TriggerIfNilable) Kind() TriggerKind { return Conditional }

// UnderlyingSite returns the underlying site this trigger's nilability depends on.
func (t *TriggerIfNilable) UnderlyingSite() Key { return t.Ann }

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (t *TriggerIfNilable) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*TriggerIfNilable); ok {
		return t.Ann.equals(other.Ann) && t.NeedsGuard == other.NeedsGuard
	}
	return false
}

// TriggerIfDeepNilable is a general trigger indicating the the bad case occurs when a certain Annotation
// key is deeply nilable
type TriggerIfDeepNilable struct {
	Ann        Key
	NeedsGuard bool
}

// Prestring returns this Prestring as a Prestring
func (*TriggerIfDeepNilable) Prestring() Prestring {
	return TriggerIfDeepNilablePrestring{}
}

// TriggerIfDeepNilablePrestring is a Prestring storing the needed information to compactly encode a TriggerIfDeepNilable
type TriggerIfDeepNilablePrestring struct{}

func (TriggerIfDeepNilablePrestring) String() string {
	return "deeply nilable value"
}

// CheckProduce returns true if the underlying annotation is present in the passed map and deeply nilable
func (t *TriggerIfDeepNilable) CheckProduce(annMap Map) bool {
	ann, ok := t.Ann.Lookup(annMap)
	return ok && ann.IsDeepNilable
}

// NeedsGuardMatch for a `TriggerIfDeepNilable` is default false,
// but overridden for most concrete triggers to read a boolean
// field
func (t *TriggerIfDeepNilable) NeedsGuardMatch() bool { return t.NeedsGuard }

// SetNeedsGuard for a `TriggerIfDeepNilable` is, by default, a noop,
// but overridden for most concrete triggers to set an underlying field
func (t *TriggerIfDeepNilable) SetNeedsGuard(b bool) { t.NeedsGuard = b }

// Kind returns DeepConditional.
func (t *TriggerIfDeepNilable) Kind() TriggerKind { return DeepConditional }

// UnderlyingSite returns the underlying site this trigger's nilability depends on.
func (t *TriggerIfDeepNilable) UnderlyingSite() Key { return t.Ann }

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (t *TriggerIfDeepNilable) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*TriggerIfDeepNilable); ok {
		return t.Ann.equals(other.Ann) && t.NeedsGuard == other.NeedsGuard
	}
	return false
}

// ProduceTriggerTautology is used for trigger producers that will always result in nil
type ProduceTriggerTautology struct {
	NeedsGuard bool
}

// CheckProduce returns true
func (*ProduceTriggerTautology) CheckProduce(Map) bool {
	return true
}

// NeedsGuardMatch for a ProduceTriggerTautology is false - there is no wiggle room with these
func (p *ProduceTriggerTautology) NeedsGuardMatch() bool {
	return p.NeedsGuard
}

// SetNeedsGuard for a ProduceTriggerTautology is a noop
func (p *ProduceTriggerTautology) SetNeedsGuard(b bool) { p.NeedsGuard = b }

// Prestring returns this Prestring as a Prestring
func (*ProduceTriggerTautology) Prestring() Prestring {
	return ProduceTriggerTautologyPrestring{}
}

// ProduceTriggerTautologyPrestring is a Prestring storing the needed information to compactly encode a ProduceTriggerTautology
type ProduceTriggerTautologyPrestring struct{}

func (ProduceTriggerTautologyPrestring) String() string {
	return "nilable value"
}

// Kind returns Always.
func (*ProduceTriggerTautology) Kind() TriggerKind { return Always }

// UnderlyingSite always returns nil.
func (*ProduceTriggerTautology) UnderlyingSite() Key { return nil }

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (p *ProduceTriggerTautology) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*ProduceTriggerTautology); ok {
		return p.NeedsGuard == other.NeedsGuard
	}
	return false
}

// ProduceTriggerNever is used for trigger producers that will never be nil
type ProduceTriggerNever struct {
	NeedsGuard bool
}

// Prestring returns this Prestring as a Prestring
func (*ProduceTriggerNever) Prestring() Prestring {
	return ProduceTriggerNeverPrestring{}
}

// ProduceTriggerNeverPrestring is a Prestring storing the needed information to compactly encode a ProduceTriggerNever
type ProduceTriggerNeverPrestring struct{}

func (ProduceTriggerNeverPrestring) String() string {
	return "is not nilable"
}

// CheckProduce returns true false
func (*ProduceTriggerNever) CheckProduce(Map) bool {
	return false
}

// NeedsGuardMatch for a ProduceTriggerNever is false, like ProduceTriggerTautology
func (p *ProduceTriggerNever) NeedsGuardMatch() bool { return p.NeedsGuard }

// SetNeedsGuard for a ProduceTriggerNever is a noop, like ProduceTriggerTautology
func (p *ProduceTriggerNever) SetNeedsGuard(b bool) { p.NeedsGuard = b }

// Kind returns Never.
func (*ProduceTriggerNever) Kind() TriggerKind { return Never }

// UnderlyingSite always returns nil.
func (*ProduceTriggerNever) UnderlyingSite() Key { return nil }

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (p *ProduceTriggerNever) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*ProduceTriggerNever); ok {
		return p.NeedsGuard == other.NeedsGuard
	}
	return false
}

// note: each of the following two productions, ExprOkCheck, and RangeIndexAssignment, should be
// obselete now that we don't add consumptions for basic-typed expressions like ints and bools to
// begin with - TODO: verify that these productions are always no-ops and remove

// ExprOkCheck is used when a value is determined to flow from the second argument of a map or typecast
// operation that necessarily makes it boolean and thus non-nil
type ExprOkCheck struct {
	*ProduceTriggerNever
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (e *ExprOkCheck) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*ExprOkCheck); ok {
		return e.ProduceTriggerNever.equals(other.ProduceTriggerNever)
	}
	return false
}

// RangeIndexAssignment is used when a value is determined to flow from the first argument of a
// range loop, and thus be an integer and non-nil
type RangeIndexAssignment struct {
	*ProduceTriggerNever
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (r *RangeIndexAssignment) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*RangeIndexAssignment); ok {
		return r.ProduceTriggerNever.equals(other.ProduceTriggerNever)
	}
	return false
}

// PositiveNilCheck is used when a value is checked in a conditional to BE nil
type PositiveNilCheck struct {
	*ProduceTriggerTautology
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (p *PositiveNilCheck) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*PositiveNilCheck); ok {
		return p.ProduceTriggerTautology.equals(other.ProduceTriggerTautology)
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (*PositiveNilCheck) Prestring() Prestring {
	return PositiveNilCheckPrestring{}
}

// PositiveNilCheckPrestring is a Prestring storing the needed information to compactly encode a PositiveNilCheck
type PositiveNilCheckPrestring struct{}

func (PositiveNilCheckPrestring) String() string {
	return "determined nil via conditional check"
}

// NegativeNilCheck is used when a value is checked in a conditional to NOT BE nil
type NegativeNilCheck struct {
	*ProduceTriggerNever
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (n *NegativeNilCheck) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*NegativeNilCheck); ok {
		return n.ProduceTriggerNever.equals(other.ProduceTriggerNever)
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (*NegativeNilCheck) Prestring() Prestring {
	return NegativeNilCheckPrestring{}
}

// NegativeNilCheckPrestring is a Prestring storing the needed information to compactly encode a NegativeNilCheck
type NegativeNilCheckPrestring struct{}

func (NegativeNilCheckPrestring) String() string {
	return "determined nonnil via conditional check"
}

// OkReadReflCheck is used to produce nonnil for artifacts of successful `ok` forms (e.g., maps, channels, type casts).
// For example, a map value `m` that was read from in a `v, ok := m[k]` check followed by a positive check of `ok`, implies `m` is non-nil.
// This is valid because nil maps contain no keys.
type OkReadReflCheck struct {
	*ProduceTriggerNever
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (o *OkReadReflCheck) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*OkReadReflCheck); ok {
		return o.ProduceTriggerNever.equals(other.ProduceTriggerNever)
	}
	return false
}

// RangeOver is used when a value is ranged over - and thus nonnil in its range body
type RangeOver struct {
	*ProduceTriggerNever
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (r *RangeOver) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*RangeOver); ok {
		return r.ProduceTriggerNever.equals(other.ProduceTriggerNever)
	}
	return false
}

// ConstNil is when a value is determined to flow from a constant nil expression
type ConstNil struct {
	*ProduceTriggerTautology
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (c *ConstNil) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*ConstNil); ok {
		return c.ProduceTriggerTautology.equals(other.ProduceTriggerTautology)
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (*ConstNil) Prestring() Prestring {
	return ConstNilPrestring{}
}

// ConstNilPrestring is a Prestring storing the needed information to compactly encode a ConstNil
type ConstNilPrestring struct{}

func (ConstNilPrestring) String() string {
	return "literal `nil`"
}

// UnassignedFld is when a field of struct is not assigned at initialization
type UnassignedFld struct {
	*ProduceTriggerTautology
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (u *UnassignedFld) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*UnassignedFld); ok {
		return u.ProduceTriggerTautology.equals(other.ProduceTriggerTautology)
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (*UnassignedFld) Prestring() Prestring {
	return UnassignedFldPrestring{}
}

// UnassignedFldPrestring is a Prestring storing the needed information to compactly encode a UnassignedFld
type UnassignedFldPrestring struct{}

func (UnassignedFldPrestring) String() string {
	return "uninitialized"
}

// NoVarAssign is when a value is determined to flow from a variable that wasn't assigned to
type NoVarAssign struct {
	*ProduceTriggerTautology
	VarObj *types.Var
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (n *NoVarAssign) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*NoVarAssign); ok {
		return n.ProduceTriggerTautology.equals(other.ProduceTriggerTautology) && n.VarObj == other.VarObj
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (n *NoVarAssign) Prestring() Prestring {
	return NoVarAssignPrestring{
		VarName: n.VarObj.Name(),
	}
}

// NoVarAssignPrestring is a Prestring storing the needed information to compactly encode a NoVarAssign
type NoVarAssignPrestring struct {
	VarName string
}

func (n NoVarAssignPrestring) String() string {
	return fmt.Sprintf("unassigned variable `%s`", n.VarName)
}

// BlankVarReturn is when a value is determined to flow from a blank variable ('_') to a return of the function
type BlankVarReturn struct {
	*ProduceTriggerTautology
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (b *BlankVarReturn) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*BlankVarReturn); ok {
		return b.ProduceTriggerTautology.equals(other.ProduceTriggerTautology)
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (*BlankVarReturn) Prestring() Prestring {
	return BlankVarReturnPrestring{}
}

// BlankVarReturnPrestring is a Prestring storing the needed information to compactly encode a BlankVarReturn
type BlankVarReturnPrestring struct{}

func (BlankVarReturnPrestring) String() string {
	return "return via a blank variable `_`"
}

// DuplicateParamProducer duplicates a given produce trigger, assuming the given produce trigger
// is of FuncParam.
func DuplicateParamProducer(t *ProduceTrigger, location token.Position) *ProduceTrigger {
	key := t.Annotation.(*FuncParam).TriggerIfNilable.Ann.(*ParamAnnotationKey)
	return &ProduceTrigger{
		Annotation: &FuncParam{
			TriggerIfNilable: &TriggerIfNilable{
				Ann: NewCallSiteParamKey(key.FuncDecl, key.ParamNum, location)}},
		Expr: t.Expr,
	}
}

// FuncParam is used when a value is determined to flow from a function parameter. This consumer
// trigger can be used on top of two different sites: ParamAnnotationKey &
// CallSiteParamAnnotationKey. ParamAnnotationKey is the parameter site in the function
// declaration; CallSiteParamAnnotationKey is the argument site in the call expression.
// CallSiteParamAnnotationKey is specifically used for functions with contracts since we need to
// duplicate the sites for context sensitivity.
type FuncParam struct {
	*TriggerIfNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *FuncParam) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*FuncParam); ok {
		return f.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

// Prestring returns this FuncParam as a Prestring
func (f *FuncParam) Prestring() Prestring {
	switch key := f.Ann.(type) {
	case *ParamAnnotationKey:
		return FuncParamPrestring{key.ParamNameString(), key.FuncDecl.Name(), ""}
	case *CallSiteParamAnnotationKey:
		return FuncParamPrestring{key.ParamNameString(), key.FuncDecl.Name(), key.Location.String()}
	default:
		panic(fmt.Sprintf("Expected ParamAnnotationKey or CallSiteParamAnnotationKey but got: %T", key))
	}
}

// FuncParamPrestring is a Prestring storing the needed information to compactly encode a FuncParam
type FuncParamPrestring struct {
	ParamName string
	FuncName  string
	// Location is empty for a FuncParam enclosing ParamAnnotationKey. Location points to the
	// location of the argument pass at the call site for a FuncParam enclosing CallSiteParamAnnotationKey.
	Location string
}

func (f FuncParamPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("function parameter `%s`", f.ParamName))
	if f.Location != "" {
		sb.WriteString(fmt.Sprintf(" at %s", f.Location))
	}
	return sb.String()
}

// MethodRecv is used when a value is determined to flow from a method receiver
type MethodRecv struct {
	*TriggerIfNilable
	VarDecl *types.Var
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (m *MethodRecv) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*MethodRecv); ok {
		return m.TriggerIfNilable.equals(other.TriggerIfNilable) && m.VarDecl == other.VarDecl
	}
	return false
}

// Prestring returns this MethodRecv as a Prestring
func (m *MethodRecv) Prestring() Prestring {
	return MethodRecvPrestring{m.VarDecl.Name()}
}

// MethodRecvPrestring is a Prestring storing the needed information to compactly encode a MethodRecv
type MethodRecvPrestring struct {
	RecvName string
}

func (m MethodRecvPrestring) String() string {
	return fmt.Sprintf("read by method receiver `%s`", m.RecvName)
}

// MethodRecvDeep is used when a value is determined to flow deeply from a method receiver
type MethodRecvDeep struct {
	*TriggerIfDeepNilable
	VarDecl *types.Var
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (m *MethodRecvDeep) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*MethodRecvDeep); ok {
		return m.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable) && m.VarDecl == other.VarDecl
	}
	return false
}

// Prestring returns this MethodRecv as a Prestring
func (m *MethodRecvDeep) Prestring() Prestring {
	return MethodRecvDeepPrestring{m.VarDecl.Name()}
}

// MethodRecvDeepPrestring is a Prestring storing the needed information to compactly encode a MethodRecv
type MethodRecvDeepPrestring struct {
	RecvName string
}

func (m MethodRecvDeepPrestring) String() string {
	return fmt.Sprintf("deep read by method receiver `%s`", m.RecvName)
}

// VariadicFuncParam is used when a value is determined to flow from a variadic function parameter,
// and thus always be nilable
type VariadicFuncParam struct {
	*ProduceTriggerTautology
	VarDecl *types.Var
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (v *VariadicFuncParam) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*VariadicFuncParam); ok {
		return v.ProduceTriggerTautology.equals(other.ProduceTriggerTautology) && v.VarDecl == other.VarDecl
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (v *VariadicFuncParam) Prestring() Prestring {
	return VariadicFuncParamPrestring{v.VarDecl.Name()}
}

// VariadicFuncParamPrestring is a Prestring storing the needed information to compactly encode a VariadicFuncParam
type VariadicFuncParamPrestring struct {
	ParamName string
}

func (v VariadicFuncParamPrestring) String() string {
	return fmt.Sprintf("read directly from variadic parameter `%s`", v.ParamName)
}

// TrustedFuncNilable is used when a value is determined to be nilable by a trusted function call
type TrustedFuncNilable struct {
	*ProduceTriggerTautology
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (t *TrustedFuncNilable) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*TrustedFuncNilable); ok {
		return t.ProduceTriggerTautology.equals(other.ProduceTriggerTautology)
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (*TrustedFuncNilable) Prestring() Prestring {
	return TrustedFuncNilablePrestring{}
}

// TrustedFuncNilablePrestring is a Prestring storing the needed information to compactly encode a TrustedFuncNilable
type TrustedFuncNilablePrestring struct{}

func (TrustedFuncNilablePrestring) String() string {
	return "determined to be nilable by a trusted function"
}

// TrustedFuncNonnil is used when a value is determined to be nonnil by a trusted function call
type TrustedFuncNonnil struct {
	*ProduceTriggerNever
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (t *TrustedFuncNonnil) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*TrustedFuncNonnil); ok {
		return t.ProduceTriggerNever.equals(other.ProduceTriggerNever)
	}
	return false
}

// Prestring returns this Prestring as a Prestring
func (*TrustedFuncNonnil) Prestring() Prestring {
	return TrustedFuncNonnilPrestring{}
}

// TrustedFuncNonnilPrestring is a Prestring storing the needed information to compactly encode a TrustedFuncNonnil
type TrustedFuncNonnilPrestring struct{}

func (TrustedFuncNonnilPrestring) String() string {
	return "determined to be nonnil by a trusted function"
}

// FldRead is used when a value is determined to flow from a read to a field
type FldRead struct {
	*TriggerIfNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *FldRead) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*FldRead); ok {
		return f.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

// Prestring returns this FldRead as a Prestring
func (f *FldRead) Prestring() Prestring {
	if ek, ok := f.Ann.(*EscapeFieldAnnotationKey); ok {
		return FldReadPrestring{ek.FieldDecl.Name()}
	}
	return FldReadPrestring{f.Ann.(*FieldAnnotationKey).FieldDecl.Name()}
}

// FldReadPrestring is a Prestring storing the needed information to compactly encode a FldRead
type FldReadPrestring struct {
	FieldName string
}

func (f FldReadPrestring) String() string {
	return fmt.Sprintf("field `%s`", f.FieldName)
}

// ParamFldRead is used when a struct field value is determined to flow from the param of a function to a consumption
// site within the body of the function
type ParamFldRead struct {
	*TriggerIfNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *ParamFldRead) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*ParamFldRead); ok {
		return f.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

// Prestring returns this ParamFldRead as a Prestring
func (f *ParamFldRead) Prestring() Prestring {
	ann := f.Ann.(*ParamFieldAnnotationKey)
	return ParamFldReadPrestring{
		FieldName: ann.FieldDecl.Name(),
	}
}

// ParamFldReadPrestring is a Prestring storing the needed information to compactly encode a ParamFldRead
type ParamFldReadPrestring struct {
	FieldName string
}

func (f ParamFldReadPrestring) String() string {
	return fmt.Sprintf("field `%s`", f.FieldName)
}

// FldReturn is used when a struct field value is determined to flow from a return value of a function
type FldReturn struct {
	*TriggerIfNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *FldReturn) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*FldReturn); ok {
		return f.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

func (f FldReturn) String() string {
	return f.Prestring().String()
}

// Prestring returns this FldReturn as a Prestring
func (f *FldReturn) Prestring() Prestring {
	key := f.Ann.(*RetFieldAnnotationKey)
	return FldReturnPrestring{key.RetNum, key.FuncDecl.Name(), key.FieldDecl.Name()}
}

// FldReturnPrestring is a Prestring storing the needed information to compactly encode a FldReturn
type FldReturnPrestring struct {
	RetNum    int
	FuncName  string
	FieldName string
}

func (f FldReturnPrestring) String() string {
	return fmt.Sprintf("field `%s` of result %d of `%s()`", f.FieldName, f.RetNum, f.FuncName)
}

// FuncReturn is used when a value is determined to flow from the return of a function. This
// consumer trigger can be used on top of two different sites: RetAnnotationKey &
// CallSiteRetAnnotationKey. RetAnnotationKey is the parameter site in the function declaration;
// CallSiteRetAnnotationKey is the argument site in the call expression. CallSiteRetAnnotationKey
// is specifically used for functions with contracts since we need to duplicate the sites for
// context sensitivity.
type FuncReturn struct {
	*TriggerIfNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *FuncReturn) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*FuncReturn); ok {
		return f.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

// Prestring returns this FuncReturn as a Prestring
func (f *FuncReturn) Prestring() Prestring {
	switch key := f.Ann.(type) {
	case *RetAnnotationKey:
		return FuncReturnPrestring{key.RetNum, key.FuncDecl.Name(), ""}
	case *CallSiteRetAnnotationKey:
		return FuncReturnPrestring{key.RetNum, key.FuncDecl.Name(), key.Location.String()}
	default:
		panic(fmt.Sprintf("Expected RetAnnotationKey or CallSiteRetAnnotationKey but got: %T", key))
	}
}

// FuncReturnPrestring is a Prestring storing the needed information to compactly encode a FuncReturn
type FuncReturnPrestring struct {
	RetNum   int
	FuncName string
	// Location is empty for a FuncReturn enclosing RetAnnotationKey. Location points to the
	// location of the result return at the call site for a FuncReturn enclosing CallSiteRetAnnotationKey.
	Location string
}

func (f FuncReturnPrestring) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("result %d of `%s()`", f.RetNum, f.FuncName))
	if f.Location != "" {
		sb.WriteString(fmt.Sprintf(" at %s", f.Location))
	}
	return sb.String()
}

// MethodReturn is used when a value is determined to flow from the return of a method
type MethodReturn struct {
	*TriggerIfNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (m *MethodReturn) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*MethodReturn); ok {
		return m.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

// Prestring returns this MethodReturn as a Prestring
func (m *MethodReturn) Prestring() Prestring {
	retKey := m.Ann.(*RetAnnotationKey)
	return MethodReturnPrestring{retKey.RetNum, retKey.FuncDecl.Name()}
}

// MethodReturnPrestring is a Prestring storing the needed information to compactly encode a MethodReturn
type MethodReturnPrestring struct {
	RetNum   int
	FuncName string
}

func (m MethodReturnPrestring) String() string {
	return fmt.Sprintf("result %d of `%s()`", m.RetNum, m.FuncName)
}

// MethodResultReachesInterface is used when a result of a method is determined to flow into a result of an interface using inheritance
type MethodResultReachesInterface struct {
	*TriggerIfNilable
	*AffiliationPair
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (m *MethodResultReachesInterface) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*MethodResultReachesInterface); ok {
		return m.TriggerIfNilable.equals(other.TriggerIfNilable) &&
			m.AffiliationPair.InterfaceMethod == other.AffiliationPair.InterfaceMethod &&
			m.AffiliationPair.ImplementingMethod == other.AffiliationPair.ImplementingMethod
	}
	return false
}

// Prestring returns this MethodResultReachesInterface as a Prestring
func (m *MethodResultReachesInterface) Prestring() Prestring {
	retAnn := m.Ann.(*RetAnnotationKey)
	return MethodResultReachesInterfacePrestring{
		retAnn.RetNum,
		util.PartiallyQualifiedFuncName(retAnn.FuncDecl),
		util.PartiallyQualifiedFuncName(m.InterfaceMethod),
	}
}

// MethodResultReachesInterfacePrestring is a Prestring storing the needed information to compactly encode a MethodResultReachesInterface
type MethodResultReachesInterfacePrestring struct {
	RetNum   int
	ImplName string
	IntName  string
}

func (m MethodResultReachesInterfacePrestring) String() string {
	return ""
}

// InterfaceParamReachesImplementation is used when a param of a method is determined to flow into the param of an implementing method
type InterfaceParamReachesImplementation struct {
	*TriggerIfNilable
	*AffiliationPair
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (i *InterfaceParamReachesImplementation) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*InterfaceParamReachesImplementation); ok {
		return i.TriggerIfNilable.equals(other.TriggerIfNilable) &&
			i.AffiliationPair.InterfaceMethod == other.AffiliationPair.InterfaceMethod &&
			i.AffiliationPair.ImplementingMethod == other.AffiliationPair.ImplementingMethod
	}
	return false
}

// Prestring returns this InterfaceParamReachesImplementation as a Prestring
func (i *InterfaceParamReachesImplementation) Prestring() Prestring {
	paramAnn := i.Ann.(*ParamAnnotationKey)
	return InterfaceParamReachesImplementationPrestring{
		paramAnn.ParamNameString(),
		util.PartiallyQualifiedFuncName(paramAnn.FuncDecl),
		util.PartiallyQualifiedFuncName(i.ImplementingMethod),
	}
}

// InterfaceParamReachesImplementationPrestring is a Prestring storing the needed information to compactly encode a InterfaceParamReachesImplementation
type InterfaceParamReachesImplementationPrestring struct {
	ParamName string
	IntName   string
	ImplName  string
}

func (i InterfaceParamReachesImplementationPrestring) String() string {
	return ""
}

// GlobalVarRead is when a value is determined to flow from a read to a global variable
type GlobalVarRead struct {
	*TriggerIfNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (g *GlobalVarRead) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*GlobalVarRead); ok {
		return g.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

// Prestring returns this GlobalVarRead as a Prestring
func (g *GlobalVarRead) Prestring() Prestring {
	key := g.Ann.(*GlobalVarAnnotationKey)
	return GlobalVarReadPrestring{
		key.VarDecl.Name(),
	}
}

// GlobalVarReadPrestring is a Prestring storing the needed information to compactly encode a GlobalVarRead
type GlobalVarReadPrestring struct {
	VarName string
}

func (g GlobalVarReadPrestring) String() string {
	return fmt.Sprintf("global variable `%s`", g.VarName)
}

// MapRead is when a value is determined to flow from a map index expression
// These should always be instantiated with NeedsGuard = true
type MapRead struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (m *MapRead) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*MapRead); ok {
		return m.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Prestring returns this MapRead as a Prestring
func (m *MapRead) Prestring() Prestring {
	key := m.Ann.(*TypeNameAnnotationKey)
	return MapReadPrestring{key.TypeDecl.Name()}
}

// MapReadPrestring is a Prestring storing the needed information to compactly encode a MapRead
type MapReadPrestring struct {
	TypeName string
}

func (m MapReadPrestring) String() string {
	return fmt.Sprintf("index of a map of type `%s`", m.TypeName)
}

// ArrayRead is when a value is determined to flow from an array index expression
type ArrayRead struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (a *ArrayRead) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*ArrayRead); ok {
		return a.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Prestring returns this ArrayRead as a Prestring
func (a *ArrayRead) Prestring() Prestring {
	key := a.Ann.(*TypeNameAnnotationKey)
	return ArrayReadPrestring{key.TypeDecl.Name()}
}

// ArrayReadPrestring is a Prestring storing the needed information to compactly encode a ArrayRead
type ArrayReadPrestring struct {
	TypeName string
}

func (a ArrayReadPrestring) String() string {
	return fmt.Sprintf("index of an array of type `%s`", a.TypeName)
}

// SliceRead is when a value is determined to flow from a slice index expression
type SliceRead struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (s *SliceRead) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*SliceRead); ok {
		return s.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Prestring returns this SliceRead as a Prestring
func (s *SliceRead) Prestring() Prestring {
	key := s.Ann.(*TypeNameAnnotationKey)
	return SliceReadPrestring{key.TypeDecl.Name()}
}

// SliceReadPrestring is a Prestring storing the needed information to compactly encode a SliceRead
type SliceReadPrestring struct {
	TypeName string
}

func (s SliceReadPrestring) String() string {
	return fmt.Sprintf("index of a slice of type `%s`", s.TypeName)
}

// PtrRead is when a value is determined to flow from a read to a pointer
type PtrRead struct {
	*TriggerIfDeepNilable
}

// Prestring returns this PtrRead as a Prestring
func (p *PtrRead) Prestring() Prestring {
	key := p.Ann.(*TypeNameAnnotationKey)
	return PtrReadPrestring{key.TypeDecl.Name()}
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (p *PtrRead) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*PtrRead); ok {
		return p.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// PtrReadPrestring is a Prestring storing the needed information to compactly encode a PtrRead
type PtrReadPrestring struct {
	TypeName string
}

func (p PtrReadPrestring) String() string {
	return fmt.Sprintf("value of a pointer of type `%s`", p.TypeName)
}

// ChanRecv is when a value is determined to flow from a channel receive
type ChanRecv struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (c *ChanRecv) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*ChanRecv); ok {
		return c.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Prestring returns this ChanRecv as a Prestring
func (c *ChanRecv) Prestring() Prestring {
	key := c.Ann.(*TypeNameAnnotationKey)
	return ChanRecvPrestring{key.TypeDecl.Name()}
}

// ChanRecvPrestring is a Prestring storing the needed information to compactly encode a ChanRecv
type ChanRecvPrestring struct {
	TypeName string
}

func (c ChanRecvPrestring) String() string {
	return fmt.Sprintf("received from a channel of type `%s`", c.TypeName)
}

// FuncParamDeep is used when a value is determined to flow deeply from a function parameter
type FuncParamDeep struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *FuncParamDeep) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*FuncParamDeep); ok {
		return f.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Prestring returns this FuncParamDeep as a Prestring
func (f *FuncParamDeep) Prestring() Prestring {
	key := f.Ann.(*ParamAnnotationKey)
	return FuncParamDeepPrestring{key.ParamNameString()}
}

// FuncParamDeepPrestring is a Prestring storing the needed information to compactly encode a FuncParamDeep
type FuncParamDeepPrestring struct {
	ParamName string
}

func (f FuncParamDeepPrestring) String() string {
	return fmt.Sprintf("deep read from parameter `%s`", f.ParamName)
}

// VariadicFuncParamDeep is used when a value is determined to flow deeply from a variadic function
// parameter, and thus be nilable iff the shallow Annotation on that parameter is nilable
type VariadicFuncParamDeep struct {
	*TriggerIfNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (v *VariadicFuncParamDeep) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*VariadicFuncParamDeep); ok {
		return v.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

// Prestring returns this VariadicFuncParamDeep as a Prestring
func (v *VariadicFuncParamDeep) Prestring() Prestring {
	return VariadicFuncParamDeepPrestring{v.Ann.(*ParamAnnotationKey).ParamNameString()}
}

// VariadicFuncParamDeepPrestring is a Prestring storing the needed information to compactly encode a VariadicFuncParamDeep
type VariadicFuncParamDeepPrestring struct {
	ParamName string
}

func (v VariadicFuncParamDeepPrestring) String() string {
	return fmt.Sprintf("index of variadic parameter `%s`", v.ParamName)
}

// FuncReturnDeep is used when a value is determined to flow from the deep Annotation of the return
// of a function
type FuncReturnDeep struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *FuncReturnDeep) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*FuncReturnDeep); ok {
		return f.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Prestring returns this FuncReturnDeep as a Prestring
func (f *FuncReturnDeep) Prestring() Prestring {
	key := f.Ann.(*RetAnnotationKey)
	return FuncReturnDeepPrestring{key.RetNum, key.FuncDecl.Name()}
}

// FuncReturnDeepPrestring is a Prestring storing the needed information to compactly encode a FuncReturnDeep
type FuncReturnDeepPrestring struct {
	RetNum   int
	FuncName string
}

func (f FuncReturnDeepPrestring) String() string {
	return fmt.Sprintf("deep read from result %d of `%s()`", f.RetNum, f.FuncName)
}

// FldReadDeep is used when a value is determined to flow from the deep Annotation of a field that is
// read and then indexed into - for example x.f[0]
type FldReadDeep struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *FldReadDeep) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*FldReadDeep); ok {
		return f.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Prestring returns this FldReadDeep as a Prestring
func (f *FldReadDeep) Prestring() Prestring {
	key := f.Ann.(*FieldAnnotationKey)
	return FldReadDeepPrestring{key.FieldDecl.Name()}
}

// FldReadDeepPrestring is a Prestring storing the needed information to compactly encode a FldReadDeep
type FldReadDeepPrestring struct {
	FieldName string
}

func (f FldReadDeepPrestring) String() string {
	return fmt.Sprintf("deep read from field `%s`", f.FieldName)
}

// LocalVarReadDeep is when a value is determined to flow deeply from a local variable. It is never nilable
// if appropriately guarded.
type LocalVarReadDeep struct {
	*ProduceTriggerNever
	ReadVar *types.Var
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (v *LocalVarReadDeep) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*LocalVarReadDeep); ok {
		return v.ProduceTriggerNever.equals(other.ProduceTriggerNever) &&
			v.ReadVar == other.ReadVar
	}
	return false
}

// Prestring returns this LocalVarReadDeep as a Prestring
func (v *LocalVarReadDeep) Prestring() Prestring {
	return LocalVarReadDeepPrestring{v.ReadVar.Name()}
}

// LocalVarReadDeepPrestring is a Prestring storing the needed information to compactly encode a LocalVarReadDeep
type LocalVarReadDeepPrestring struct {
	VarName string
}

func (v LocalVarReadDeepPrestring) String() string {
	return fmt.Sprintf("deep read from variable `%s`", v.VarName)
}

// GlobalVarReadDeep is when a value is determined to flow from the deep Annotation of a global variable
// that is read and indexed into
type GlobalVarReadDeep struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (g *GlobalVarReadDeep) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*GlobalVarReadDeep); ok {
		return g.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Prestring returns this GlobalVarReadDeep as a Prestring
func (g *GlobalVarReadDeep) Prestring() Prestring {
	key := g.Ann.(*GlobalVarAnnotationKey)
	return GlobalVarReadDeepPrestring{key.VarDecl.Name()}
}

// GlobalVarReadDeepPrestring is a Prestring storing the needed information to compactly encode a GlobalVarReadDeep
type GlobalVarReadDeepPrestring struct {
	VarName string
}

func (g GlobalVarReadDeepPrestring) String() string {
	return fmt.Sprintf("deep read from global variable `%s`", g.VarName)
}

// GuardMissing is when a value is determined to flow from a site that requires a guard,
// to a site that is not guarded by that guard.
//
// GuardMissing is never created during backpropagation, but on a call to RootAssertionNode.ProcessEntry
// that checks the guards on ever FullTrigger created, it is substituted for the producer in any
// FullTrigger whose producer has NeedsGuard = true and whose consumer has GuardMatched = false,
// guaranteeing that that producer triggers.
//
// For example, from a read to map without the `v, ok := m[k]` form, thus always resulting in nilable
// regardless of `m`'s deep nilability
type GuardMissing struct {
	*ProduceTriggerTautology
	OldAnnotation ProducingAnnotationTrigger
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (g *GuardMissing) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*GuardMissing); ok {
		return g.ProduceTriggerTautology.equals(other.ProduceTriggerTautology) && g.OldAnnotation.equals(other.OldAnnotation)
	}
	return false
}

// Prestring returns this GuardMissing as a Prestring
func (g *GuardMissing) Prestring() Prestring {
	return GuardMissingPrestring{g.OldAnnotation.Prestring()}
}

// GuardMissingPrestring is a Prestring storing the needed information to compactly encode a GuardMissing
type GuardMissingPrestring struct {
	OldPrestring Prestring
}

func (g GuardMissingPrestring) String() string {
	return fmt.Sprintf("%s lacking guarding;", g.OldPrestring.String())
}

// don't modify the ConsumeTrigger and ProduceTrigger objects after construction! Pointers
// to them are duplicated

// A ProduceTrigger represents a point at which a value is produced that may be nilable because of
// an Annotation (ProducingAnnotationTrigger). Will always be paired with a ConsumeTrigger.
// For semantics' sake, the Annotation field of a ProduceTrigger is all that matters - the Expr is
// included only to produce more informative error messages
type ProduceTrigger struct {
	Annotation ProducingAnnotationTrigger
	Expr       ast.Expr
}
