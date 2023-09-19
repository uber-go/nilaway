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
	"go/types"

	"go.uber.org/nilaway/util"
)

// DeepNilabilityAsNamedType tries to interpret the named type as a typedef of a map or slice,
// returning the deep nilability annotation of that typedef if found. Otherwise, it returns
// ProduceTriggerNever to indicate that we assume in the default case the type is NOT deeply nilable
func DeepNilabilityAsNamedType(typ types.Type) ProducingAnnotationTrigger {
	t, ok := typ.(*types.Named)
	if !ok {
		return &ProduceTriggerNever{}
	}

	nameAsDeepTrigger := func(name *types.TypeName) *TriggerIfDeepNilable {
		return &TriggerIfDeepNilable{Ann: &TypeNameAnnotationKey{TypeDecl: name}}
	}

	// Calling Underlying on [types.Named] will always return the unnamed type, so we
	// do not have to recursively "unwrap" the [types.Named].
	// See [https://github.com/golang/example/tree/master/gotypes#named-types].
	switch t.Underlying().(type) {
	case *types.Slice:
		return &SliceRead{TriggerIfDeepNilable: nameAsDeepTrigger(t.Obj())}
	case *types.Array:
		return &ArrayRead{TriggerIfDeepNilable: nameAsDeepTrigger(t.Obj())}
	case *types.Map:
		trigger := nameAsDeepTrigger(t.Obj())
		trigger.NeedsGuard = true
		return &MapRead{
			TriggerIfDeepNilable: trigger,
		}
	case *types.Pointer:
		return &PtrRead{TriggerIfDeepNilable: nameAsDeepTrigger(t.Obj())}
	case *types.Chan:
		return &ChanRecv{TriggerIfDeepNilable: nameAsDeepTrigger(t.Obj())}
	}

	return &ProduceTriggerNever{}
}

// DeepNilabilityOfFuncRet inspects a function return for deep nilability annotation
func DeepNilabilityOfFuncRet(fn *types.Func, retNum int) ProducingAnnotationTrigger {
	fsig := fn.Type().(*types.Signature)
	retType := fsig.Results().At(retNum).Type()
	if util.TypeIsDeep(retType) {
		return &FuncReturnDeep{
			TriggerIfDeepNilable: &TriggerIfDeepNilable{
				Ann:        RetKeyFromRetNum(fn, retNum),
				NeedsGuard: util.TypeIsDeeplyMap(retType)},
		}
	}
	return DeepNilabilityAsNamedType(retType)
}

// DeepNilabilityOfFld inspects a struct field for deep nilability annotation
func DeepNilabilityOfFld(fld *types.Var) ProducingAnnotationTrigger {
	if util.TypeIsDeep(fld.Type()) {
		// in this case, the deep nilability of the field comes from its declaring annotations
		return &FldReadDeep{
			TriggerIfDeepNilable: &TriggerIfDeepNilable{
				Ann: &FieldAnnotationKey{
					FieldDecl: fld},
				NeedsGuard: util.TypeIsDeeplyMap(fld.Type())},
		}
	}
	return DeepNilabilityAsNamedType(fld.Type())
}

// DeepNilabilityOfVar inspects a variable for deep nilability annotation
func DeepNilabilityOfVar(fdecl *types.Func, v *types.Var) ProducingAnnotationTrigger {
	if util.TypeIsDeep(v.Type()) {
		// in each of the following cases, the deep nilability of the variable comes from its
		// declaring annotations
		if VarIsGlobal(v) {
			return &GlobalVarReadDeep{
				TriggerIfDeepNilable: &TriggerIfDeepNilable{
					Ann: &GlobalVarAnnotationKey{
						VarDecl: v,
					},
					NeedsGuard: util.TypeIsDeeplyMap(v.Type())},
			}
		}
		if VarIsParam(fdecl, v) {
			return paramAsDeepProducer(fdecl, v)
		}
		if VarIsRecv(fdecl, v) {
			return &MethodRecvDeep{
				TriggerIfDeepNilable: &TriggerIfDeepNilable{
					Ann: &RecvAnnotationKey{FuncDecl: fdecl}},
				VarDecl: v,
			}
		}
		// in this case, the deep nilability of the variable is dependent only on its possible guarding
		return &LocalVarReadDeep{
			ReadVar:             v,
			ProduceTriggerNever: &ProduceTriggerNever{NeedsGuard: util.TypeIsDeeplyMap(v.Type())}}
	}
	// otherwise, the deep nilability of this variable is either that of its named type,
	// or not deeply nilable - a logical split captured in the method DeepNilabilityAsNamedType
	return DeepNilabilityAsNamedType(v.Type())
}

// VarIsGlobal returns true iff `v` is a global variable
// this check is performed by looking up the package of the variable,
// then the declaring scope of that package,
// then checking that declaring scope to see if it maps the name of `v` to
// the passed `*types.Var` instance of `v`
func VarIsGlobal(v *types.Var) bool {
	return v.Pkg().Scope().Lookup(v.Name()) == v
}

// VarIsParam returns true iff `v` is a parameter of `fdecl`
func VarIsParam(fdecl *types.Func, v *types.Var) bool {
	fsig := fdecl.Type().(*types.Signature)

	// could avoid this iteration by hashing once, but param lists are short enough that
	// it feels like a premature optimization
	for i := 0; i < fsig.Params().Len(); i++ {
		if fsig.Params().At(i) == v {
			return true
		}
	}

	return false
}

// VarIsVariadicParam returns true iff `v` is a variadic parameter of `fdecl`
func VarIsVariadicParam(fdecl *types.Func, v *types.Var) bool {
	fsig := fdecl.Type().(*types.Signature)

	return fsig.Params().Len() > 0 &&
		fsig.Params().At(fsig.Params().Len()-1) == v &&
		fsig.Variadic()
}

// VarIsRecv returns true iff `v` is the receiver of `fdecl`
func VarIsRecv(fdecl *types.Func, v *types.Var) bool {
	fsig := fdecl.Type().(*types.Signature)

	return fsig.Recv() == v
}

// ParamAsProducer inspects a variable, which must be a parameter to the passed function, and returns
// a produce trigger for its value as annotated at its function's declaration. The interesting case
// is when the parameter is variadic - and then the annotation is interpreted as referring to the
// elements of the variadic slice not the slice itself
func ParamAsProducer(fdecl *types.Func, param *types.Var) ProducingAnnotationTrigger {
	if !VarIsParam(fdecl, param) {
		panic(fmt.Sprintf("non-param %s passed to ParamAsProducer", param.Name()))
	}
	if VarIsVariadicParam(fdecl, param) {
		return &VariadicFuncParam{ProduceTriggerTautology: &ProduceTriggerTautology{}, VarDecl: param}
	}
	return &FuncParam{
		TriggerIfNilable: &TriggerIfNilable{
			Ann: ParamKeyFromName(fdecl, param)}}
}

// paramAsDeepProducer inspects a variable, which must be a parameter to the passed function, and returns
// a produce trigger for its deep value as annotated at its function's declaration.
//
// As above, the interesting case is when the parameter is variadic
func paramAsDeepProducer(fdecl *types.Func, param *types.Var) ProducingAnnotationTrigger {
	if !VarIsParam(fdecl, param) {
		panic(fmt.Sprintf("non-param %s passed to ParamAsProducer", param.Name()))
	}

	paramKey := ParamKeyFromName(fdecl, param)

	if VarIsVariadicParam(fdecl, param) {
		return &VariadicFuncParamDeep{
			TriggerIfNilable: &TriggerIfNilable{
				Ann:        paramKey,
				NeedsGuard: util.TypeIsDeeplyMap(param.Type())}}
	}
	return &FuncParamDeep{
		TriggerIfDeepNilable: &TriggerIfDeepNilable{
			Ann:        paramKey,
			NeedsGuard: util.TypeIsDeeplyMap(param.Type())},
	}
}

// NewRetFldAnnKey returns the RetFieldAnnotationKey for return at retNum and the field fieldDecl.
// This function is called from multiple places where funcDecl could be the declaration of the function being analyzed
// (when looking at a return statement) or a function called from the function being analyzed
// (when looking at a function call statement, in which case funcDecl is the callee).
func NewRetFldAnnKey(funcDecl *types.Func, retNum int, fieldDecl *types.Var) *RetFieldAnnotationKey {
	return &RetFieldAnnotationKey{
		FuncDecl:  funcDecl,
		RetNum:    retNum,
		FieldDecl: fieldDecl,
	}
}

// NewParamFldAnnKey returns ParamFieldAnnotationKey for a field fieldDecl of param at index of the function funcObj
func NewParamFldAnnKey(funcObj *types.Func, index int, fieldDecl *types.Var) *ParamFieldAnnotationKey {
	return &ParamFieldAnnotationKey{
		FuncDecl:  funcObj,
		ParamNum:  index,
		FieldDecl: fieldDecl,
	}
}

// NewEscapeFldAnnKey returns a new EscapeFieldAnnotationKey for field fieldObj
func NewEscapeFldAnnKey(fieldObj *types.Var) *EscapeFieldAnnotationKey {
	return &EscapeFieldAnnotationKey{
		FieldDecl: fieldObj,
	}
}
