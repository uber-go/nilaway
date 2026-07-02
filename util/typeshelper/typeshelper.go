//  Copyright (c) 2025 Uber Technologies, Inc.
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

// Package typeshelper implements utility functions for the go/types package.
package typeshelper

import (
	"fmt"
	"go/types"

	"go.uber.org/nilaway/util/tokenhelper"
	"golang.org/x/exp/typeparams"
)

// ErrorType is the type of the builtin "error" interface.
var ErrorType = types.Universe.Lookup("error").Type()

// ErrorInterface is the underlying type of the builtin "error" interface.
var ErrorInterface = ErrorType.Underlying().(*types.Interface)

// BoolType is the type of the builtin "bool" interface.
var BoolType = types.Universe.Lookup("bool").Type()

// BuiltinLen is the builtin "len" function object.
var BuiltinLen = types.Universe.Lookup("len")

// BuiltinMin is the builtin "min" function object.
var BuiltinMin = types.Universe.Lookup("min")

// BuiltinMax is the builtin "max" function object.
var BuiltinMax = types.Universe.Lookup("max")

// BuiltinAppend is the builtin "append" function object.
var BuiltinAppend = types.Universe.Lookup("append")

// BuiltinNew is the builtin "new" function object.
var BuiltinNew = types.Universe.Lookup("new")

// IsDeep checks if a type is an expression that admits deep nilability, such as maps, slices, arrays, etc.
// Only consider pointers to deep types (e.g., `var x *[]int`) as deep type,
// not pointers to basic types (e.g., `var x *int`) or struct types (e.g., `var x *S`)
func IsDeep(t types.Type) bool {
	switch UnwrapPtr(t).(type) {
	case *types.Slice, *types.Array, *types.Map, *types.Chan, *types.Struct:
		return true
	case *types.Basic:
		return false
	}
	if t, ok := t.(*types.Pointer); ok {
		if AsDeeplyStruct(t.Underlying()) == nil {
			return true
		}
	}

	return false
}

// IsDeeplyType returns true if the underlying type of `t` is a T (e.g., *types.Array), resolving
// named types and aliases, as well as type parameters whose type sets contain only such types
// (see underlyingAlwaysSatisfies for the exact type parameter handling).
func IsDeeplyType[T types.Type](t types.Type) bool {
	return underlyingAlwaysSatisfies(t, func(u types.Type) bool {
		_, ok := u.(T)
		return ok
	})
}

// IsDeeplyArrayOrArrayPtr is like IsDeeplyType[*types.Array], but additionally accepts pointers to arrays
// (again resolving named types, aliases, and type parameters). Slice expressions and range
// statements auto-dereference pointers to arrays, so for them an operand of either type
// behaves like an array.
func IsDeeplyArrayOrArrayPtr(t types.Type) bool {
	return underlyingAlwaysSatisfies(t, func(u types.Type) bool {
		if ptr, ok := u.(*types.Pointer); ok {
			u = ptr.Elem().Underlying()
		}
		_, ok := u.(*types.Array)
		return ok
	})
}

// underlyingAlwaysSatisfies reports whether the underlying type of `t` satisfies pred. Named
// types and aliases are resolved via Underlying(). For type parameters, the underlying type of
// every term in the constraint's normalized type set must satisfy pred. This is conservative:
// type parameters with no structural restriction (e.g. `any`, or method-only constraints), an
// empty type set, or a type set too complex to normalize all yield false.
func underlyingAlwaysSatisfies(t types.Type, pred func(types.Type) bool) bool {
	if t == nil {
		return false
	}
	if tp, ok := types.Unalias(t).(*types.TypeParam); ok {
		// NormalTerms returns nil (no error) for an unconstrained type set, ErrEmptyTypeSet for
		// an empty one, and an error for constraints that are invalid or exceed complexity
		// bounds; in every such case we conservatively return false.
		terms, err := typeparams.NormalTerms(tp)
		if err != nil || len(terms) == 0 {
			return false
		}
		for _, term := range terms {
			if !pred(term.Type().Underlying()) {
				return false
			}
		}
		return true
	}
	return pred(t.Underlying())
}

// AsDeeplyStruct returns the underlying struct type if `typ` is a struct or a pointer to a
// named struct (resolving named types and aliases). Returns nil otherwise.
// Note: pointer-to-anonymous-struct is intentionally excluded — the struct-init analyzer does
// not yet handle anonymous struct initialization.
func AsDeeplyStruct(typ types.Type) *types.Struct {
	if s, ok := typ.Underlying().(*types.Struct); ok {
		return s
	}
	if ptr, ok := types.Unalias(typ).(*types.Pointer); ok {
		if named, ok := types.Unalias(ptr.Elem()).(*types.Named); ok {
			if s, ok := named.Underlying().(*types.Struct); ok {
				return s
			}
		}
	}
	return nil
}

// IsPointer checks whether the type `t` is an explicit or implicit pointer type, which could also be of deep type.
// Examples of explicit pointer types are `*int`, `*S`, etc.
// Examples of implicit pointer types are `[]int`, `map[string]*S`, `chan int`, etc.
func IsPointer(t types.Type) bool {
	return IsDeeplyType[*types.Pointer](t) ||
		IsDeeplyType[*types.Slice](t) ||
		IsDeeplyType[*types.Map](t) ||
		IsDeeplyType[*types.Array](t) ||
		IsDeeplyType[*types.Chan](t)
}

// UnwrapPtr unwraps a pointer type and returns the element type. For all other types it returns
// the type unmodified.
func UnwrapPtr(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// PartiallyQualifiedFuncName returns the name of the passed function, with the name of its receiver
// if defined
func PartiallyQualifiedFuncName(f *types.Func) string {
	if sig, ok := f.Type().(*types.Signature); ok && sig.Recv() != nil {
		return fmt.Sprintf("%s.%s", tokenhelper.PortionAfterSep(sig.Recv().Type().String(), ".", 0), f.Name())
	}
	return f.Name()
}

// FuncNumResults looks at a function declaration and returns the number of results of that function
func FuncNumResults(decl *types.Func) int {
	return decl.Type().(*types.Signature).Results().Len()
}

// ImplementsError checks if the given object implements the error interface. It also covers the case of
// interfaces that embed the error interface.
func ImplementsError(t types.Type) bool {
	if t == nil {
		return false
	}
	return types.Implements(t, ErrorInterface)
}

// FuncIsErrReturning encodes the conditions that a function is deemed "error-returning".
// This guards its results to require an `err` check before use as nonnil.
// A function is deemed "error-returning" iff it has a single result of type `error`, and that
// result is the last in the list of results.
func FuncIsErrReturning(sig *types.Signature) bool {
	if sig == nil {
		return false
	}

	results := sig.Results()
	n := results.Len()
	if n == 0 {
		return false
	}

	errRes := results.At(n - 1)
	if !ImplementsError(errRes.Type()) {
		return false
	}

	for i := 0; i < n-1; i++ {
		if ImplementsError(results.At(i).Type()) {
			return false
		}
	}
	return true
}

// FuncIsOkReturning encodes the conditions that a function is deemed "ok-returning".
// This guards its results to require an `ok` check before use as nonnil.
// A function is deemed "ok-returning" iff it has a single result of type `bool`, and that
// result is the last in the list of results.
func FuncIsOkReturning(sig *types.Signature) bool {
	results := sig.Results()
	n := results.Len()
	if n == 0 {
		return false
	}
	if !types.Identical(results.At(n-1).Type(), BoolType) {
		return false
	}
	for i := 0; i < n-1; i++ {
		if types.Identical(results.At(i).Type(), BoolType) {
			return false
		}
	}
	return true
}

// GetParamObjFromIndex get the variable corresponding to the parameter from the function functionType
func GetParamObjFromIndex(functionType *types.Func, argIdx int) *types.Var {
	fSig := functionType.Type().(*types.Signature)

	functionParams := fSig.Params()
	if argIdx < functionParams.Len() {
		return functionParams.At(argIdx)
	}
	// In this case the argument is given to a variadic function and the object is last element of the param signature
	if !fSig.Variadic() {
		panic("Function is expected to be variadic in the case when argument index >= length of params")
	}
	return functionParams.At(functionParams.Len() - 1)
}

// IsIterType returns true if the underlying type is an iterator func:
//
// func(func() bool)
// func(func(K) bool)
// func(func(K, V) bool)
//
// See more at https://tip.golang.org/doc/go1.23.
func IsIterType(t types.Type) bool {
	// Ensure it is a function signature.
	sig, ok := t.Underlying().(*types.Signature)
	if !ok {
		return false
	}

	// Ensure it has exactly one parameter (the yield func).
	params := sig.Params()
	if params.Len() != 1 {
		return false
	}

	// Ensure the single parameter is a function type (the yield func).
	paramType, ok := params.At(0).Type().Underlying().(*types.Signature)
	if !ok {
		return false
	}

	// Ensure the yield func takes fewer than 2 arguments and returns exactly one boolean value.
	res := paramType.Results()
	if paramType.Params().Len() > 2 || res.Len() != 1 {
		return false
	}

	// Final check: ensure the return type of the yield func is a boolean.
	basic, ok := res.At(0).Type().Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Bool
}

// GetFuncSignature returns the signature of a function or an anonymous function.
func GetFuncSignature(t types.Type) *types.Signature {
	var sig *types.Signature
	switch t2 := t.(type) {
	case *types.Signature:
		sig = t2
	case *types.Alias:
		// If the alias is a named function pointer, we extract its signature.
		// Example: `type MyFunc func() (*int, error)`
		if s, ok := t2.Underlying().(*types.Signature); ok {
			sig = s
		}
	}
	return sig
}

// TypeBarsNilness returns false iff the type `t` is inhabited by nil.
func TypeBarsNilness(t types.Type) bool {
	switch t := t.(type) {
	case *types.Array:
		return true
	case *types.Slice:
		return false
	case *types.Pointer:
		return false
	case *types.Tuple:
		return false
	case *types.Signature:
		return true // function-types are not inhabited by nil
	case *types.Map:
		return false
	case *types.Chan:
		return false
	case *types.Alias, *types.Named:
		return TypeBarsNilness(t.Underlying())
	case *types.Interface:
		return false
	case *types.Basic:
		// all basic types except UntypedNil are not inhabited by nil
		return t.Kind() != types.UntypedNil
	default:
		return true
	}
}
