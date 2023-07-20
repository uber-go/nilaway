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
	"go/types"

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
	SetNeedsGuard(bool) ProducingAnnotationTrigger

	String() string
	Prestring() Prestring

	// Kind returns the kind of the trigger.
	Kind() TriggerKind

	// UnderlyingSite returns the underlying site this trigger's nilability depends on. If the
	// trigger always or never fires, the site is nil.
	UnderlyingSite() Key
}

// TriggerIfNilable is a general trigger indicating that the bad case occurs when a certain Annotation
// key is nilable
type TriggerIfNilable struct {
	Ann Key
}

func (t TriggerIfNilable) String() string {
	return t.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (TriggerIfNilable) Prestring() Prestring {
	return TriggerIfNilablePrestring{}
}

// TriggerIfNilablePrestring is a Prestring storing the needed information to compactly encode a TriggerIfNilable
type TriggerIfNilablePrestring struct{}

func (TriggerIfNilablePrestring) String() string {
	return "could be nilable"
}

// CheckProduce returns true if the underlying annotation is present in the passed map and nilable
func (t TriggerIfNilable) CheckProduce(annMap Map) bool {
	ann, ok := t.Ann.Lookup(annMap)
	return ok && ann.IsNilable
}

// NeedsGuardMatch for a `TriggerIfNilable` is default false, as guarding
// applies mostly to deep reads, but this behavior is overriden
// for `VariadicFuncParamDeep`s, which have the semantics of
// deep reads despite consulting shallow annotations
func (TriggerIfNilable) NeedsGuardMatch() bool { return false }

// SetNeedsGuard for a `TriggerIfNilable` is, by default, a noop, as guarding
// applies mostly to deep reads, but this behavior is overriden
// for `VariadicFuncParamDeep`s, which have the semantics of
// deep reads despite consulting shallow annotations
func (t TriggerIfNilable) SetNeedsGuard(bool) ProducingAnnotationTrigger { return t }

// Kind returns Conditional.
func (t TriggerIfNilable) Kind() TriggerKind { return Conditional }

// UnderlyingSite returns the underlying site this trigger's nilability depends on.
func (t TriggerIfNilable) UnderlyingSite() Key { return t.Ann }

// TriggerIfDeepNilable is a general trigger indicating the the bad case occurs when a certain Annotation
// key is deeply nilable
type TriggerIfDeepNilable struct {
	Ann Key
}

func (t TriggerIfDeepNilable) String() string {
	return t.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (TriggerIfDeepNilable) Prestring() Prestring {
	return TriggerIfDeepNilablePrestring{}
}

// TriggerIfDeepNilablePrestring is a Prestring storing the needed information to compactly encode a TriggerIfDeepNilable
type TriggerIfDeepNilablePrestring struct{}

func (TriggerIfDeepNilablePrestring) String() string {
	return "could be nilable"
}

// CheckProduce returns true if the underlying annotation is present in the passed map and deeply nilable
func (t TriggerIfDeepNilable) CheckProduce(annMap Map) bool {
	ann, ok := t.Ann.Lookup(annMap)
	return ok && ann.IsDeepNilable
}

// NeedsGuardMatch for a `TriggerIfDeepNilable` is default false,
// but overridden for most concrete triggers to read a boolean
// field
func (TriggerIfDeepNilable) NeedsGuardMatch() bool { return false }

// SetNeedsGuard for a `TriggerIfDeepNilable` is, by default, a noop,
// but overridden for most concrete triggers to set an underlying field
func (t TriggerIfDeepNilable) SetNeedsGuard(bool) ProducingAnnotationTrigger { return t }

// Kind returns DeepConditional.
func (t TriggerIfDeepNilable) Kind() TriggerKind { return DeepConditional }

// UnderlyingSite returns the underlying site this trigger's nilability depends on.
func (t TriggerIfDeepNilable) UnderlyingSite() Key { return t.Ann }

// ProduceTriggerTautology is used for trigger producers that will always result in nil
type ProduceTriggerTautology struct{}

// CheckProduce returns true
func (ProduceTriggerTautology) CheckProduce(Map) bool {
	return true
}

// NeedsGuardMatch for a ProduceTriggerTautology is false - there is no wiggle room with these
func (ProduceTriggerTautology) NeedsGuardMatch() bool { return false }

// SetNeedsGuard for a ProduceTriggerTautology is a noop
func (p ProduceTriggerTautology) SetNeedsGuard(bool) ProducingAnnotationTrigger { return p }

func (p ProduceTriggerTautology) String() string {
	return p.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (ProduceTriggerTautology) Prestring() Prestring {
	return ProduceTriggerTautologyPrestring{}
}

// Kind returns Always.
func (ProduceTriggerTautology) Kind() TriggerKind { return Always }

// UnderlyingSite always returns nil.
func (ProduceTriggerTautology) UnderlyingSite() Key { return nil }

// ProduceTriggerTautologyPrestring is a Prestring storing the needed information to compactly encode a ProduceTriggerTautology
type ProduceTriggerTautologyPrestring struct{}

func (ProduceTriggerTautologyPrestring) String() string {
	return "is nilable"
}

// ProduceTriggerNever is used for trigger producers that will never be nil
type ProduceTriggerNever struct{}

func (p ProduceTriggerNever) String() string {
	return p.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (ProduceTriggerNever) Prestring() Prestring {
	return ProduceTriggerNeverPrestring{}
}

// ProduceTriggerNeverPrestring is a Prestring storing the needed information to compactly encode a ProduceTriggerNever
type ProduceTriggerNeverPrestring struct{}

func (ProduceTriggerNeverPrestring) String() string {
	return "could not be nilable"
}

// CheckProduce returns true false
func (ProduceTriggerNever) CheckProduce(Map) bool {
	return false
}

// NeedsGuardMatch for a ProduceTriggerNever is false, like ProduceTriggerTautology
func (ProduceTriggerNever) NeedsGuardMatch() bool { return false }

// SetNeedsGuard for a ProduceTriggerNever is a noop, like ProduceTriggerTautology
func (p ProduceTriggerNever) SetNeedsGuard(bool) ProducingAnnotationTrigger { return p }

// Kind returns Never.
func (ProduceTriggerNever) Kind() TriggerKind { return Never }

// UnderlyingSite always returns nil.
func (ProduceTriggerNever) UnderlyingSite() Key { return nil }

// note: each of the following two productions, ExprOkCheck, and RangeIndexAssignment, should be
// obselete now that we don't add consumptions for basic-typed expressions like ints and bools to
// begin with - TODO: verify that these productions are always no-ops and remove

// ExprOkCheck is used when a value is determined to flow from the second argument of a map or typecast
// operation that necessarily makes it boolean and thus non-nil
type ExprOkCheck struct {
	ProduceTriggerNever
}

// RangeIndexAssignment is used when a value is determined to flow from the first argument of a
// range loop, and thus be an integer and non-nil
type RangeIndexAssignment struct {
	ProduceTriggerNever
}

// PositiveNilCheck is used when a value is checked in a conditional to BE nil
type PositiveNilCheck struct {
	ProduceTriggerTautology
}

func (p PositiveNilCheck) String() string {
	return p.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (PositiveNilCheck) Prestring() Prestring {
	return PositiveNilCheckPrestring{}
}

// PositiveNilCheckPrestring is a Prestring storing the needed information to compactly encode a PositiveNilCheck
type PositiveNilCheckPrestring struct{}

func (PositiveNilCheckPrestring) String() string {
	return "determined to be nil by a conditional check"
}

// NegativeNilCheck is used when a value is checked in a conditional to NOT BE nil
type NegativeNilCheck struct {
	ProduceTriggerNever
}

func (p NegativeNilCheck) String() string {
	return p.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (NegativeNilCheck) Prestring() Prestring {
	return NegativeNilCheckPrestring{}
}

// NegativeNilCheckPrestring is a Prestring storing the needed information to compactly encode a NegativeNilCheck
type NegativeNilCheckPrestring struct{}

func (NegativeNilCheckPrestring) String() string {
	return "determined not to be nil by a conditional check"
}

// OkReadReflCheck is used to produce nonnil for artifacts of successful `ok` forms (e.g., maps, channels, type casts).
// For example, a map value `m` that was read from in a `v, ok := m[k]` check followed by a positive check of `ok`, implies `m` is non-nil.
// This is valid because nil maps contain no keys.
type OkReadReflCheck struct {
	ProduceTriggerNever
}

// RangeOver is used when a value is ranged over - and thus nonnil in its range body
type RangeOver struct {
	ProduceTriggerNever
}

// ConstNil is when a value is determined to flow from a constant nil expression
type ConstNil struct {
	ProduceTriggerTautology
}

func (c ConstNil) String() string {
	return c.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (ConstNil) Prestring() Prestring {
	return ConstNilPrestring{}
}

// ConstNilPrestring is a Prestring storing the needed information to compactly encode a ConstNil
type ConstNilPrestring struct{}

func (ConstNilPrestring) String() string {
	return "literal nil"
}

// UnassignedFld is when a field of struct is not assigned at initialization
type UnassignedFld struct {
	ProduceTriggerTautology
}

func (c UnassignedFld) String() string {
	return c.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (UnassignedFld) Prestring() Prestring {
	return UnassignedFldPrestring{}
}

// UnassignedFldPrestring is a Prestring storing the needed information to compactly encode a UnassignedFld
type UnassignedFldPrestring struct{}

func (UnassignedFldPrestring) String() string {
	return "unassigned at struct initialization"
}

// NoVarAssign is when a value is determined to flow from a variable that wasn't assigned to
type NoVarAssign struct {
	ProduceTriggerTautology
}

func (n NoVarAssign) String() string {
	return n.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (NoVarAssign) Prestring() Prestring {
	return NoVarAssignPrestring{}
}

// NoVarAssignPrestring is a Prestring storing the needed information to compactly encode a NoVarAssign
type NoVarAssignPrestring struct{}

func (NoVarAssignPrestring) String() string {
	return "read from a variable that was never assigned to"
}

// BlankVarReturn is when a value is determined to flow from a blank variable ('_') to a return of the function
type BlankVarReturn struct {
	ProduceTriggerTautology
}

func (BlankVarReturn) String() string {
	return "read from a blank variable that can never be assigned to"
}

// FuncParam is used when a value is determined to flow from a function parameter
type FuncParam struct {
	TriggerIfNilable
}

func (f FuncParam) String() string {
	return f.Prestring().String()
}

// Prestring returns this FuncParam as a Prestring
func (f FuncParam) Prestring() Prestring {
	key := f.Ann.(ParamAnnotationKey)
	return FuncParamPrestring{key.ParamNameString(), key.FuncDecl.Name()}
}

// FuncParamPrestring is a Prestring storing the needed information to compactly encode a FuncParam
type FuncParamPrestring struct {
	ParamName string
	FuncName  string
}

func (f FuncParamPrestring) String() string {
	return fmt.Sprintf("read from the function parameter `%s` of function `%s`", f.ParamName, f.FuncName)
}

// MethodRecv is used when a value is determined to flow from a method receiver
type MethodRecv struct {
	TriggerIfNilable
}

func (m MethodRecv) String() string {
	return m.Prestring().String()
}

// Prestring returns this FuncParam as a Prestring
func (m MethodRecv) Prestring() Prestring {
	key := m.Ann.(RecvAnnotationKey)
	return MethodRecvPrestring{key.FuncDecl.Name()}
}

// MethodRecvPrestring is a Prestring storing the needed information to compactly encode a FuncParam
type MethodRecvPrestring struct {
	FuncName string
}

func (m MethodRecvPrestring) String() string {
	return fmt.Sprintf("read from the receiver of method `%s`", m.FuncName)
}

// MethodRecvDeep is used when a value is determined to flow deeply from a method receiver
type MethodRecvDeep struct {
	TriggerIfDeepNilable
}

func (m MethodRecvDeep) String() string {
	return m.Prestring().String()
}

// Prestring returns this FuncParam as a Prestring
func (m MethodRecvDeep) Prestring() Prestring {
	key := m.Ann.(RecvAnnotationKey)
	return MethodRecvDeepPrestring{key.FuncDecl.Name()}
}

// MethodRecvDeepPrestring is a Prestring storing the needed information to compactly encode a FuncParam
type MethodRecvDeepPrestring struct {
	FuncName string
}

func (m MethodRecvDeepPrestring) String() string {
	return fmt.Sprintf("read deeply from the receiver of method `%s`", m.FuncName)
}

// VariadicFuncParam is used when a value is determined to flow from a variadic function parameter,
// and thus always be nilable
type VariadicFuncParam struct {
	ProduceTriggerTautology
}

func (v VariadicFuncParam) String() string {
	return v.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (VariadicFuncParam) Prestring() Prestring {
	return VariadicFuncParamPrestring{}
}

// VariadicFuncParamPrestring is a Prestring storing the needed information to compactly encode a VariadicFuncParam
type VariadicFuncParamPrestring struct{}

func (VariadicFuncParamPrestring) String() string {
	return "read directly from a variadic function parameter"
}

// TrustedFuncNilable is used when a value is determined to be nilable by a trusted function call
type TrustedFuncNilable struct {
	ProduceTriggerTautology
}

func (t TrustedFuncNilable) String() string {
	return t.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (TrustedFuncNilable) Prestring() Prestring {
	return TrustedFuncNilablePrestring{}
}

// TrustedFuncNilablePrestring is a Prestring storing the needed information to compactly encode a TrustedFuncNilable
type TrustedFuncNilablePrestring struct{}

func (TrustedFuncNilablePrestring) String() string {
	return "determined to be nilable by a trusted function"
}

// TrustedFuncNonnil is used when a value is determined to be nonnil by a trusted function call
type TrustedFuncNonnil struct {
	ProduceTriggerNever
}

func (t TrustedFuncNonnil) String() string {
	return t.Prestring().String()
}

// Prestring returns this Prestring as a Prestring
func (TrustedFuncNonnil) Prestring() Prestring {
	return TrustedFuncNonnilPrestring{}
}

// TrustedFuncNonnilPrestring is a Prestring storing the needed information to compactly encode a TrustedFuncNonnil
type TrustedFuncNonnilPrestring struct{}

func (TrustedFuncNonnilPrestring) String() string {
	return "determined to be nonnil by a trusted function"
}

// FldRead is used when a value is determined to flow from a read to a field
type FldRead struct {
	TriggerIfNilable
}

func (f FldRead) String() string {
	return f.Prestring().String()
}

// Prestring returns this FldRead as a Prestring
func (f FldRead) Prestring() Prestring {
	if ek, ok := f.Ann.(EscapeFieldAnnotationKey); ok {
		return FldReadPrestring{ek.FieldDecl.Name()}
	}
	return FldReadPrestring{f.Ann.(FieldAnnotationKey).FieldDecl.Name()}
}

// FldReadPrestring is a Prestring storing the needed information to compactly encode a FldRead
type FldReadPrestring struct {
	FieldName string
}

func (f FldReadPrestring) String() string {
	return fmt.Sprintf("read from the field `%s`", f.FieldName)
}

// ParamFldRead is used when a struct field value is determined to flow from the param of a function to a consumption
// site within the body of the function
type ParamFldRead struct {
	TriggerIfNilable
}

func (f ParamFldRead) String() string {
	return f.Prestring().String()
}

// Prestring returns this ParamFldRead as a Prestring
func (f ParamFldRead) Prestring() Prestring {
	ann := f.Ann.(ParamFieldAnnotationKey)
	return ParamFldReadPrestring{
		FieldName: ann.FieldDecl.Name(),
		FuncName:  ann.FuncDecl.Name(),
		ParamNum:  ann.ParamNum,
	}
}

// ParamFldReadPrestring is a Prestring storing the needed information to compactly encode a ParamFldRead
type ParamFldReadPrestring struct {
	// FieldName is the name of the field that is read
	FieldName string
	// FuncName is name of the function
	FuncName string
	// ParamNum is the index of the function param. In case of a receiver this is set to const ReceiverParamIndex
	ParamNum int
}

// IsReceiver returns true if the prestring is corresponding to a receiver of a method
func (f ParamFldReadPrestring) IsReceiver() bool {
	return f.ParamNum == ReceiverParamIndex
}

func (f ParamFldReadPrestring) String() string {
	if f.IsReceiver() {
		return fmt.Sprintf("of the field `%s` of receiver of `%s`", f.FieldName, f.FuncName)
	}
	return fmt.Sprintf("of the field `%s` of param %d of `%s`", f.FieldName, f.ParamNum, f.FuncName)
}

// FuncReturn is used when a value is determined to flow from the return of a function
type FuncReturn struct {
	TriggerIfNilable
	Guarded bool
}

func (f FuncReturn) String() string {
	return f.Prestring().String()
}

// Prestring returns this FuncReturn as a Prestring
func (f FuncReturn) Prestring() Prestring {
	retKey := f.Ann.(RetAnnotationKey)
	return FuncReturnPrestring{retKey.RetNum, retKey.FuncDecl.Name()}
}

// FuncReturnPrestring is a Prestring storing the needed information to compactly encode a FuncReturn
type FuncReturnPrestring struct {
	RetNum   int
	FuncName string
}

func (f FuncReturnPrestring) String() string {
	return fmt.Sprintf("returned as result %d from the function `%s`", f.RetNum, f.FuncName)
}

// NeedsGuardMatch for a FuncReturn returns whether this function return is guarded.
// Function returns should be guarded iff they are the non-error return of an error-returning function
func (f FuncReturn) NeedsGuardMatch() bool {
	return f.Guarded
}

// SetNeedsGuard for a FuncReturn sets its Guarded field - but right now there is no valid use case for this
func (f FuncReturn) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	f.Guarded = b
	return f
}

// MethodReturn is used when a value is determined to flow from the return of a method
type MethodReturn struct {
	TriggerIfNilable
}

func (m MethodReturn) String() string {
	return m.Prestring().String()
}

// Prestring returns this MethodReturn as a Prestring
func (m MethodReturn) Prestring() Prestring {
	retKey := m.Ann.(RetAnnotationKey)
	return MethodReturnPrestring{retKey.RetNum, retKey.FuncDecl.Name()}
}

// MethodReturnPrestring is a Prestring storing the needed information to compactly encode a MethodReturn
type MethodReturnPrestring struct {
	RetNum   int
	FuncName string
}

func (m MethodReturnPrestring) String() string {
	return fmt.Sprintf("returned as result %d from the method `%s`", m.RetNum, m.FuncName)
}

// MethodResultReachesInterface is used when a result of a method is determined to flow into a result of an interface using inheritance
type MethodResultReachesInterface struct {
	TriggerIfNilable
	AffiliationPair
}

func (m MethodResultReachesInterface) String() string {
	return m.Prestring().String()
}

// Prestring returns this MethodResultReachesInterface as a Prestring
func (m MethodResultReachesInterface) Prestring() Prestring {
	retAnn := m.Ann.(RetAnnotationKey)
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
	return fmt.Sprintf("could be returned as result %d from the method `%s` (implementing `%s`)",
		m.RetNum, m.ImplName, m.IntName)
}

// InterfaceParamReachesImplementation is used when a param of a method is determined to flow into the param of an implementing method
type InterfaceParamReachesImplementation struct {
	TriggerIfNilable
	AffiliationPair
}

func (i InterfaceParamReachesImplementation) String() string {
	return i.Prestring().String()
}

// Prestring returns this InterfaceParamReachesImplementation as a Prestring
func (i InterfaceParamReachesImplementation) Prestring() Prestring {
	paramAnn := i.Ann.(ParamAnnotationKey)
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
	return fmt.Sprintf("could be passed as param `%s` to the method `%s` (implemented by `%s`)",
		i.ParamName, i.IntName, i.ImplName)
}

// GlobalVarRead is when a value is determined to flow from a read to a global variable
type GlobalVarRead struct {
	TriggerIfNilable
}

func (g GlobalVarRead) String() string {
	return g.Prestring().String()
}

// Prestring returns this GlobalVarRead as a Prestring
func (g GlobalVarRead) Prestring() Prestring {
	key := g.Ann.(GlobalVarAnnotationKey)
	return GlobalVarReadPrestring{
		key.VarDecl.Name(),
	}
}

// GlobalVarReadPrestring is a Prestring storing the needed information to compactly encode a GlobalVarRead
type GlobalVarReadPrestring struct {
	VarName string
}

func (g GlobalVarReadPrestring) String() string {
	return fmt.Sprintf("read from the global variable `%s`", g.VarName)
}

// MapRead is when a value is determined to flow from a map index expression
// These should always be instantiated with NeedsGuard = true
type MapRead struct {
	TriggerIfDeepNilable
	NeedsGuard bool
}

func (m MapRead) String() string {
	return m.Prestring().String()
}

// Prestring returns this MapRead as a Prestring
func (m MapRead) Prestring() Prestring {
	key := m.Ann.(TypeNameAnnotationKey)
	return MapReadPrestring{key.TypeDecl.Name()}
}

// MapReadPrestring is a Prestring storing the needed information to compactly encode a MapRead
type MapReadPrestring struct {
	TypeName string
}

func (m MapReadPrestring) String() string {
	return fmt.Sprintf("read from an index of a map of type `%s`", m.TypeName)
}

// NeedsGuardMatch for a map read is always true - map reads are always intended to be guarded unless checked
func (m MapRead) NeedsGuardMatch() bool { return m.NeedsGuard }

// SetNeedsGuard for a map read sets the field NeedsGuard
func (m MapRead) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	m.NeedsGuard = b
	return m
}

// ArrayRead is when a value is determined to flow from an array index expression
type ArrayRead struct {
	TriggerIfDeepNilable
}

func (a ArrayRead) String() string {
	return a.Prestring().String()
}

// Prestring returns this ArrayRead as a Prestring
func (a ArrayRead) Prestring() Prestring {
	key := a.Ann.(TypeNameAnnotationKey)
	return ArrayReadPrestring{key.TypeDecl.Name()}
}

// ArrayReadPrestring is a Prestring storing the needed information to compactly encode a ArrayRead
type ArrayReadPrestring struct {
	TypeName string
}

func (a ArrayReadPrestring) String() string {
	return fmt.Sprintf("read from an index of an array of type `%s`", a.TypeName)
}

// SliceRead is when a value is determined to flow from a slice index expression
type SliceRead struct {
	TriggerIfDeepNilable
}

func (s SliceRead) String() string {
	return s.Prestring().String()
}

// Prestring returns this SliceRead as a Prestring
func (s SliceRead) Prestring() Prestring {
	key := s.Ann.(TypeNameAnnotationKey)
	return SliceReadPrestring{key.TypeDecl.Name()}
}

// SliceReadPrestring is a Prestring storing the needed information to compactly encode a SliceRead
type SliceReadPrestring struct {
	TypeName string
}

func (s SliceReadPrestring) String() string {
	return fmt.Sprintf("read from an index of a slice of type `%s`", s.TypeName)
}

// PtrRead is when a value is determined to flow from a read to a pointer
type PtrRead struct {
	TriggerIfDeepNilable
}

func (p PtrRead) String() string {
	return p.Prestring().String()
}

// Prestring returns this PtrRead as a Prestring
func (p PtrRead) Prestring() Prestring {
	key := p.Ann.(TypeNameAnnotationKey)
	return PtrReadPrestring{key.TypeDecl.Name()}
}

// PtrReadPrestring is a Prestring storing the needed information to compactly encode a PtrRead
type PtrReadPrestring struct {
	TypeName string
}

func (p PtrReadPrestring) String() string {
	return fmt.Sprintf("read from the value of a pointer of type `%s`", p.TypeName)
}

// ChanRecv is when a value is determined to flow from a channel receive
type ChanRecv struct {
	TriggerIfDeepNilable
	NeedsGuard bool
}

func (c ChanRecv) String() string {
	return c.Prestring().String()
}

// Prestring returns this ChanRecv as a Prestring
func (c ChanRecv) Prestring() Prestring {
	key := c.Ann.(TypeNameAnnotationKey)
	return ChanRecvPrestring{key.TypeDecl.Name()}
}

// ChanRecvPrestring is a Prestring storing the needed information to compactly encode a ChanRecv
type ChanRecvPrestring struct {
	TypeName string
}

func (c ChanRecvPrestring) String() string {
	return fmt.Sprintf("received from a channel of type `%s`", c.TypeName)
}

// NeedsGuardMatch for a ChanRecv reads the field NeedsGuard of the
// struct - set to indicate whether the channel receive is in the `v, ok := <- ch` form
func (c ChanRecv) NeedsGuardMatch() bool { return c.NeedsGuard }

// SetNeedsGuard for a channel receive sets the field NeedsGuard if it is in the `v, ok := <- ch` form
func (c ChanRecv) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	c.NeedsGuard = b
	return c
}

// FuncParamDeep is used when a value is determined to flow deeply from a function parameter
type FuncParamDeep struct {
	TriggerIfDeepNilable
	NeedsGuard bool
}

func (f FuncParamDeep) String() string {
	return f.Prestring().String()
}

// Prestring returns this FuncParamDeep as a Prestring
func (f FuncParamDeep) Prestring() Prestring {
	key := f.Ann.(ParamAnnotationKey)
	return FuncParamDeepPrestring{key.ParamNameString(), key.FuncDecl.Name()}
}

// FuncParamDeepPrestring is a Prestring storing the needed information to compactly encode a FuncParamDeep
type FuncParamDeepPrestring struct {
	ParamName string
	FuncName  string
}

func (f FuncParamDeepPrestring) String() string {
	return fmt.Sprintf("read deeply from the parameter `%s` of function `%s`", f.ParamName, f.FuncName)
}

// NeedsGuardMatch for a FuncParamDeep reads the field NeedsGuard of the
// struct - set to indicate whether the func param is of type `map` or `channel`
func (f FuncParamDeep) NeedsGuardMatch() bool { return f.NeedsGuard }

// SetNeedsGuard for a FuncParamDeep sets the field NeedsGuard
func (f FuncParamDeep) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	f.NeedsGuard = b
	return f
}

// VariadicFuncParamDeep is used when a value is determined to flow deeply from a variadic function
// parameter, and thus be nilable iff the shallow Annotation on that parameter is nilable
type VariadicFuncParamDeep struct {
	TriggerIfNilable
	NeedsGuard bool
}

func (v VariadicFuncParamDeep) String() string {
	return v.Prestring().String()
}

// Prestring returns this VariadicFuncParamDeep as a Prestring
func (v VariadicFuncParamDeep) Prestring() Prestring {
	key := v.Ann.(ParamAnnotationKey)
	return VariadicFuncParamDeepPrestring{key.ParamNameString(), key.FuncDecl.Name()}
}

// VariadicFuncParamDeepPrestring is a Prestring storing the needed information to compactly encode a VariadicFuncParamDeep
type VariadicFuncParamDeepPrestring struct {
	ParamName string
	FuncName  string
}

func (v VariadicFuncParamDeepPrestring) String() string {
	return fmt.Sprintf("read as an index of the variadic parameter `%s` of function `%s`",
		v.ParamName, v.FuncName)
}

// NeedsGuardMatch for a VariadicFuncParamDeep reads the field NeedsGuard of the
// struct - set to indicate whether the variadic func param is of type `map` or `channel`
func (v VariadicFuncParamDeep) NeedsGuardMatch() bool { return v.NeedsGuard }

// SetNeedsGuard for a VariadicFuncParamDeep sets its underlying field NeedsGuard
func (v VariadicFuncParamDeep) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	v.NeedsGuard = b
	return v
}

// FuncReturnDeep is used when a value is determined to flow from the deep Annotation of the return
// of a function
type FuncReturnDeep struct {
	TriggerIfDeepNilable
	NeedsGuard bool
}

func (f FuncReturnDeep) String() string {
	return f.Prestring().String()
}

// Prestring returns this FuncReturnDeep as a Prestring
func (f FuncReturnDeep) Prestring() Prestring {
	key := f.Ann.(RetAnnotationKey)
	return FuncReturnDeepPrestring{key.RetNum, key.FuncDecl.Name()}
}

// FuncReturnDeepPrestring is a Prestring storing the needed information to compactly encode a FuncReturnDeep
type FuncReturnDeepPrestring struct {
	RetNum   int
	FuncName string
}

func (f FuncReturnDeepPrestring) String() string {
	return fmt.Sprintf("read deeply from result %d of the function `%s`", f.RetNum, f.FuncName)
}

// NeedsGuardMatch for a FuncReturnDeep reads the field NeedsGuard of the
// struct - set to indicate whether the func return is of type `map` or `channel`
func (f FuncReturnDeep) NeedsGuardMatch() bool { return f.NeedsGuard }

// SetNeedsGuard for a FuncReturnDeep sets the field NeedsGuard
func (f FuncReturnDeep) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	f.NeedsGuard = b
	return f
}

// FldReadDeep is used when a value is determined to flow from the deep Annotation of a field that is
// read and then indexed into - for example x.f[0]
type FldReadDeep struct {
	TriggerIfDeepNilable
	NeedsGuard bool
}

func (f FldReadDeep) String() string {
	return f.Prestring().String()
}

// Prestring returns this FldReadDeep as a Prestring
func (f FldReadDeep) Prestring() Prestring {
	key := f.Ann.(FieldAnnotationKey)
	return FldReadDeepPrestring{key.FieldDecl.Name()}
}

// FldReadDeepPrestring is a Prestring storing the needed information to compactly encode a FldReadDeep
type FldReadDeepPrestring struct {
	FieldName string
}

func (f FldReadDeepPrestring) String() string {
	return fmt.Sprintf("read deeply from the field `%s`", f.FieldName)
}

// NeedsGuardMatch for a FldReadDeep reads the field NeedsGuard of the
// struct - set to indicate whether the field read is of type `map` or `channel`
func (f FldReadDeep) NeedsGuardMatch() bool { return f.NeedsGuard }

// SetNeedsGuard for a FldReadDeep sets its underlying field NeedsGuard
func (f FldReadDeep) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	f.NeedsGuard = b
	return f
}

// LocalVarReadDeep is when a value is determined to flow deeply from a local variable. It is never nilable
// if appropriately guarded.
type LocalVarReadDeep struct {
	ProduceTriggerNever
	NeedsGuard bool
	ReadVar    *types.Var
}

func (v LocalVarReadDeep) String() string {
	return v.Prestring().String()
}

// Prestring returns this LocalVarReadDeep as a Prestring
func (v LocalVarReadDeep) Prestring() Prestring {
	return LocalVarReadDeepPrestring{v.ReadVar.Name()}
}

// LocalVarReadDeepPrestring is a Prestring storing the needed information to compactly encode a LocalVarReadDeep
type LocalVarReadDeepPrestring struct {
	VarName string
}

func (v LocalVarReadDeepPrestring) String() string {
	return fmt.Sprintf("read deeply from the variable `%s`", v.VarName)
}

// NeedsGuardMatch for a LocalVarReadDeep reads the field NeedsGuard of the
// struct - set to indicate whether the global variable is of map or channel type
func (v LocalVarReadDeep) NeedsGuardMatch() bool { return v.NeedsGuard }

// SetNeedsGuard for a VarReadDeep writes the field NeedsGuard
func (v LocalVarReadDeep) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	v.NeedsGuard = b
	return v
}

// GlobalVarReadDeep is when a value is determined to flow from the deep Annotation of a global variable
// that is read and indexed into
type GlobalVarReadDeep struct {
	TriggerIfDeepNilable
	NeedsGuard bool
}

func (g GlobalVarReadDeep) String() string {
	return g.Prestring().String()
}

// Prestring returns this GlobalVarReadDeep as a Prestring
func (g GlobalVarReadDeep) Prestring() Prestring {
	key := g.Ann.(GlobalVarAnnotationKey)
	return GlobalVarReadDeepPrestring{key.VarDecl.Name()}
}

// GlobalVarReadDeepPrestring is a Prestring storing the needed information to compactly encode a GlobalVarReadDeep
type GlobalVarReadDeepPrestring struct {
	VarName string
}

func (g GlobalVarReadDeepPrestring) String() string {
	return fmt.Sprintf("read deeply from the global variable `%s`", g.VarName)
}

// NeedsGuardMatch for a GlobalVarReadDeep reads the field NeedsGuard of the
// struct - set to indicate whether the global variable is of type `map` or `channel`
func (g GlobalVarReadDeep) NeedsGuardMatch() bool { return g.NeedsGuard }

// SetNeedsGuard for a GlobalVarReadDeep writes the field NeedsGuard
func (g GlobalVarReadDeep) SetNeedsGuard(b bool) ProducingAnnotationTrigger {
	g.NeedsGuard = b
	return g
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
	ProduceTriggerTautology
	OldAnnotation ProducingAnnotationTrigger
}

func (g GuardMissing) String() string {
	return g.Prestring().String()
}

// Prestring returns this GuardMissing as a Prestring
func (g GuardMissing) Prestring() Prestring {
	return GuardMissingPrestring{g.OldAnnotation.Prestring()}
}

// GuardMissingPrestring is a Prestring storing the needed information to compactly encode a GuardMissing
type GuardMissingPrestring struct {
	OldPrestring Prestring
}

func (g GuardMissingPrestring) String() string {
	return fmt.Sprintf("%s lacking guarding", g.OldPrestring.String())
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
