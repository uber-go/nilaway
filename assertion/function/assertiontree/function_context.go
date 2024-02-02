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

package assertiontree

import (
	"go/ast"
	"go/types"

	"go.uber.org/nilaway/assertion/anonymousfunc"
	"go.uber.org/nilaway/assertion/function/functioncontracts"
	"golang.org/x/tools/go/analysis"
)

// SelectorExprMap is used to cache artificially created ast selector expressions
type SelectorExprMap map[ast.Expr]map[*types.Var]*ast.SelectorExpr

// FunctionContext holds the context of the function during backpropagation. The state should include function
// declaration, map objects that are created at initialization, and configurations that are passed through
// function analyzer.
type FunctionContext struct {
	// funcDecl records the function declaration this assertion node is being used to check.
	funcDecl *ast.FuncDecl

	// funcLit records the func lit node if this function is created from it; otherwise nil.
	funcLit *ast.FuncLit

	// pass records the overarching analysis pass - needed for identifier resolution.
	pass *analysis.Pass

	// selectorExpressionCache we cache artificially created selector expressions nodes to avoid
	// duplication. Duplication is dangerous as it will result in duplicate triggers and the
	// analysis will not reach a fixpoint.
	selectorExpressionCache SelectorExprMap

	// fakeIdentMap is used to undo the creation of fake identifiers as sometimes needed
	// (see annotation.GetObjByIdent) - This is not really a hack - it exists exactly to
	// make up for the fact that some types.Objects just aren't matched with an AST node
	// if they come from an upstream package, and we use this as a new source of
	// canonical association between the two because we need access to both modes
	fakeIdentMap map[*ast.Ident]types.Object

	// pkgFakeIdentMap is similar to fakeIdentMap, but it is used for the entire package for better
	// performance.
	pkgFakeIdentMap map[*ast.Ident]types.Object

	// funcLitMap stores the mapping between func lit nodes and the auxiliary information.
	funcLitMap map[*ast.FuncLit]*anonymousfunc.FuncLitInfo

	// functionConfig contains the user set configuration for analyzing a function
	functionConfig FunctionConfig

	// funcContracts stores the function contracts of all the functions.
	funcContracts functioncontracts.Map
}

// FunctionConfig is meant to hold all the user set configuration for analyzing a function
type FunctionConfig struct {
	// EnableStructInitCheck is a flag to enable tracking struct initializations.
	EnableStructInitCheck bool
	// EnableAnonymousFunc is a flag to enable checking anonymous functions.
	EnableAnonymousFunc bool
}

// NewFunctionContext returns a new FunctionContext and initializes all the maps
func NewFunctionContext(
	pass *analysis.Pass,
	decl *ast.FuncDecl,
	funcLit *ast.FuncLit,
	functionConfig FunctionConfig,
	funcLitMap map[*ast.FuncLit]*anonymousfunc.FuncLitInfo,
	pkgFakeIdentMap map[*ast.Ident]types.Object,
	funcContracts functioncontracts.Map,
) FunctionContext {
	return FunctionContext{
		pass:                    pass,
		funcDecl:                decl,
		funcLit:                 funcLit,
		fakeIdentMap:            make(map[*ast.Ident]types.Object),
		selectorExpressionCache: make(SelectorExprMap),
		functionConfig:          functionConfig,
		funcLitMap:              funcLitMap,
		pkgFakeIdentMap:         pkgFakeIdentMap,
		funcContracts:           funcContracts,
	}
}

// getCachedSelectorExpr returns cached selector expression. It returns artificially created ast expression. Which is cached to
// avoid duplication of triggers.
// if not present in the cache creates a new expression and adds it to the cache.
func (fc *FunctionContext) getCachedSelectorExpr(fieldDecl *types.Var, fieldOf ast.Expr, fieldIdent *ast.Ident) *ast.SelectorExpr {
	selectorExpressionCache := fc.selectorExpressionCache
	if _, ok := selectorExpressionCache[fieldOf]; !ok {
		selectorExpressionCache[fieldOf] = make(map[*types.Var]*ast.SelectorExpr)
	}

	if selExpr, ok := selectorExpressionCache[fieldOf][fieldDecl]; ok {
		return selExpr
	}

	selExpr := &ast.SelectorExpr{
		Sel: fieldIdent,
		X:   fieldOf,
	}

	// TODO: This check should ideally be not necessary but currently Nilaway reporting FP
	if fieldMap, ok := selectorExpressionCache[fieldOf]; ok {
		fieldMap[fieldDecl] = selExpr
	}

	return selExpr
}

// AddFakeIdent adds fake ident to fakeIdentMap
func (fc *FunctionContext) AddFakeIdent(ident *ast.Ident, obj types.Object) {
	fc.fakeIdentMap[ident] = obj
}

// findFakeIdent returns the object mapped to ident from fakeIdentMap
func (fc *FunctionContext) findFakeIdent(ident *ast.Ident) types.Object {
	if obj, ok := fc.fakeIdentMap[ident]; ok {
		return obj
	}
	return fc.pkgFakeIdentMap[ident]
}
