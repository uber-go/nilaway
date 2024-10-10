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
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis"
)

// ErrorType is the type of the builtin "error" interface.
var ErrorType = types.Universe.Lookup("error").Type()

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

// UnwrapPtr unwraps a pointer type and returns the element type. For all other types it returns
// the type unmodified.
func UnwrapPtr(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// FuncIdentFromCallExpr return a function identified from a call expression, nil otherwise
// nilable(result 0)
func FuncIdentFromCallExpr(expr *ast.CallExpr) *ast.Ident {
	switch fun := expr.Fun.(type) {
	case *ast.Ident:
		return fun
	case *ast.SelectorExpr:
		return fun.Sel
	default:
		// case of anonymous function
		return nil
	}
}

// PartiallyQualifiedFuncName returns the name of the passed function, with the name of its receiver
// if defined
func PartiallyQualifiedFuncName(f *types.Func) string {
	if sig, ok := f.Type().(*types.Signature); ok && sig.Recv() != nil {
		return fmt.Sprintf("%s.%s", PortionAfterSep(sig.Recv().Type().String(), ".", 0), f.Name())
	}
	return f.Name()
}

// PortionAfterSep returns the suffix of the passed string `input` containing at most `occ` occurrences
// of the separator `sep`
func PortionAfterSep(input, sep string, occ int) string {
	splits := strings.Split(input, sep)
	n := len(splits)
	if n <= occ+1 {
		return input // input contains at most `occ` occurrences of `sep`
	}
	out := ""
	for i := n - (1 + occ); i < n; i++ {
		if len(out) > 0 {
			out += sep
		}
		out += splits[i]
	}
	return out
}

// ExprIsAuthentic aims to return true iff the passed expression is an AST node
// found in the source program of this pass - not one that we created as an intermediate value.
// There is no fully sound way to do this - but returning whether it is present in the `Types` map
// map is a good approximation.
// Right now, this is used only to decide whether to print the location of the producer expression
// in a full trigger.
func ExprIsAuthentic(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	return t != nil
}

// IsSliceAppendCall checks if `node` represents the builtin append(slice []Type, elems ...Type) []Type
// call on a slice.
// The function checks 2 things,
// 1) Name of the called function is "builtin append"
// 2) The first argument to the function is a slice
func IsSliceAppendCall(node *ast.CallExpr, pass *analysis.Pass) (*types.Slice, bool) {
	if funcName, ok := node.Fun.(*ast.Ident); ok {
		if declObj := pass.TypesInfo.Uses[funcName]; declObj != nil {
			if declObj.String() == "builtin append" {
				if sliceType, ok := pass.TypesInfo.TypeOf(node.Args[0]).(*types.Slice); ok {
					return sliceType, true
				}
			}
		}
	}
	return nil, false
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

// ExprBarsNilness returns if the expression can never be nil for the simple reason that nil does
// not inhabit its type.
func ExprBarsNilness(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	// `pass.TypesInfo.TypeOf` only checks Types, Uses, and Defs maps in TypesInfo. However, we may
	// miss types for some expressions. For example, `f` in `s.f` can only be found in
	// `pass.TypesInfo.Selections` map (see the comments of pass.TypesInfo.Types for more details).
	// Be conservative for those cases for now.
	// TODO:  to investigate and find more cases.
	if t == nil {
		return false
	}
	return TypeBarsNilness(pass.TypesInfo.TypeOf(expr))
}

// FuncNumResults looks at a function declaration and returns the number of results of that function
func FuncNumResults(decl *types.Func) int {
	return decl.Type().(*types.Signature).Results().Len()
}

// IsEmptyExpr checks if an expression is the empty identifier
func IsEmptyExpr(expr ast.Expr) bool {
	if id, ok := expr.(*ast.Ident); ok {
		if id.Name == "_" {
			return true
		}
	}
	return false
}

// funcIsRichCheckEffectReturning encodes the conditions that a function is deemed "rich-check-effect-returning", i.e.,
// it is an error-returning function or a bool(ok)-returning function.
// A function is deemed "rich-check-effect-returning" iff it has a single result of type `typName` (error or bool),
// and that result is the last in the list of results.
func funcIsRichCheckEffectReturning(fdecl *types.Func, expectedType types.Type) bool {
	results := fdecl.Type().(*types.Signature).Results()
	n := results.Len()
	if n == 0 {
		return false
	}
	if !types.Identical(results.At(n-1).Type(), expectedType) {
		return false
	}
	for i := 0; i < n-1; i++ {
		if types.Identical(results.At(i).Type(), expectedType) {
			return false
		}
	}
	return true
}

// FuncIsErrReturning encodes the conditions that a function is deemed "error-returning".
// This guards its results to require an `err` check before use as nonnil.
// A function is deemed "error-returning" iff it has a single result of type `error`, and that
// result is the last in the list of results.
func FuncIsErrReturning(fdecl *types.Func) bool {
	return funcIsRichCheckEffectReturning(fdecl, ErrorType)
}

// FuncIsOkReturning encodes the conditions that a function is deemed "ok-returning".
// This guards its results to require an `ok` check before use as nonnil.
// A function is deemed "ok-returning" iff it has a single result of type `bool`, and that
// result is the last in the list of results.
func FuncIsOkReturning(fdecl *types.Func) bool {
	return funcIsRichCheckEffectReturning(fdecl, BoolType)
}

// IsFieldSelectorChain returns true if the expr is chain of idents. e.g, x.y.z
// It returns for false for expressions such as x.y().z
func IsFieldSelectorChain(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return IsFieldSelectorChain(expr.X)
	default:
		return false
	}
}

// GetFieldVal returns the assigned value for the field at index. compElts holds the  elements of the composite literal expression
// for struct initialization
func GetFieldVal(compElts []ast.Expr, fieldName string, numFields int, index int) ast.Expr {
	for _, elt := range compElts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok {
				if key.Name == fieldName {
					return kv.Value
				}
			}
		}
	}

	// In this case the initialization is serial e.g. a = &A{p, q}
	if numFields == len(compElts) {
		return compElts[index]
	}
	return nil
}

// GetFunctionParamNode returns the ast param node matching the variable searchParam
func GetFunctionParamNode(funcDecl *ast.FuncDecl, searchParam *types.Var) ast.Expr {
	for _, params := range funcDecl.Type.Params.List {
		for _, param := range params.Names {
			if searchParam.Name() == param.Name && param.Name != "" && param.Name != "_" {
				return param
			}
		}
	}

	return nil
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

// GetSelectorExprHeadIdent gets the head of the chained selector expression if it is an ident. Returns nil otherwise
func GetSelectorExprHeadIdent(selExpr *ast.SelectorExpr) *ast.Ident {
	if ident, ok := selExpr.X.(*ast.Ident); ok {
		return ident
	}
	if x, ok := selExpr.X.(*ast.SelectorExpr); ok {
		return GetSelectorExprHeadIdent(x)
	}
	return nil
}

// IsLiteral returns true if `expr` is a literal that matches with one of the given literal values (e.g., "nil", "true", "false)
func IsLiteral(expr ast.Expr, literals ...string) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		for _, literal := range literals {
			if ident.Name == literal {
				return true
			}
		}
	}
	return false
}

// TruncatePosition truncates the prefix of the filename to keep it at the given depth (config.DirLevelsToPrintForTriggers)
func TruncatePosition(position token.Position) token.Position {
	position.Filename = PortionAfterSep(
		position.Filename, "/",
		config.DirLevelsToPrintForTriggers)
	return position
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

// truncatePosition removes part of prefix of the full file path, determined by
// config.DirLevelsToPrintForTriggers.
func truncatePosition(position token.Position) token.Position {
	position.Filename = PortionAfterSep(
		position.Filename, "/",
		config.DirLevelsToPrintForTriggers)
	return position
}

// PosToLocation converts a token.Pos as a real code location, of token.Position.
func PosToLocation(pos token.Pos, pass *analysis.Pass) token.Position {
	return truncatePosition(pass.Fset.Position(pos))
}
