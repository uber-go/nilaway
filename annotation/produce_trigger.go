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

	"go.uber.org/nilaway/util/typeshelper"
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
	// This should be very sparingly used, and only with utter conviction of correctness.
	// Default setting for ProduceTriggers is to not need a guard.
	SetNeedsGuard(bool)

	Repr() fmt.Stringer

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

// Repr returns a compact string representation.
func (*TriggerIfNilable) Repr() fmt.Stringer {
	return TriggerIfNilableRepr{}
}

// TriggerIfNilableRepr is a fmt.Stringer storing the needed information to compactly encode a TriggerIfNilable
type TriggerIfNilableRepr struct{}

func (TriggerIfNilableRepr) String() string {
	return "nilable value"
}

// CheckProduce returns true if the underlying annotation is present in the passed map and nilable
func (t *TriggerIfNilable) CheckProduce(annMap Map) bool {
	ann, ok := t.Ann.Lookup(annMap)
	return ok && ann.IsNilable
}

// NeedsGuardMatch returns true if this trigger needs to be matched with a guarded consumer
func (t *TriggerIfNilable) NeedsGuardMatch() bool { return t.NeedsGuard }

// SetNeedsGuard sets the underlying Guard-Neediness of this ProduceTrigger, if present
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

// Repr returns a compact string representation.
func (*TriggerIfDeepNilable) Repr() fmt.Stringer {
	return TriggerIfDeepNilableRepr{}
}

// TriggerIfDeepNilableRepr is a fmt.Stringer storing the needed information to compactly encode a TriggerIfDeepNilable
type TriggerIfDeepNilableRepr struct{}

func (TriggerIfDeepNilableRepr) String() string {
	return "deeply nilable value"
}

// CheckProduce returns true if the underlying annotation is present in the passed map and deeply nilable
func (t *TriggerIfDeepNilable) CheckProduce(annMap Map) bool {
	ann, ok := t.Ann.Lookup(annMap)
	return ok && ann.IsDeepNilable
}

// NeedsGuardMatch returns true if this trigger needs to be matched with a guarded consumer
func (t *TriggerIfDeepNilable) NeedsGuardMatch() bool { return t.NeedsGuard }

// SetNeedsGuard sets the underlying Guard-Neediness of this ProduceTrigger, if present
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

// NeedsGuardMatch returns true if this trigger needs to be matched with a guarded consumer
func (p *ProduceTriggerTautology) NeedsGuardMatch() bool {
	return p.NeedsGuard
}

// SetNeedsGuard sets the underlying Guard-Neediness of this ProduceTrigger, if present
func (p *ProduceTriggerTautology) SetNeedsGuard(b bool) { p.NeedsGuard = b }

// Repr returns a compact string representation.
func (*ProduceTriggerTautology) Repr() fmt.Stringer {
	return ProduceTriggerTautologyRepr{}
}

// ProduceTriggerTautologyRepr is a fmt.Stringer storing the needed information to compactly encode a ProduceTriggerTautology
type ProduceTriggerTautologyRepr struct{}

func (ProduceTriggerTautologyRepr) String() string {
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

// Repr returns a compact string representation.
func (*ProduceTriggerNever) Repr() fmt.Stringer {
	return ProduceTriggerNeverRepr{}
}

// ProduceTriggerNeverRepr is a fmt.Stringer storing the needed information to compactly encode a ProduceTriggerNever
type ProduceTriggerNeverRepr struct{}

func (ProduceTriggerNeverRepr) String() string {
	return "is not nilable"
}

// CheckProduce returns true false
func (*ProduceTriggerNever) CheckProduce(Map) bool {
	return false
}

// NeedsGuardMatch returns true if this trigger needs to be matched with a guarded consumer
func (p *ProduceTriggerNever) NeedsGuardMatch() bool { return p.NeedsGuard }

// SetNeedsGuard sets the underlying Guard-Neediness of this ProduceTrigger, if present
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

// Repr returns a compact string representation.
func (*PositiveNilCheck) Repr() fmt.Stringer {
	return PositiveNilCheckRepr{}
}

// PositiveNilCheckRepr is a fmt.Stringer storing the needed information to compactly encode a PositiveNilCheck
type PositiveNilCheckRepr struct{}

func (PositiveNilCheckRepr) String() string {
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

// Repr returns a compact string representation.
func (*NegativeNilCheck) Repr() fmt.Stringer {
	return NegativeNilCheckRepr{}
}

// NegativeNilCheckRepr is a fmt.Stringer storing the needed information to compactly encode a NegativeNilCheck
type NegativeNilCheckRepr struct{}

func (NegativeNilCheckRepr) String() string {
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

// Repr returns a compact string representation.
func (*ConstNil) Repr() fmt.Stringer {
	return ConstNilRepr{}
}

// ConstNilRepr is a fmt.Stringer storing the needed information to compactly encode a ConstNil
type ConstNilRepr struct{}

func (ConstNilRepr) String() string {
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

// Repr returns a compact string representation.
func (*UnassignedFld) Repr() fmt.Stringer {
	return UnassignedFldRepr{}
}

// UnassignedFldRepr is a fmt.Stringer storing the needed information to compactly encode a UnassignedFld
type UnassignedFldRepr struct{}

func (UnassignedFldRepr) String() string {
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

// Repr returns a compact string representation.
func (n *NoVarAssign) Repr() fmt.Stringer {
	return NoVarAssignRepr{
		VarName: n.VarObj.Name(),
	}
}

// NoVarAssignRepr is a fmt.Stringer storing the needed information to compactly encode a NoVarAssign
type NoVarAssignRepr struct {
	VarName string
}

func (n NoVarAssignRepr) String() string {
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

// Repr returns a compact string representation.
func (*BlankVarReturn) Repr() fmt.Stringer {
	return BlankVarReturnRepr{}
}

// BlankVarReturnRepr is a fmt.Stringer storing the needed information to compactly encode a BlankVarReturn
type BlankVarReturnRepr struct{}

func (BlankVarReturnRepr) String() string {
	return "return via a blank variable `_`"
}

// DuplicateParamProducer duplicates a given produce trigger, assuming the given produce trigger
// is of FuncParam.
func DuplicateParamProducer(t *ProduceTrigger, location token.Position) *ProduceTrigger {
	key := t.Annotation.(*FuncParam).Ann.(*ParamAnnotationKey)
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

// Repr returns this FuncParam as a fmt.Stringer
func (f *FuncParam) Repr() fmt.Stringer {
	switch key := f.Ann.(type) {
	case *ParamAnnotationKey:
		return FuncParamRepr{key.ParamNameString(), key.FuncDecl.Name(), ""}
	case *CallSiteParamAnnotationKey:
		return FuncParamRepr{key.ParamNameString(), key.FuncDecl.Name(), key.Location.String()}
	default:
		panic(fmt.Sprintf("Expected ParamAnnotationKey or CallSiteParamAnnotationKey but got: %T", key))
	}
}

// FuncParamRepr is a fmt.Stringer storing the needed information to compactly encode a FuncParam
type FuncParamRepr struct {
	ParamName string
	FuncName  string
	// Location is empty for a FuncParam enclosing ParamAnnotationKey. Location points to the
	// location of the argument pass at the call site for a FuncParam enclosing CallSiteParamAnnotationKey.
	Location string
}

func (f FuncParamRepr) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "function parameter `%s`", f.ParamName)
	if f.Location != "" {
		fmt.Fprintf(&sb, " at %s", f.Location)
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

// Repr returns this MethodRecv as a fmt.Stringer
func (m *MethodRecv) Repr() fmt.Stringer {
	return MethodRecvRepr{m.VarDecl.Name()}
}

// MethodRecvRepr is a fmt.Stringer storing the needed information to compactly encode a MethodRecv
type MethodRecvRepr struct {
	RecvName string
}

func (m MethodRecvRepr) String() string {
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

// Repr returns this MethodRecv as a fmt.Stringer
func (m *MethodRecvDeep) Repr() fmt.Stringer {
	return MethodRecvDeepRepr{m.VarDecl.Name()}
}

// MethodRecvDeepRepr is a fmt.Stringer storing the needed information to compactly encode a MethodRecv
type MethodRecvDeepRepr struct {
	RecvName string
}

func (m MethodRecvDeepRepr) String() string {
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

// Repr returns a compact string representation.
func (v *VariadicFuncParam) Repr() fmt.Stringer {
	return VariadicFuncParamRepr{v.VarDecl.Name()}
}

// VariadicFuncParamRepr is a fmt.Stringer storing the needed information to compactly encode a VariadicFuncParam
type VariadicFuncParamRepr struct {
	ParamName string
}

func (v VariadicFuncParamRepr) String() string {
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

// Repr returns a compact string representation.
func (*TrustedFuncNilable) Repr() fmt.Stringer {
	return TrustedFuncNilableRepr{}
}

// TrustedFuncNilableRepr is a fmt.Stringer storing the needed information to compactly encode a TrustedFuncNilable
type TrustedFuncNilableRepr struct{}

func (TrustedFuncNilableRepr) String() string {
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

// Repr returns a compact string representation.
func (*TrustedFuncNonnil) Repr() fmt.Stringer {
	return TrustedFuncNonnilRepr{}
}

// TrustedFuncNonnilRepr is a fmt.Stringer storing the needed information to compactly encode a TrustedFuncNonnil
type TrustedFuncNonnilRepr struct{}

func (TrustedFuncNonnilRepr) String() string {
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

// Repr returns this FldRead as a fmt.Stringer
func (f *FldRead) Repr() fmt.Stringer {
	if ek, ok := f.Ann.(*EscapeFieldAnnotationKey); ok {
		return FldReadRepr{ek.FieldDecl.Name()}
	}
	return FldReadRepr{f.Ann.(*FieldAnnotationKey).FieldDecl.Name()}
}

// FldReadRepr is a fmt.Stringer storing the needed information to compactly encode a FldRead
type FldReadRepr struct {
	FieldName string
}

func (f FldReadRepr) String() string {
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

// Repr returns this ParamFldRead as a fmt.Stringer
func (f *ParamFldRead) Repr() fmt.Stringer {
	ann := f.Ann.(*ParamFieldAnnotationKey)
	return ParamFldReadRepr{
		FieldName: ann.FieldDecl.Name(),
	}
}

// ParamFldReadRepr is a fmt.Stringer storing the needed information to compactly encode a ParamFldRead
type ParamFldReadRepr struct {
	FieldName string
}

func (f ParamFldReadRepr) String() string {
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
	return f.Repr().String()
}

// Repr returns this FldReturn as a fmt.Stringer
func (f *FldReturn) Repr() fmt.Stringer {
	key := f.Ann.(*RetFieldAnnotationKey)
	return FldReturnRepr{key.RetNum, key.FuncDecl.Name(), key.FieldDecl.Name()}
}

// FldReturnRepr is a fmt.Stringer storing the needed information to compactly encode a FldReturn
type FldReturnRepr struct {
	RetNum    int
	FuncName  string
	FieldName string
}

func (f FldReturnRepr) String() string {
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

	IsFromRichCheckEffectFunc bool
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (f *FuncReturn) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*FuncReturn); ok {
		return f.TriggerIfNilable.equals(other.TriggerIfNilable) &&
			f.IsFromRichCheckEffectFunc == other.IsFromRichCheckEffectFunc
	}
	return false
}

// Repr returns this FuncReturn as a fmt.Stringer
func (f *FuncReturn) Repr() fmt.Stringer {
	switch key := f.Ann.(type) {
	case *RetAnnotationKey:
		return FuncReturnRepr{key.RetNum, key.FuncDecl.Name(), ""}
	case *CallSiteRetAnnotationKey:
		return FuncReturnRepr{key.RetNum, key.FuncDecl.Name(), key.Location.String()}
	default:
		panic(fmt.Sprintf("Expected RetAnnotationKey or CallSiteRetAnnotationKey but got: %T", key))
	}
}

// FuncReturnRepr is a fmt.Stringer storing the needed information to compactly encode a FuncReturn
type FuncReturnRepr struct {
	RetNum   int
	FuncName string
	// Location is empty for a FuncReturn enclosing RetAnnotationKey. Location points to the
	// location of the result return at the call site for a FuncReturn enclosing CallSiteRetAnnotationKey.
	Location string
}

func (f FuncReturnRepr) String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "result %d of `%s()`", f.RetNum, f.FuncName)
	if f.Location != "" {
		fmt.Fprintf(&sb, " at %s", f.Location)
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

// Repr returns this MethodReturn as a fmt.Stringer
func (m *MethodReturn) Repr() fmt.Stringer {
	retKey := m.Ann.(*RetAnnotationKey)
	return MethodReturnRepr{retKey.RetNum, retKey.FuncDecl.Name()}
}

// MethodReturnRepr is a fmt.Stringer storing the needed information to compactly encode a MethodReturn
type MethodReturnRepr struct {
	RetNum   int
	FuncName string
}

func (m MethodReturnRepr) String() string {
	return fmt.Sprintf("result %d of `%s()`", m.RetNum, m.FuncName)
}

// MethodResultReachesInterface is used when a result of a method is determined to flow into a result of an interface using inheritance
type MethodResultReachesInterface struct {
	*TriggerIfNilable
	AffiliationPair
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (m *MethodResultReachesInterface) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*MethodResultReachesInterface); ok {
		return m.TriggerIfNilable.equals(other.TriggerIfNilable) &&
			m.InterfaceMethod == other.InterfaceMethod &&
			m.ImplementingMethod == other.ImplementingMethod
	}
	return false
}

// Repr returns this MethodResultReachesInterface as a fmt.Stringer
func (m *MethodResultReachesInterface) Repr() fmt.Stringer {
	retAnn := m.Ann.(*RetAnnotationKey)
	return MethodResultReachesInterfaceRepr{
		retAnn.RetNum,
		typeshelper.PartiallyQualifiedFuncName(retAnn.FuncDecl),
		typeshelper.PartiallyQualifiedFuncName(m.InterfaceMethod),
	}
}

// MethodResultReachesInterfaceRepr is a fmt.Stringer storing the needed information to compactly encode a MethodResultReachesInterface
type MethodResultReachesInterfaceRepr struct {
	RetNum   int
	ImplName string
	IntName  string
}

func (m MethodResultReachesInterfaceRepr) String() string {
	return ""
}

// InterfaceParamReachesImplementation is used when a param of a method is determined to flow into the param of an implementing method
type InterfaceParamReachesImplementation struct {
	*TriggerIfNilable
	AffiliationPair
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (i *InterfaceParamReachesImplementation) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*InterfaceParamReachesImplementation); ok {
		return i.TriggerIfNilable.equals(other.TriggerIfNilable) &&
			i.InterfaceMethod == other.InterfaceMethod &&
			i.ImplementingMethod == other.ImplementingMethod
	}
	return false
}

// Repr returns this InterfaceParamReachesImplementation as a fmt.Stringer
func (i *InterfaceParamReachesImplementation) Repr() fmt.Stringer {
	paramAnn := i.Ann.(*ParamAnnotationKey)
	return InterfaceParamReachesImplementationRepr{
		paramAnn.ParamNameString(),
		typeshelper.PartiallyQualifiedFuncName(paramAnn.FuncDecl),
		typeshelper.PartiallyQualifiedFuncName(i.ImplementingMethod),
	}
}

// InterfaceParamReachesImplementationRepr is a fmt.Stringer storing the needed information to compactly encode a InterfaceParamReachesImplementation
type InterfaceParamReachesImplementationRepr struct {
	ParamName string
	IntName   string
	ImplName  string
}

func (i InterfaceParamReachesImplementationRepr) String() string {
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

// Repr returns this GlobalVarRead as a fmt.Stringer
func (g *GlobalVarRead) Repr() fmt.Stringer {
	key := g.Ann.(*GlobalVarAnnotationKey)
	return GlobalVarReadRepr{
		key.VarDecl.Name(),
	}
}

// GlobalVarReadRepr is a fmt.Stringer storing the needed information to compactly encode a GlobalVarRead
type GlobalVarReadRepr struct {
	VarName string
}

func (g GlobalVarReadRepr) String() string {
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

// Repr returns this MapRead as a fmt.Stringer
func (m *MapRead) Repr() fmt.Stringer {
	key := m.Ann.(*TypeNameAnnotationKey)
	return MapReadRepr{key.TypeDecl.Name()}
}

// MapReadRepr is a fmt.Stringer storing the needed information to compactly encode a MapRead
type MapReadRepr struct {
	TypeName string
}

func (m MapReadRepr) String() string {
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

// Repr returns this ArrayRead as a fmt.Stringer
func (a *ArrayRead) Repr() fmt.Stringer {
	key := a.Ann.(*TypeNameAnnotationKey)
	return ArrayReadRepr{key.TypeDecl.Name()}
}

// ArrayReadRepr is a fmt.Stringer storing the needed information to compactly encode a ArrayRead
type ArrayReadRepr struct {
	TypeName string
}

func (a ArrayReadRepr) String() string {
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

// Repr returns this SliceRead as a fmt.Stringer
func (s *SliceRead) Repr() fmt.Stringer {
	key := s.Ann.(*TypeNameAnnotationKey)
	return SliceReadRepr{key.TypeDecl.Name()}
}

// SliceReadRepr is a fmt.Stringer storing the needed information to compactly encode a SliceRead
type SliceReadRepr struct {
	TypeName string
}

func (s SliceReadRepr) String() string {
	return fmt.Sprintf("index of a slice of type `%s`", s.TypeName)
}

// PtrRead is when a value is determined to flow from a read to a pointer
type PtrRead struct {
	*TriggerIfDeepNilable
}

// Repr returns this PtrRead as a fmt.Stringer
func (p *PtrRead) Repr() fmt.Stringer {
	key := p.Ann.(*TypeNameAnnotationKey)
	return PtrReadRepr{key.TypeDecl.Name()}
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (p *PtrRead) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*PtrRead); ok {
		return p.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// PtrReadRepr is a fmt.Stringer storing the needed information to compactly encode a PtrRead
type PtrReadRepr struct {
	TypeName string
}

func (p PtrReadRepr) String() string {
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

// Repr returns this ChanRecv as a fmt.Stringer
func (c *ChanRecv) Repr() fmt.Stringer {
	key := c.Ann.(*TypeNameAnnotationKey)
	return ChanRecvRepr{key.TypeDecl.Name()}
}

// ChanRecvRepr is a fmt.Stringer storing the needed information to compactly encode a ChanRecv
type ChanRecvRepr struct {
	TypeName string
}

func (c ChanRecvRepr) String() string {
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

// Repr returns this FuncParamDeep as a fmt.Stringer
func (f *FuncParamDeep) Repr() fmt.Stringer {
	key := f.Ann.(*ParamAnnotationKey)
	return FuncParamDeepRepr{key.ParamNameString()}
}

// FuncParamDeepRepr is a fmt.Stringer storing the needed information to compactly encode a FuncParamDeep
type FuncParamDeepRepr struct {
	ParamName string
}

func (f FuncParamDeepRepr) String() string {
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

// Repr returns this VariadicFuncParamDeep as a fmt.Stringer
func (v *VariadicFuncParamDeep) Repr() fmt.Stringer {
	return VariadicFuncParamDeepRepr{v.Ann.(*ParamAnnotationKey).ParamNameString()}
}

// VariadicFuncParamDeepRepr is a fmt.Stringer storing the needed information to compactly encode a VariadicFuncParamDeep
type VariadicFuncParamDeepRepr struct {
	ParamName string
}

func (v VariadicFuncParamDeepRepr) String() string {
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

// Repr returns this FuncReturnDeep as a fmt.Stringer
func (f *FuncReturnDeep) Repr() fmt.Stringer {
	key := f.Ann.(*RetAnnotationKey)
	return FuncReturnDeepRepr{key.RetNum, key.FuncDecl.Name()}
}

// FuncReturnDeepRepr is a fmt.Stringer storing the needed information to compactly encode a FuncReturnDeep
type FuncReturnDeepRepr struct {
	RetNum   int
	FuncName string
}

func (f FuncReturnDeepRepr) String() string {
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

// Repr returns this FldReadDeep as a fmt.Stringer
func (f *FldReadDeep) Repr() fmt.Stringer {
	key := f.Ann.(*FieldAnnotationKey)
	return FldReadDeepRepr{key.FieldDecl.Name()}
}

// FldReadDeepRepr is a fmt.Stringer storing the needed information to compactly encode a FldReadDeep
type FldReadDeepRepr struct {
	FieldName string
}

func (f FldReadDeepRepr) String() string {
	return fmt.Sprintf("deep read from field `%s`", f.FieldName)
}

// LocalVarReadDeep is when a value is determined to flow deeply from a local variable.
type LocalVarReadDeep struct {
	*TriggerIfDeepNilable
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one
func (v *LocalVarReadDeep) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*LocalVarReadDeep); ok {
		return v.TriggerIfDeepNilable.equals(other.TriggerIfDeepNilable)
	}
	return false
}

// Repr returns this LocalVarReadDeep as a fmt.Stringer
func (v LocalVarReadDeep) Repr() fmt.Stringer {
	varAnn := v.Ann.(*LocalVarAnnotationKey)
	return LocalVarReadDeepRepr{varAnn.VarDecl.Name()}
}

// LocalVarReadDeepRepr is a fmt.Stringer storing the needed information to compactly encode a LocalVarReadDeep
type LocalVarReadDeepRepr struct {
	VarName string
}

func (v LocalVarReadDeepRepr) String() string {
	return fmt.Sprintf("deep read from local variable `%s`", v.VarName)
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

// Repr returns this GlobalVarReadDeep as a fmt.Stringer
func (g *GlobalVarReadDeep) Repr() fmt.Stringer {
	key := g.Ann.(*GlobalVarAnnotationKey)
	return GlobalVarReadDeepRepr{key.VarDecl.Name()}
}

// GlobalVarReadDeepRepr is a fmt.Stringer storing the needed information to compactly encode a GlobalVarReadDeep
type GlobalVarReadDeepRepr struct {
	VarName string
}

func (g GlobalVarReadDeepRepr) String() string {
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

// Repr returns this GuardMissing as a fmt.Stringer
func (g *GuardMissing) Repr() fmt.Stringer {
	return GuardMissingRepr{g.OldAnnotation.Repr()}
}

// GuardMissingRepr is a fmt.Stringer storing the needed information to compactly encode a GuardMissing
type GuardMissingRepr struct {
	// OldPrestring retains the existing gob field name for fact encoding compatibility.
	OldPrestring fmt.Stringer
}

func (g GuardMissingRepr) String() string {
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
