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
	"go/token"
	"go/types"
)

// A Key is an object that can be looked up in a Map
type Key interface {
	// Lookup checks whether this key is present in a given Map - returning false as its
	// second result if not present, and true as its second result with the Val found if
	// one is found.
	// To provide optimistic defaults for unannotated files (formally - files on which the annotations
	// checker has not been run), uses of `Lookup` such as `CheckProduce` and `CheckConsume` always
	// return false (i.e. "don't trigger") if the key they wrap is not found in the map.
	// Since not triggering on the level of a produce or consume trigger always results in fewer
	// errors, this gives optimistic defaults to library code.
	Lookup(Map) (Val, bool)

	// Object returns the underlying object that this annotation key can be interpreted as annotating
	Object() types.Object

	// String returns a string representation of this annotation key
	// These get stored into PrimitiveAnnotationKeys - so KEEP THEM COMPACT
	// a good guideline would be the length of their name plus no more than 10 characters
	String() string

	// equals returns true if the passed key is equal to this key
	equals(Key) bool
}

// FieldAnnotationKey allows the Lookup of a field's Annotation in the Annotation map
type FieldAnnotationKey struct {
	FieldDecl *types.Var
}

// Lookup looks this key up in the passed map, returning a Val
func (k *FieldAnnotationKey) Lookup(annMap Map) (Val, bool) {
	if val, ok := annMap.CheckFieldAnn(k.FieldDecl); ok {
		return val, true
	}
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (k *FieldAnnotationKey) Object() types.Object {
	return k.FieldDecl
}

// equals returns true if the passed key is equal to this key
func (k *FieldAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*FieldAnnotationKey); ok {
		return *k == *other
	}
	return false
}

func (k *FieldAnnotationKey) String() string {
	return fmt.Sprintf("Field %s", k.FieldDecl.Name())
}

// CallSiteParamAnnotationKey is similar to ParamAnnotationKey but it represents the site in the
// caller where the actual argument is passed to the called function. For the same parameter of the
// same function, there is only one distinct ParamAnnotationKey but there is a new
// CallSiteParamAnnotationKey for the parameter for every call of the same function.
type CallSiteParamAnnotationKey struct {
	FuncDecl *types.Func
	ParamNum int
	Location token.Position
}

// ParamName returns the *types.Var naming the parameter associate with this key.
// nilable(result 0)
func (pk *CallSiteParamAnnotationKey) ParamName() *types.Var {
	return pk.FuncDecl.Type().(*types.Signature).Params().At(pk.ParamNum)
}

// Lookup looks this key up in the passed map, returning a Val.
func (pk *CallSiteParamAnnotationKey) Lookup(annMap Map) (Val, bool) {
	if paramVal, ok := annMap.CheckFuncCallSiteParamAnn(pk); ok {
		return paramVal, true
	}
	// Revert to the function's ParamAnnotationKey look up if there is no call-site annotation.
	if paramVal, ok := annMap.CheckFuncParamAnn(pk.FuncDecl, pk.ParamNum); ok {
		return paramVal, true
	}
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating.
func (pk *CallSiteParamAnnotationKey) Object() types.Object {
	return pk.FuncDecl
}

// equals returns true if the passed key is equal to this key
func (pk *CallSiteParamAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*CallSiteParamAnnotationKey); ok {
		return *pk == *other
	}
	return false
}

func (pk *CallSiteParamAnnotationKey) String() string {
	argname := ""
	if pk.ParamName() != nil {
		argname = fmt.Sprintf(": '%s'", pk.ParamName().Name())
	}
	return fmt.Sprintf("Param %d%s of Function %s at Location %s",
		pk.ParamNum, argname, pk.FuncDecl.Name(), pk.Location.String())
}

// MinimalString returns a string representation for this CallSiteParamAnnotationKey consisting
// only of the word "arg" followed by the name of the parameter, if named, or its position
// otherwise.
func (pk *CallSiteParamAnnotationKey) MinimalString() string {
	if pk.ParamName() != nil && len(pk.ParamName().Name()) > 0 {
		return fmt.Sprintf("arg `%s`", pk.ParamName().Name())
	}
	return fmt.Sprintf("arg %d", pk.ParamNum)
}

// ParamNameString returns the name of this parameter, if named, or a placeholder string otherwise.
func (pk *CallSiteParamAnnotationKey) ParamNameString() string {
	if pk.ParamName() != nil {
		return pk.ParamName().Name()
	}
	return fmt.Sprintf("<unnamed param %d>", pk.ParamNum)
}

// NewCallSiteParamKey returns a new instance of CallSiteParamAnnotationKey constructed along with
// validation that its passed argument number is valid for the passed function declaration.
func NewCallSiteParamKey(
	fdecl *types.Func, num int, location token.Position) *CallSiteParamAnnotationKey {
	sig := fdecl.Type().(*types.Signature)
	// for variadic functions - "round down" their argument number to the variadic arg
	if sig.Variadic() && num >= sig.Params().Len()-1 {
		return &CallSiteParamAnnotationKey{
			FuncDecl: fdecl,
			ParamNum: sig.Params().Len() - 1,
			Location: location,
		}
	}

	// for regular functions - panic if arg num too high
	if sig.Params().Len() <= num {
		panic(fmt.Sprintf(
			"no such parameter number %d - out of bounds for function %s with %d parameters",
			sig.Params().Len(), fdecl.Name(), num))
	}
	return &CallSiteParamAnnotationKey{
		FuncDecl: fdecl,
		ParamNum: num,
		Location: location,
	}
}

// ParamAnnotationKey allows the Lookup of a function parameter's Annotation in the Annotation map
// Only construct these using ParamKeyFromArgNum and ParamKeyFromName
type ParamAnnotationKey struct {
	FuncDecl *types.Func
	ParamNum int
}

// ParamName returns the *types.Var naming the parameter associate with this key
// nilable(result 0)
func (pk *ParamAnnotationKey) ParamName() *types.Var {
	return pk.FuncDecl.Type().(*types.Signature).Params().At(pk.ParamNum)
}

// ParamKeyFromArgNum returns a new instance of ParamAnnotationKey constructed along with validation
// that its passed argument number is valid for the passed function declaration
func ParamKeyFromArgNum(fdecl *types.Func, num int) *ParamAnnotationKey {
	sig := fdecl.Type().(*types.Signature)
	// for variadic functions - "round down" their argument number to the variadic arg
	if sig.Variadic() && num >= sig.Params().Len()-1 {
		return &ParamAnnotationKey{
			FuncDecl: fdecl,
			ParamNum: sig.Params().Len() - 1,
		}
	}

	// for regular functions - panic if arg num too high
	if sig.Params().Len() <= num {
		panic(fmt.Sprintf("no such parameter number %d - out of bounds for function %s with %d parameters", sig.Params().Len(), fdecl.Name(), num))
	}
	return &ParamAnnotationKey{
		FuncDecl: fdecl,
		ParamNum: num,
	}
}

// ParamKeyFromName returns a new instance of ParamAnnotationKey constructed from the name of the parameter
func ParamKeyFromName(fdecl *types.Func, paramName *types.Var) *ParamAnnotationKey {
	sig := fdecl.Type().(*types.Signature)

	for i := 0; i < sig.Params().Len(); i++ {
		if sig.Params().At(i) == paramName {
			return &ParamAnnotationKey{
				FuncDecl: fdecl,
				ParamNum: i,
			}
		}
	}
	panic(fmt.Sprintf("no such parameter %s for function %s", paramName.String(), fdecl.String()))
}

// Lookup looks this key up in the passed map, returning a Val
func (pk *ParamAnnotationKey) Lookup(annMap Map) (Val, bool) {

	if paramVal, ok := annMap.CheckFuncParamAnn(pk.FuncDecl, pk.ParamNum); ok {
		return paramVal, true
	}
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (pk *ParamAnnotationKey) Object() types.Object {
	return pk.FuncDecl
}

// equals returns true if the passed key is equal to this key
func (pk *ParamAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*ParamAnnotationKey); ok {
		return *pk == *other
	}
	return false
}

func (pk *ParamAnnotationKey) String() string {
	argname := ""
	if pk.ParamName() != nil {
		argname = fmt.Sprintf(": '%s'", pk.ParamName().Name())
	}
	return fmt.Sprintf("Param %d%s of Function %s",
		pk.ParamNum, argname, pk.FuncDecl.Name())
}

// MinimalString returns a string representation for this ParamAnnotationKey consisting only
// of the word "arg" followed by the name of the parameter, if named, or its position otherwise
func (pk *ParamAnnotationKey) MinimalString() string {
	if pk.ParamName() != nil && len(pk.ParamName().Name()) > 0 {
		return fmt.Sprintf("arg `%s`", pk.ParamName().Name())
	}
	return fmt.Sprintf("arg %d", pk.ParamNum)
}

// ParamNameString returns the name of this parameter, if named, or a placeholder string otherwise
func (pk *ParamAnnotationKey) ParamNameString() string {
	if pk.ParamName() != nil {
		return pk.ParamName().Name()
	}
	return fmt.Sprintf("<unnamed param %d>", pk.ParamNum)
}

// CallSiteRetAnnotationKey is similar to RetAnnotationKey, but it represents the site in the
// caller where the actual result is returned from the function. For the same return result of the
// same function, there is only one distinct RetAnnotationKey but there is a new
// CallSiteRetAnnotationKey for the return result for every call of the same function.
type CallSiteRetAnnotationKey struct {
	FuncDecl *types.Func
	RetNum   int // which result
	Location token.Position
}

// Lookup looks this key up in the passed map, returning a Val.
func (rk *CallSiteRetAnnotationKey) Lookup(annMap Map) (Val, bool) {
	if retVal, ok := annMap.CheckFuncCallSiteRetAnn(rk); ok {
		return retVal, true
	}
	// Revert to the function's RetAnnotationKey look up if there is no call-site annotation.
	if paramVal, ok := annMap.CheckFuncRetAnn(rk.FuncDecl, rk.RetNum); ok {
		return paramVal, true
	}
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating.
func (rk *CallSiteRetAnnotationKey) Object() types.Object {
	return rk.FuncDecl
}

// equals returns true if the passed key is equal to this key
func (rk *CallSiteRetAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*CallSiteRetAnnotationKey); ok {
		return *rk == *other
	}
	return false
}

func (rk *CallSiteRetAnnotationKey) String() string {
	return fmt.Sprintf("Result %d of Function %s at Location %v",
		rk.RetNum, rk.FuncDecl.Name(), rk.Location)
}

// NewCallSiteRetKey returns a new instance of CallSiteRetAnnotationKey constructed from the name
// of the parameter.
func NewCallSiteRetKey(fdecl *types.Func, retNum int, location token.Position) *CallSiteRetAnnotationKey {
	return &CallSiteRetAnnotationKey{
		FuncDecl: fdecl,
		RetNum:   retNum,
		Location: location,
	}
}

// RetAnnotationKey allows the Lookup of a function's return Annotation in the Annotation Map
type RetAnnotationKey struct {
	FuncDecl *types.Func
	RetNum   int // which result
}

// Lookup looks this key up in the passed map, returning a Val
func (rk *RetAnnotationKey) Lookup(annMap Map) (Val, bool) {
	if retVal, ok := annMap.CheckFuncRetAnn(rk.FuncDecl, rk.RetNum); ok {
		return retVal, true
	}
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (rk *RetAnnotationKey) Object() types.Object {
	return rk.FuncDecl
}

// equals returns true if the passed key is equal to this key
func (rk *RetAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*RetAnnotationKey); ok {
		return *rk == *other
	}
	return false
}

func (rk *RetAnnotationKey) String() string {
	return fmt.Sprintf("Result %d of Function %s",
		rk.RetNum, rk.FuncDecl.Name())
}

// RetKeyFromRetNum returns a new instance of RetAnnotationKey constructed from the name of the parameter
func RetKeyFromRetNum(fdecl *types.Func, retNum int) *RetAnnotationKey {
	return &RetAnnotationKey{
		FuncDecl: fdecl,
		RetNum:   retNum,
	}
}

// TypeNameAnnotationKey allows the Lookup of a named type annotations in the Annotation Map
type TypeNameAnnotationKey struct {
	TypeDecl *types.TypeName
}

// Lookup looks this key up in the passed map, returning a Val
func (tk *TypeNameAnnotationKey) Lookup(annMap Map) (Val, bool) {
	if typeVal, ok := annMap.CheckDeepTypeAnn(tk.TypeDecl); ok {
		return typeVal, true
	}
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (tk *TypeNameAnnotationKey) Object() types.Object {
	return tk.TypeDecl
}

// equals returns true if the passed key is equal to this key
func (tk *TypeNameAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*TypeNameAnnotationKey); ok {
		return *tk == *other
	}
	return false
}

func (tk *TypeNameAnnotationKey) String() string {
	return fmt.Sprintf("Type %s", tk.TypeDecl.Name())
}

// GlobalVarAnnotationKey allows the Lookup of a global variable's annotations in the Annotation Map
type GlobalVarAnnotationKey struct {
	VarDecl *types.Var
}

// Lookup looks this key up in the passed map, returning a Val
func (gk *GlobalVarAnnotationKey) Lookup(annMap Map) (Val, bool) {
	if typeVal, ok := annMap.CheckGlobalVarAnn(gk.VarDecl); ok {
		return typeVal, true
	}
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (gk *GlobalVarAnnotationKey) Object() types.Object {
	return gk.VarDecl
}

// equals returns true if the passed key is equal to this key
func (gk *GlobalVarAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*GlobalVarAnnotationKey); ok {
		return *gk == *other
	}
	return false
}

func (gk *GlobalVarAnnotationKey) String() string {
	return fmt.Sprintf("Global Variable %s", gk.VarDecl.Name())
}

// RetFieldAnnotationKey allows the Lookup of the Annotation on a specific field within a function's return of struct
// (or pointer to struct) type, in the Annotation Map. This key is only effective when the struct initialization checking
// is enabled.
//
// TODO: Add support for field of function return with no inference (Currently, only works with inference)
type RetFieldAnnotationKey struct {
	// FuncDecl is the function type of function containing return
	FuncDecl *types.Func
	// RetNum is the index of the return for the key
	RetNum int
	// FieldDecl is the declaration for the field of the key
	FieldDecl *types.Var
}

// Lookup looks this key up in the passed map, returning a Val.
func (rf *RetFieldAnnotationKey) Lookup(_ Map) (Val, bool) {
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (rf *RetFieldAnnotationKey) Object() types.Object {
	return rf.FuncDecl
}

// equals returns true if the passed key is equal to this key
func (rf *RetFieldAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*RetFieldAnnotationKey); ok {
		return *rf == *other
	}
	return false
}

// String returns a string representation of this annotation key
func (rf *RetFieldAnnotationKey) String() string {
	// If the function has a receiver, we add info in the error message
	if rec, ok := rf.FuncDecl.Type().(*types.Signature); ok {
		if rec.Recv() != nil {
			return fmt.Sprintf("Field %s of Result %d of Function %s with receiver %s",
				rf.FieldDecl.Name(), rf.RetNum, rf.FuncDecl.Name(), rec.Recv().Name())
		}
	}

	return fmt.Sprintf("Field %s of Result %d of Function %s",
		rf.FieldDecl.Name(), rf.RetNum, rf.FuncDecl.Name())
}

// EscapeFieldAnnotationKey allows the Lookup of a field's Annotation in the Annotation map
// For fields of depth 1, with struct initialization check, we track the nilability using param field and return field.
// Anything that is not trackable using those, rely on the default nilability of the field.
// Thus, we use the escape information for choosing the nilability of the fields that we do not track.
// The annotation site is only used when the struct initialization check is enabled.
// The trigger that uses this key creates constraints on escaping fields. We create constraints only on the fields
// that have nilable type.
// There are 2 cases, that we currently consider as escaping:
// 1. If a struct is returned from the function where the field has nilable value,
// e.g, If aptr is pointer in struct A, then  `return &A{}` causes the field aptr to escape
// 2. If a struct is parameter of a function and the field is not initialized
// e.g., if we have fun(&A{}) then the field aptr is considered escaped
// TODO: Add struct assignment as another possible cause of field escape
type EscapeFieldAnnotationKey struct {
	FieldDecl *types.Var
}

// Lookup looks this key up in the passed map, returning a Val
// Currently, the annotation key is used only with inference
// TODO: This should be updated on supporting no-infer with struct initialization
func (ek *EscapeFieldAnnotationKey) Lookup(_ Map) (Val, bool) {
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (ek *EscapeFieldAnnotationKey) Object() types.Object {
	return ek.FieldDecl
}

// equals returns true if the passed key is equal to this key
func (ek *EscapeFieldAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*EscapeFieldAnnotationKey); ok {
		return *ek == *other
	}
	return false
}

func (ek *EscapeFieldAnnotationKey) String() string {
	return fmt.Sprintf("escaped Field %s", ek.FieldDecl.Name())
}

// ParamFieldAnnotationKey allows the Lookup of Annotation of a function parameter's fields in the
// Annotation map.
// The key is used for tracking flows through both function params and the receiver. In case, the key is tracking
// nilability flow through receivers ParamNum is set to ReceiverParamIndex
// If the key is tracking flow from caller to callee then IsTrackingSideEffect is false. If the key is tracking flow
// from callee to caller at return of the callee function then IsTrackingSideEffect is true
type ParamFieldAnnotationKey struct {
	// FuncDecl is the function corresponding to the key
	FuncDecl *types.Func
	// ParamNum is the index of the param. It is set to const ReceiverParamIndex -1 if IsReceiver is set to true
	ParamNum int
	// FieldDecl is the declaration of the field
	FieldDecl *types.Var
	// IsTrackingSideEffect is true if the key is used for tracking the param field nilability from callee to caller
	IsTrackingSideEffect bool
}

// ReceiverParamIndex is used as the virtual index of the receiver. Since the struct initialization checking for fields
// of params and fields of receiver uses similar logic, using this virtual index reduces a lot of code repetition.
const ReceiverParamIndex = -1

// IsReceiver returns true if the key is corresponding to a receiver of a method
func (pf *ParamFieldAnnotationKey) IsReceiver() bool {
	return pf.ParamNum == ReceiverParamIndex
}

// ParamName returns the *types.Var naming the parameter associate with this key
// nilable(result 0)
func (pf *ParamFieldAnnotationKey) ParamName() *types.Var {

	if pf.IsReceiver() {
		return pf.FuncDecl.Type().(*types.Signature).Recv()
	}
	return pf.FuncDecl.Type().(*types.Signature).Params().At(pf.ParamNum)
}

// Lookup looks this key up in the passed map, returning a Val
// Currently, the annotation key is used only with inference
// TODO: This should be updated on supporting no-infer with struct initialization
func (pf *ParamFieldAnnotationKey) Lookup(_ Map) (Val, bool) {
	return nonAnnotatedDefault, false
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (pf *ParamFieldAnnotationKey) Object() types.Object {
	return pf.FuncDecl
}

// equals returns true if the passed key is equal to this key
func (pf *ParamFieldAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*ParamFieldAnnotationKey); ok {
		return *pf == *other
	}
	return false
}

// String returns a string representation of this annotation key for ParamFieldAnnotationKey
func (pf *ParamFieldAnnotationKey) String() string {
	argName := ""
	if pf.ParamName() != nil {
		argName = fmt.Sprintf(": '%s'", pf.ParamName().Name())
	}

	paramSite := "at input"
	if pf.IsTrackingSideEffect {
		paramSite = "at output"
	}

	if pf.IsReceiver() {
		return fmt.Sprintf("Field %s of Receiver%s %s of Method %s", pf.FieldDecl.Name(), argName, paramSite, pf.FuncDecl.Name())
	}

	return fmt.Sprintf("Field %s of Param %d%s %s of Function %s", pf.FieldDecl.Name(),
		pf.ParamNum, argName, paramSite, pf.FuncDecl.Name())

}

// RecvAnnotationKey allows the Lookup of a method's receiver Annotation in the Annotation map
type RecvAnnotationKey struct {
	FuncDecl *types.Func
}

// Lookup looks this key up in the passed map, returning a Val
func (rk *RecvAnnotationKey) Lookup(annMap Map) (Val, bool) {
	if retVal, ok := annMap.CheckFuncRecvAnn(rk.FuncDecl); ok {
		return retVal, true
	}
	return nonAnnotatedDefault, false
}

// Package returns the package containing the site of this annotation key
func (rk *RecvAnnotationKey) Package() *types.Package {
	return rk.FuncDecl.Pkg()
}

// Object returns the types.Object that this annotation can best be interpreted as annotating
func (rk *RecvAnnotationKey) Object() types.Object {
	return rk.FuncDecl
}

// Exported returns true iff this annotation is observable by downstream packages
func (rk *RecvAnnotationKey) Exported() bool {
	return rk.FuncDecl.Exported()
}

// equals returns true if the passed key is equal to this key
func (rk *RecvAnnotationKey) equals(other Key) bool {
	if other, ok := other.(*RecvAnnotationKey); ok {
		return *rk == *other
	}
	return false
}

func (rk *RecvAnnotationKey) String() string {
	return fmt.Sprintf("Receiver of Method %s", rk.FuncDecl.Name())
}
