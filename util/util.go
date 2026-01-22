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

// Package util implements utility functions for AST and types.
package util

import (
	"fmt"
	"go/types"
	"regexp"

	"go.uber.org/nilaway/util/tokenhelper"
)

// ErrorType is the type of the builtin "error" interface.
var ErrorType = types.Universe.Lookup("error").Type()

// ErrorInterface is the underlying type of the builtin "error" interface.
var ErrorInterface = ErrorType.Underlying().(*types.Interface)

// BoolType is the type of the builtin "bool" interface.
var BoolType = types.Universe.Lookup("bool").Type()

// BuiltinLen is the builtin "len" function object.
var BuiltinLen = types.Universe.Lookup("len")

// BuiltinAppend is the builtin "append" function object.
var BuiltinAppend = types.Universe.Lookup("append")

// BuiltinNew is the builtin "new" function object.
var BuiltinNew = types.Universe.Lookup("new")

// TypeIsDeep checks if a type is an expression that admits deep nilability, such as maps, slices, arrays, etc.
// Only consider pointers to deep types (e.g., `var x *[]int`) as deep type,
// not pointers to basic types (e.g., `var x *int`) or struct types (e.g., `var x *S`)
func TypeIsDeep(t types.Type) bool {
	switch UnwrapPtr(t).(type) {
	case *types.Slice, *types.Array, *types.Map, *types.Chan, *types.Struct:
		return true
	case *types.Basic:
		return false
	}
	if t, ok := t.(*types.Pointer); ok {
		if TypeAsDeeplyStruct(t.Underlying()) == nil {
			return true
		}
	}

	return false
}

// TypeIsSlice returns true if `t` is of slice type
func TypeIsSlice(t types.Type) bool {
	switch t.(type) {
	case *types.Slice:
		return true
	default:
		return false
	}
}

// TypeIsDeeplyArray returns true if `t` is of array type, including
// transitively through Named types
func TypeIsDeeplyArray(t types.Type) bool {
	switch tt := UnwrapPtr(t).(type) {
	case *types.Array:
		return true
	case *types.Named:
		return TypeIsDeeplyArray(tt.Underlying())
	}
	return false
}

// TypeIsDeeplySlice returns true if `t` is of slice type, including
// transitively through Named types
func TypeIsDeeplySlice(t types.Type) bool {
	if TypeIsSlice(t) {
		return true
	}
	if t, ok := t.(*types.Named); ok {
		return TypeIsDeeplySlice(t.Underlying())
	}
	return false
}

// TypeIsDeeplyMap returns true if `t` is of map type, including
// transitively through Named types
func TypeIsDeeplyMap(t types.Type) bool {
	if _, ok := t.(*types.Map); ok {
		return true
	}
	if t, ok := t.(*types.Named); ok {
		return TypeIsDeeplyMap(t.Underlying())
	}
	return false
}

// TypeIsDeeplyPtr returns true if `t` is of pointer type, including
// transitively through Named types
func TypeIsDeeplyPtr(t types.Type) bool {
	if _, ok := t.(*types.Pointer); ok {
		return true
	}
	if t, ok := t.(*types.Named); ok {
		return TypeIsDeeplyPtr(t.Underlying())
	}
	return false
}

// TypeIsDeeplyChan returns true if `t` is of channel type, including
// transitively through Named types
func TypeIsDeeplyChan(t types.Type) bool {
	if _, ok := t.(*types.Chan); ok {
		return true
	}
	if t, ok := t.(*types.Named); ok {
		return TypeIsDeeplyChan(t.Underlying())
	}
	return false
}

// TypeAsDeeplyStruct returns underlying struct type if the type is struct type or a pointer to a struct type
// returns nil otherwise
func TypeAsDeeplyStruct(typ types.Type) *types.Struct {
	if typ, ok := typ.(*types.Struct); ok {
		return typ
	}

	if typ, ok := typ.(*types.Named); ok {
		if resType, ok := typ.Underlying().(*types.Struct); ok {
			return resType
		}
	}

	if ptType, ok := typ.(*types.Pointer); ok {
		if namedType, ok := types.Unalias(ptType.Elem()).(*types.Named); ok {
			if resType, ok := namedType.Underlying().(*types.Struct); ok {
				return resType
			}
		}
	}
	return nil
}

// TypeIsDeeplyInterface returns true if `t` is of struct type, including
// transitively through Named types
func TypeIsDeeplyInterface(t types.Type) bool {
	if _, ok := t.(*types.Interface); ok {
		return true
	}
	if t, ok := t.(*types.Named); ok {
		return TypeIsDeeplyInterface(t.Underlying())
	}
	return false
}

// TypeIsPointer checks whether the type `t` is an explicit or implicit pointer type, which could also be of deep type.
// Examples of explicit pointer types are `*int`, `*S`, etc.
// Examples of implicit pointer types are `[]int`, `map[string]*S`, `chan int`, etc.
func TypeIsPointer(t types.Type) bool {
	return TypeIsDeeplyPtr(t) ||
		TypeIsDeeplySlice(t) ||
		TypeIsDeeplyMap(t) ||
		TypeIsDeeplyArray(t) ||
		TypeIsDeeplyChan(t)
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

var codeReferencePattern = regexp.MustCompile("\\`(.*?)\\`")
var pathPattern = regexp.MustCompile(`"(.*?)"`)
var nilabilityPattern = regexp.MustCompile(`([\(|^\t](?i)(found\s|must\sbe\s)(nilable|nonnil)[\)]?)`)

// PrettyPrintErrorMessage is used in error reporting to post process and pretty print the output with colors
func PrettyPrintErrorMessage(msg string) string {
	// TODO: below string parsing should not be required after  is implemented
	errorStr := fmt.Sprintf("\x1b[%dm%s\x1b[0m", 31, "error: ")      // red
	codeStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 95, "`${1}`")    // magenta
	pathStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 36, "${1}")      // cyan
	nilabilityStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 1, "${1}") // bold

	msg = nilabilityPattern.ReplaceAllString(msg, nilabilityStr)
	msg = codeReferencePattern.ReplaceAllString(msg, codeStr)
	msg = pathPattern.ReplaceAllString(msg, pathStr)
	msg = errorStr + msg
	return msg
}
