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
	"regexp"
	"strings"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// Map is an abstraction that concrete annotation maps must implement to be checked against.
type Map interface {
	CheckFieldAnn(*types.Var) (Val, bool)
	CheckFuncParamAnn(*types.Func, int) (Val, bool)
	CheckFuncRetAnn(*types.Func, int) (Val, bool)
	CheckFuncRecvAnn(*types.Func) (Val, bool)
	CheckDeepTypeAnn(*types.TypeName) (Val, bool)
	CheckGlobalVarAnn(*types.Var) (Val, bool)
	CheckFuncCallSiteParamAnn(*CallSiteParamAnnotationKey) (Val, bool)
	CheckFuncCallSiteRetAnn(*CallSiteRetAnnotationKey) (Val, bool)
}

// Val is a possible value of an Annotation
type Val struct {
	IsNilable        bool
	IsDeepNilable    bool
	IsNilableSet     bool
	IsDeepNilableSet bool
}

// EmptyVal indicates an annotation value that is fully nonnil but not "set"
var EmptyVal = Val{
	IsNilable:        false,
	IsDeepNilable:    false,
	IsNilableSet:     false,
	IsDeepNilableSet: false,
}

// makeNilable inspects a Val to see if its nilability has already been set.
// If it has, then makeNilable is a noop, otherwise, it returns a copy of the passed
// Val with the nilability set to true.
// The parameter isFinalVal indicates whether later calls to these utility functions
// can update the `Val` further - uses cases are commented on to discuss why
// this is or is not desirable in given cases.
func (a Val) makeNilable(isFinalVal bool) Val {
	if a.IsNilableSet {
		return a
	}
	return Val{
		IsNilable:        true,
		IsDeepNilable:    a.IsDeepNilable,
		IsNilableSet:     isFinalVal,
		IsDeepNilableSet: a.IsDeepNilableSet,
	}
}

// makeDeepNilable inspects a Val to see if its deep nilability has already been set.
// If it has, then makeDeepNilable is a noop, otherwise, it returns a copy of the passed
// Val with the deep nilability set to true.
// The parameter isFinalVal indicates whether later calls to these utility functions
// can update the `Val` further - uses cases are commented on to discuss why
// this is or is not desirable in given cases.
func (a Val) makeDeepNilable(isFinalVal bool) Val {
	if a.IsDeepNilableSet {
		return a
	}
	return Val{
		IsNilable:        a.IsNilable,
		IsDeepNilable:    true,
		IsNilableSet:     a.IsNilableSet,
		IsDeepNilableSet: isFinalVal,
	}
}

// makeNonNil inspects a Val to see if its nilability has already been set.
// If it has, then makeNonNil is a noop, otherwise, it returns a copy of the passed
// Val with the nilability set to false.
// The parameter isFinalVal indicates whether later calls to these utility functions
// can update the `Val` further - uses cases are commented on to discuss why
// this is or is not desirable in given cases.
func (a Val) makeNonNil(isFinalVal bool) Val {
	if a.IsNilableSet {
		return a
	}
	return Val{
		IsNilable:        false,
		IsDeepNilable:    a.IsDeepNilable,
		IsNilableSet:     isFinalVal,
		IsDeepNilableSet: a.IsDeepNilableSet,
	}
}

// makeDeepNonNil inspects a Val to see if its deep nilability has already been set.
// If it has, then makeDeepNonNil is a noop, otherwise, it returns a copy of the passed
// Val with the deep nilability set to false.
// The parameter isFinalVal indicates whether later calls to these utility functions
// can update the `Val` further - uses cases are commented on to discuss why
// this is or is not desirable in given cases.
func (a Val) makeDeepNonNil(isFinalVal bool) Val {
	if a.IsDeepNilableSet {
		return a
	}
	return Val{
		IsNilable:        a.IsNilable,
		IsDeepNilable:    false,
		IsNilableSet:     a.IsNilableSet,
		IsDeepNilableSet: isFinalVal,
	}
}

// A ObservedMap represents a completed set of annotations read from a file or set of files,
// it can be checked against an assertionTree using RootAssertionNode.ReportErrors
//
// The maps are keyed by *ast.Idents because such an object is unique at each site in the code
// it is used; canonically, declarations are identified with the identifier used at the site
// of the declaration
//
// TODO: handle annotations for anonymous functions too
type ObservedMap struct {
	// this maps fields by the identifier declaring them to their Annotation type
	fieldAnnMap map[*types.Var]Val

	// this maps functions by the identifier declaring them to a slice with the
	// annotations of its params
	funcParamAnnMap map[*types.Func][]Val

	// this maps functions by the identifier declaring them to a slice with the
	// annotations of its results
	funcRetAnnMap map[*types.Func][]Val

	// this maps functions by the identifier declaring them to the annotations of their receivers
	funcRecvAnnMap map[*types.Func]Val

	// this maps named types to annotations describing their deep nilability
	deepTypeAnnMap map[*types.TypeName]Val

	// this maps declarations of global variables to their annotations
	globalVarsAnnMap map[*types.Var]Val

	// funcCallSiteParamAnnMap maps a function call site to a slice with the annotations of its
	// duplicated params at the call site.
	funcCallSiteParamAnnMap map[CallSite][]ArgLocAndVal

	// funcCallSiteRetAnnMap maps a function call site to a slice with the annotations of its
	// duplicated returns at the call site.
	funcCallSiteRetAnnMap map[CallSite][]Val
}

// CallSite uniquely identifies a function call. It contains the called function object and the
// code location of the call expression.
type CallSite struct {
	Fun      *types.Func
	Location token.Position
}

// ArgLocAndVal pairs the code location of the argument expression and the annotation value.
type ArgLocAndVal struct {
	Location token.Position
	Val      Val
}

// Range calls the passed function `op` on each annotation site in this map. If `setSitesOnly`
// is true, then it only calls `op` only on the sites with is<Deep?>NilableSet true.
func (m *ObservedMap) Range(op func(key Key, isDeep bool, val bool), setSitesOnly bool) {

	callOpOnKeyVal := func(key Key, val Val) {
		if !setSitesOnly || val.IsNilableSet {
			op(key, false /* isDeep */, val.IsNilable)
		}
		if !setSitesOnly || val.IsDeepNilableSet {
			op(key, true /* isDeep */, val.IsDeepNilable)
		}
	}

	for fld, val := range m.fieldAnnMap {
		callOpOnKeyVal(&FieldAnnotationKey{FieldDecl: fld}, val)
	}

	for fdecl, vals := range m.funcParamAnnMap {
		for i, val := range vals {
			callOpOnKeyVal(ParamKeyFromArgNum(fdecl, i), val)
		}
	}

	for fdecl, vals := range m.funcRetAnnMap {
		for i, val := range vals {
			callOpOnKeyVal(RetKeyFromRetNum(fdecl, i), val)
		}
	}

	for fdecl, val := range m.funcRecvAnnMap {
		callOpOnKeyVal((&RecvAnnotationKey{FuncDecl: fdecl}), val)
	}

	for tdecl, val := range m.deepTypeAnnMap {
		callOpOnKeyVal(&TypeNameAnnotationKey{TypeDecl: tdecl}, val)
	}

	for gvar, val := range m.globalVarsAnnMap {
		callOpOnKeyVal(&GlobalVarAnnotationKey{VarDecl: gvar}, val)
	}

	for callSite, vals := range m.funcCallSiteParamAnnMap {
		for i, argLocAndVal := range vals {
			// the location inside the callSite is the location of the call expression, we want
			// the location of every argument expression
			funcObj := callSite.Fun
			callOpOnKeyVal(NewCallSiteParamKey(funcObj, i, argLocAndVal.Location), argLocAndVal.Val)
		}
	}

	for callSite, vals := range m.funcCallSiteRetAnnMap {
		for i, val := range vals {
			callOpOnKeyVal(NewCallSiteRetKey(callSite.Fun, i, callSite.Location), val)
		}
	}
}

// defaults for anonymous functions and structs (ones for which definitions just can't be found
// aren't even looked up for now)
var (
	nonAnnotatedDefault = EmptyVal
)

const nilableKeyword = "nilable"
const nonNilKeyword = "nonnil"

var annotationKeyword = fmt.Sprintf("(%s|%s)", nilableKeyword, nonNilKeyword)

const sep = ","
const identRegexStr = "[a-zA-Z][a-zA-Z0-9]*"

const paramTemplateStr = "param %s"

var paramRegexStr = fmt.Sprintf(paramTemplateStr, "[0-9]+")

const resultTemplateStr = "result %s"

var resultRegexStr = fmt.Sprintf(resultTemplateStr, "[0-9]+")

var tokenRegexStr = fmt.Sprintf("((%s)|(%s)|(%s))",
	identRegexStr, paramRegexStr, resultRegexStr)

func paramStr(i int) string {
	return fmt.Sprintf(paramTemplateStr, fmt.Sprintf("%d", i))
}

func resultStr(i int) string {
	return fmt.Sprintf(resultTemplateStr, fmt.Sprintf("%d", i))
}

var deepIdentRegexStr = fmt.Sprintf("((\\*%s)|(%s\\[\\])|(<-%s)|%s)",
	tokenRegexStr, tokenRegexStr, tokenRegexStr, tokenRegexStr)
var seqRegexStr = fmt.Sprintf("%s\\((\\s*%s\\s*(%s\\s*%s\\s*)*)\\)",
	annotationKeyword, deepIdentRegexStr, sep, deepIdentRegexStr)
var seqRegex = regexp.MustCompile(seqRegexStr)

type nilabilitySet map[string]Val

// from a CommentGroup return a nilabilitySet of which identifiers are known annotated nilable
func nilabilityFromCommentGroup(group *ast.CommentGroup) nilabilitySet {
	set := make(nilabilitySet)
	// in each of the following utility functions, isFinalVal=true because literally read annotations
	// are considered final
	markNilable := func(s string) {
		if v, ok := set[s]; ok {
			set[s] = v.makeNilable(true)
		} else {
			set[s] = EmptyVal.makeNilable(true)
		}
	}
	markDeepNilable := func(s string) {
		if v, ok := set[s]; ok {
			set[s] = v.makeDeepNilable(true)
		} else {
			set[s] = EmptyVal.makeDeepNilable(true)
		}
	}
	markNonNil := func(s string) {
		if v, ok := set[s]; ok {
			set[s] = v.makeNonNil(true)
		} else {
			set[s] = EmptyVal.makeNonNil(true)
		}
	}
	markDeepNonNil := func(s string) {
		if v, ok := set[s]; ok {
			set[s] = v.makeDeepNonNil(true)
		} else {
			set[s] = EmptyVal.makeDeepNonNil(true)
		}
	}

	if group != nil {
		for _, comment := range group.List {
			for _, seqMatch := range seqRegex.FindAllStringSubmatch(comment.Text, -1) {

				deepFunc, shallowFunc := markDeepNonNil, markNonNil
				if seqMatch[1] == nilableKeyword {
					deepFunc, shallowFunc = markDeepNilable, markNilable
				}

				for _, match := range strings.Split(seqMatch[2], sep) {
					match = strings.TrimSpace(match)
					n := len(match)

					if n >= 2 && match[0] == '*' {
						deepFunc(match[1:])
						continue
					}

					if n >= 3 && match[n-2:] == "[]" {
						deepFunc(match[:n-2])
						continue
					}

					if n >= 3 && match[:2] == "<-" {
						deepFunc(match[2:])
						continue
					}

					shallowFunc(match)
				}
			}
		}
	}

	return set
}

// TypeIsDefaultNilable takes a type and returns true iff we assume default nilability for that
// type - in contrast to the remaining cases, in which we assume default non-nil.
func TypeIsDefaultNilable(t types.Type) bool {
	if t == nil {
		return false
	}

	// Builtin error type should be nilable by default.
	if types.Identical(t, util.ErrorType) {
		return true
	}

	// Slice, map, and chan should also be nilable by default (after unwrapping the named type).
	switch t.Underlying().(type) {
	case *types.Slice, *types.Map, *types.Chan:
		return true
	}

	return false
}

// TypeIsDeepDefaultNilable takes an `ast.Expr` that evaluates to a type, and returns true iff
// we assume default deep nilability for that type - in contrast to the remaining cases, in which
// we assume default deep non-nil.
func TypeIsDeepDefaultNilable(t types.Type) bool {
	switch t := t.(type) {
	case *types.Array:
		// the array case is handled different from others, since an array is not default nilable,
		// but can be default deeply nilable based on its containing type

		// recurse if multi-dimensional array until containing type is reached
		if e, ok := t.Elem().(*types.Array); ok {
			return TypeIsDeepDefaultNilable(e)
		}
		// assign deep nilability based on the element type
		return !util.TypeBarsNilness(t.Elem())
	case *types.Slice:
		return TypeIsDefaultNilable(t.Elem())
	case *types.Map:
		return TypeIsDefaultNilable(t.Elem())
	case *types.Pointer:
		return TypeIsDefaultNilable(t.Elem())
	case *types.Chan:
		return TypeIsDefaultNilable(t.Elem())
	case *types.Named:
		return TypeIsDeepDefaultNilable(t.Underlying())
	}
	return false
}

// checkNilability for a nilabilitySet checks to see if a string is mapped to an Annotation by that
// set. If it is, then that Annotation is returned. If not, then `nonNil` is returned.
// the type of the Annotation site is also passed, and it can possibly serve to mark a site
// as `nilable` when its Annotation doesn't indicate so.
func (set nilabilitySet) checkNilability(name string, t types.Type) Val {
	val := EmptyVal
	if v, ok := set[name]; ok {
		val = v
	}
	// in each of the following cases, isFinalVal=false because defaults are not considered final
	if TypeIsDefaultNilable(t) {
		val = val.makeNilable(false)
	}
	if TypeIsDeepDefaultNilable(t) {
		val = val.makeDeepNilable(false)
	}
	return val
}

func newObservedMap(pass *analysis.Pass, files []*ast.File) *ObservedMap {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)
	// TODO - only store annotations for fields/vars/parameters of types that do not bar nilness

	fieldAnnMap := make(map[*types.Var]Val)
	funcParamAnnMap := make(map[*types.Func][]Val)
	funcRetAnnMap := make(map[*types.Func][]Val)
	funcRecvAnnMap := make(map[*types.Func]Val)
	paramIndexMap := make(map[*types.Var]int)
	deepTypeAnnMap := make(map[*types.TypeName]Val)
	globalVarsAnnMap := make(map[*types.Var]Val)

	funcObjToFuncDecl := make(map[*types.Func]*ast.FuncDecl)
	funcCallSiteParamAnnMap := make(map[CallSite][]ArgLocAndVal)
	funcCallSiteRetAnnMap := make(map[CallSite][]Val)

	typeOf := func(expr ast.Expr) types.Type {
		return pass.TypesInfo.Types[expr].Type
	}

	// for a function declaration, accumulate its parameters from an *ast.Fieldlist object
	// listing them, look them up in the docstring, and return an equally long list of
	// annotationVals
	accFromFieldList := func(set nilabilitySet, fieldList *ast.FieldList, isParamList bool,
		isCallSiteAnnotation bool) []Val {
		if fieldList == nil {
			// this is included for nil-safety
			return nil
		}

		var annVals []Val
		var lookupKey string
		for _, field := range fieldList.List {
			if len(field.Names) == 0 {
				// case of anonymous field - on which we do not permit annotations
				// non-named fields

				if isParamList {
					lookupKey = paramStr(len(annVals))
				} else {
					lookupKey = resultStr(len(annVals))
				}

				annVals = append(annVals, set.checkNilability(lookupKey, typeOf(field.Type)))
			} else {
				for _, name := range field.Names {
					declFld := pass.TypesInfo.ObjectOf(name).(*types.Var)

					// for each named field, check the docstring for its Annotation and append that
					paramIndexMap[declFld] = len(annVals)

					var fieldType types.Type
					if t, ok := field.Type.(*ast.Ellipsis); ok {
						// in the case that our argument is variadic (hence has a type expression
						// of the form `...T`, we treat the arguments as having type `T` not type
						// `T[]`
						fieldType = typeOf(t.Elt)
					} else {
						fieldType = typeOf(field.Type)
					}

					if isCallSiteAnnotation {
						if isParamList {
							lookupKey = paramStr(len(annVals))
						} else {
							lookupKey = resultStr(len(annVals))
						}
					} else {
						lookupKey = name.Name
					}
					annVals = append(annVals, set.checkNilability(lookupKey, fieldType))
				}
			}
		}
		return annVals
	}

	readRecvAnnotations := func(decl *ast.FuncDecl, set nilabilitySet) Val {
		if decl.Recv != nil {
			if len(decl.Recv.List) > 1 {
				panic(fmt.Sprintf("Multiple receivers found for method %s", decl.Name))
			}
			return accFromFieldList(set, decl.Recv, false, false)[0]
		}
		return nonAnnotatedDefault
	}

	for _, file := range files {
		if conf.IsFileInScope(file) {
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					funcObj := pass.TypesInfo.ObjectOf(decl.Name).(*types.Func)
					set := nilabilityFromCommentGroup(decl.Doc)
					funcParamAnnMap[funcObj] = accFromFieldList(set, decl.Type.Params, true, false)
					funcRetAnnMap[funcObj] = accFromFieldList(set, decl.Type.Results, false, false)
					funcRecvAnnMap[funcObj] = readRecvAnnotations(decl, set)
					// store the mapping from the function object to the ast node.
					funcObjToFuncDecl[funcObj] = decl
				case *ast.GenDecl:
					// this is used for any declaration besides a function
					// here, we specifically look for declarations of struct types

					// this set will contain the nilability annotations read from the appropriate
					// docstring (this takes into account the syntax option to group declarations -
					// in which a single keyword may be used to declare a group)
					readDocNilabilitySet := func(specDoc *ast.CommentGroup) nilabilitySet {
						if len(decl.Specs) == 1 {
							// this reads declarations like type A struct {}
							return nilabilityFromCommentGroup(decl.Doc)
						}

						// this reads declarations like type (A struct{}, B struct{})
						return nilabilityFromCommentGroup(specDoc)
					}

					for _, spec := range decl.Specs {
						switch spec := spec.(type) {
						case *ast.ValueSpec:
							// we've found a declaration using the `var` or `const` keyword
							if decl.Tok == token.VAR {
								// narrow down to the case of a `var` declaration - i.e., a global var
								docNilabilitySet := readDocNilabilitySet(spec.Doc)
								for _, name := range spec.Names {
									varObj := pass.TypesInfo.ObjectOf(name).(*types.Var)
									globalVarsAnnMap[varObj] =
										docNilabilitySet.checkNilability(name.Name, typeOf(spec.Type))
								}
							}
						case *ast.TypeSpec:
							// we've found a declaration for a `type`

							docNilabilitySet := readDocNilabilitySet(spec.Doc)

							// readDeepNilability is called on type declarations of maps, slices, and
							// pointers to see if their contained values are nilable
							readDeepNilability := func() {
								typeName := pass.TypesInfo.ObjectOf(spec.Name).(*types.TypeName)
								deepTypeAnnMap[typeName] =
									docNilabilitySet.checkNilability(spec.Name.Name, typeOf(spec.Type))
							}
							var handleTypeVal func(expr ast.Expr)
							handleTypeVal = func(expr ast.Expr) {
								switch typeVal := expr.(type) {
								case *ast.StructType:
									for _, field := range typeVal.Fields.List {
										for _, name := range field.Names {
											fieldAnnMap[pass.TypesInfo.ObjectOf(name).(*types.Var)] =
												docNilabilitySet.checkNilability(name.Name, typeOf(field.Type))
										}
									}
								case *ast.InterfaceType:
									// iterate over the methods of this interface
									for _, method := range typeVal.Methods.List {
										switch len(method.Names) {
										case 1:
											// this is the common case - a simply declared method
											set := nilabilityFromCommentGroup(method.Doc)
											funcObj := pass.TypesInfo.ObjectOf(method.Names[0]).(*types.Func)
											funcParamAnnMap[funcObj] = accFromFieldList(set, method.Type.(*ast.FuncType).Params, true, false)
											funcRetAnnMap[funcObj] = accFromFieldList(set, method.Type.(*ast.FuncType).Results, false, false)
										case 0:
										// this is the case of inheritance - i.e. a method with another
										// method named within it, in this case the identifiers will
										// correctly resolve field references to the super-interface
										// so no work needs to be done
										default:
											// unrecognized
											panic("unrecognized case - method with > 1 names")
										}
									}
								case *ast.StarExpr:
									readDeepNilability()
								case *ast.MapType:
									readDeepNilability()
								case *ast.ArrayType:
									readDeepNilability()
								case *ast.Ident: // type alias - do nothing
								case *ast.SelectorExpr: // type alias - do nothing
								case *ast.FuncType: // function type - do nothing (for now)
								case *ast.ChanType:
									// TODO - treat channel types as deeply nilable at the typedef level
								case *ast.IndexExpr, *ast.IndexListExpr:
									// TODO - handle generics
								case *ast.ParenExpr:
									handleTypeVal(typeVal.X)
								default:
									panic(fmt.Sprintf("unrecognized type %T in AST - add a case for this", spec.Type))
								}
							}
							handleTypeVal(spec.Type)
						case *ast.ImportSpec: // do nothing - we don't care about these for annotations' sake
						default:
							panic(fmt.Sprintf("error - unrecognized spec: %T", spec))
						}
					}
				}
			}
		}
	}

	// Parse inline annotations at call sites.
	for _, file := range files {
		if !conf.IsFileInScope(file) {
			continue
		}
		// Store a mapping between single comment's line number to its text.
		comments := make(map[int]*ast.CommentGroup)
		for _, group := range file.Comments {
			if len(group.List) != 1 {
				continue
			}
			comment := group.List[0]
			comments[getLineFromPos(comment.Pos(), pass)] = group
		}

		// Now, find all *ast.CallExpr nodes and find their comment and extract annotations.
		// Comment nodes are floating in GO asts. https://github.com/golang/go/issues/20744
		// Thus, we require the comments for annotations are written in the same line as the
		// function call expression, so we can match them by line numbers.
		ast.Inspect(file, func(node ast.Node) bool {
			expr, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			commentGroup, ok := comments[getLineFromPos(expr.Pos(), pass)]
			if !ok {
				// No annotations for this CallExpr node, but we still need to traverse further since
				// there could be comments for nested CallExpr nodes.
				return true
			}

			ident := util.FuncIdentFromCallExpr(expr)
			// if ident is nil, keep searching for nested CallExpr nodes.
			if ident == nil {
				return true
			}
			funcObj, ok := pass.TypesInfo.ObjectOf(ident).(*types.Func)
			if !ok {
				// not a function, keep searching for nested CallExpr nodes.
				return true
			}

			set := nilabilityFromCommentGroup(commentGroup)
			if len(set) == 0 {
				// empty set, no annotation, keep searching for nested CallExpr nodes.
				return true
			}
			funcDecl, ok := funcObjToFuncDecl[funcObj]
			if !ok {
				panic(funcObj.FullName() + " not found but this should not happen since we " +
					"have parsed annotations once for every function declaration and the " +
					"mappings should have been set up.")
			}
			callSite := CallSite{Fun: funcObj, Location: util.PosToLocation(expr.Pos(), pass)}
			for i, val := range accFromFieldList(set, funcDecl.Type.Params, true, true) {
				argLoc := util.PosToLocation(expr.Args[i].Pos(), pass)
				funcCallSiteParamAnnMap[callSite] = append(funcCallSiteParamAnnMap[callSite],
					ArgLocAndVal{Location: argLoc, Val: val})
			}
			funcCallSiteRetAnnMap[callSite] = accFromFieldList(set, funcDecl.Type.Results, false, true)
			// keep searching for nested CallExpr nodes.
			return true
		})
	}
	return &ObservedMap{
		fieldAnnMap:             fieldAnnMap,
		funcParamAnnMap:         funcParamAnnMap,
		funcRetAnnMap:           funcRetAnnMap,
		funcRecvAnnMap:          funcRecvAnnMap,
		deepTypeAnnMap:          deepTypeAnnMap,
		globalVarsAnnMap:        globalVarsAnnMap,
		funcCallSiteParamAnnMap: funcCallSiteParamAnnMap,
		funcCallSiteRetAnnMap:   funcCallSiteRetAnnMap,
	}
}

func getLineFromPos(pos token.Pos, pass *analysis.Pass) int {
	return pass.Fset.Position(pos).Line
}
