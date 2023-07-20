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

package util

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis"
)

// ErrorType is the type of the builtin "error" interface.
var ErrorType = types.Universe.Lookup("error").Type()

// BuiltinLen is the builtin "len" function object.
var BuiltinLen = types.Universe.Lookup("len")

// TypeIsDeep checks if a type is an expression that directly admits a deep nilability annotation - deep
// nilability annotations on all other types are ignored
func TypeIsDeep(t types.Type) bool {
	_, isDeep := TypeAsDeepType(t)
	return isDeep
}

// TypeAsDeepType checks if a type is an expression that directly admits a deep nilability annotation,
// returning true as its boolean param if so, along with the element type as its `types.Type` param
// nilable(result 0)
func TypeAsDeepType(t types.Type) (types.Type, bool) {
	switch t := t.(type) {
	case *types.Slice:
		return t.Elem(), true
	case *types.Array:
		return t.Elem(), true
	case *types.Map:
		return t.Elem(), true
	case *types.Chan:
		return t.Elem(), true
	case *types.Pointer:
		return t.Elem(), true
	}
	return nil, false
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
	if _, ok := t.(*types.Array); ok {
		return true
	}
	if t, ok := t.(*types.Named); ok {
		return TypeIsDeeplyArray(t.Underlying())
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
		if namedType, ok := ptType.Elem().(*types.Named); ok {
			if resType, ok := namedType.Underlying().(*types.Struct); ok {
				return resType
			}
		}
	}
	return nil
}

// UnwrapPtr unwraps a pointer type and returns the element type. For all other types it returns
// the type unmodified.
func UnwrapPtr(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

// TypeOf returns the type of the passed AST expression
func TypeOf(pass *analysis.Pass, expr ast.Expr) types.Type {
	return pass.TypesInfo.TypeOf(expr)
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
	_, ok := pass.TypesInfo.Types[expr]
	return ok
}

// StripParens takes an ast node and strips it of any outmost parentheses
func StripParens(expr ast.Node) ast.Node {
	if parenExpr, ok := expr.(*ast.ParenExpr); ok {
		return StripParens(parenExpr.X)
	}
	return expr
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
				if sliceType, ok := TypeOf(pass, node.Args[0]).(*types.Slice); ok {
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
	case *types.Named:
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

// TypeIsErrorType checks if the type is an error type
func TypeIsErrorType(typ types.Type) bool {
	if typ, ok := typ.(*types.Named); ok {
		return typ.String() == "error"
	}
	return false
}

// FuncIsErrReturning encodes the conditions that a function is deemed "error-returning"
// this guards its results to require an `err` check before use as nonnil.
// a function is deemed "error-returning" iff it has a single result of type `error`, and that
// result is the last in the list of results.
func FuncIsErrReturning(fdecl *types.Func) bool {
	results := fdecl.Type().(*types.Signature).Results()
	n := results.Len()
	if n == 0 {
		return false
	}
	if !TypeIsErrorType(results.At(n - 1).Type()) {
		return false
	}
	for i := 0; i < n-1; i++ {
		if TypeIsErrorType(results.At(i).Type()) {
			return false
		}
	}
	return true
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

// DocContainsIgnore is used by analyzers to check if the file should be ignored by the analyzer.
func DocContainsIgnore(group *ast.CommentGroup) bool {
	return docContainsString(config.NilAwayIgnoreString)(group)
}

// DocContainsStructInitCheck is used by analyzers to check if the struct initialization check enabling string is present
// in the comments.
func DocContainsStructInitCheck(group *ast.CommentGroup) config.StructInitCheckType {
	if docContainsString(config.NilAwayStructInitCheckString)(group) {
		return config.DepthOneFieldCheck
	}
	return config.NoCheck
}

// DocContainsAnonymousFuncCheck is used by analyzers to check if the anonymous function check enabling string is present
// in the comments.
func DocContainsAnonymousFuncCheck(group *ast.CommentGroup) bool {
	return docContainsString(config.NilAwayAnonymousFuncCheckString)(group)
}

// docContainsString is used to check if the file comments contain a string s.
func docContainsString(s string) func(*ast.CommentGroup) bool {
	return func(group *ast.CommentGroup) bool {
		if group != nil {
			for _, comment := range group.List {
				if strings.Contains(comment.Text, s) {
					return true
				}
			}
		}
		return false
	}
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

var codeReferencePattern = regexp.MustCompile("\\`(.*?)\\`")
var pathPattern = regexp.MustCompile(`"(.*?)"`)
var nilabilityPattern = regexp.MustCompile(`([\(|^\t](?i)(definitely\s|must\sbe\s)(nilable|nonnil)[\)]?)`)

// PrettyPrintErrorMessage is used in error reporting to post process and pretty print the output with colors
func PrettyPrintErrorMessage(msg string) string {
	// TODO: below string parsing should not be required after  is implemented
	errorStr := fmt.Sprintf("\x1b[%dm%s\x1b[0m", 31, "error:")       // red
	codeStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 95, "`${1}`")    // magenta
	pathStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 36, "${1}")      // cyan
	nilabilityStr := fmt.Sprintf("\u001B[%dm%s\u001B[0m", 1, "${1}") // bold

	msg = nilabilityPattern.ReplaceAllString(msg, nilabilityStr)
	msg = codeReferencePattern.ReplaceAllString(msg, codeStr)
	msg = pathPattern.ReplaceAllString(msg, pathStr)
	msg = errorStr + msg
	return msg
}
