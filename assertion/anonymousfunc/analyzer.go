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

// Package anonymousfunc implements a sub-analyzer to analyze anonymous functions in a package.
package anonymousfunc

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"strconv"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Collect variables from closure that are being assigned and/or accessed from within each" +
	" anonymous function to later update the the anonymous function's signature at the call side"

// Analyzer collects a set of variables from closure for each function literal
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_anonymous_func_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[map[*ast.FuncLit]*FuncLitInfo])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

// FuncLitInfo is the struct that stores auxiliary information (e.g., the closure variables it uses,
// its corresponding fake func decl node, etc.) about a func lit that is useful in the main
// analysis.
type FuncLitInfo struct {
	// FakeFuncDecl is the fake func decl node created for the func lit node so that it can be
	// treated like a regular function declaration during the analysis. The parameter list is
	// extended to include variables used from the closure.
	FakeFuncDecl *ast.FuncDecl
	// FakeFuncObj is the fake object for the fake func decl node.
	FakeFuncObj *types.Func
	// ClosureVars stores a slice of assigned / accessed variables from closure within each
	// function literal in the order of their appearances.
	ClosureVars []*VarInfo
}

// VarInfo keeps the information about a variable (*ast.Ident) and its associated object type
// (*types.Var). It can either be a real identifier we collected from the analysis, or a fake
// one we created to aid the analysis.
type VarInfo struct {
	// Ident stores the ident node.
	Ident *ast.Ident
	// Obj stores the named entity of the variable.
	Obj *types.Var
}

// _fakeFuncDeclPrefix is prepended to the fake func decl node we generated for the func lit nodes.
// It contains an illegal character to avoid collisions with other variables.
const _fakeFuncDeclPrefix = "__anonymousFunction$"

func run(pass *analysis.Pass) (map[*ast.FuncLit]*FuncLitInfo, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return nil, nil
	}

	funcLitMap := make(map[*ast.FuncLit]*FuncLitInfo)

	for _, file := range pass.Files {
		if !conf.IsFileInScope(file) || !conf.ExperimentalAnonymousFuncEnable {
			continue
		}

		// Search for top-level function literal declarations across all declarations in a file and call
		// collectClosure on that, any further recursions will happen in collectClosure
		closureMap := make(map[*ast.FuncLit][]*VarInfo)
		ast.Inspect(file, func(node ast.Node) bool {
			if n, ok := node.(*ast.FuncLit); ok {
				collectClosure(n, pass, closureMap)
				return false
			}
			return true
		})

		for funcLit, vars := range closureMap {
			fakeDecl, fakeType := createFakeFuncDecl(pass, funcLit, vars)

			funcLitMap[funcLit] = &FuncLitInfo{
				FakeFuncDecl: fakeDecl,
				FakeFuncObj:  fakeType,
				ClosureVars:  vars,
			}
		}
	}

	return funcLitMap, nil
}

// createFakeFuncDecl creates a fake function declaration (AST node and a type object) for the
// given func lit node, where the parameter list is extended to include fake parameters that
// represent the closure variables.
func createFakeFuncDecl(pass *analysis.Pass, funcLit *ast.FuncLit, fakeParams []*VarInfo) (*ast.FuncDecl, *types.Func) {
	// The name for the node is named "<prefix>Line:Column" for easier identification.
	pos := pass.Fset.Position(funcLit.Pos())
	name := _fakeFuncDeclPrefix + strconv.Itoa(pos.Line) + ":" + strconv.Itoa(pos.Column)
	ident := &ast.Ident{
		NamePos: funcLit.Pos(),
		Name:    name,
	}
	// The list of formal AST parameter nodes (*ast.Field nodes) is extended.
	fakeFields := make([]*ast.Field, len(fakeParams))
	for i, p := range fakeParams {
		fakeFields[i] = &ast.Field{
			// Note that there is no easy way to retrieve the AST nodes for the type of the
			// parameter (we only have type information from the type-checking package `go/types`,
			// via `pass.TypeInfo`), and we are not using the AST type throughout the rest of
			// NilAway system. So here we simply assign a nil to the Type field. However, this is
			// a potential risk and should be resolved upon further investigations.
			// TODO: fix this
			Type: nil,
			Names: []*ast.Ident{
				p.Ident,
			},
		}
	}
	funcDecl := &ast.FuncDecl{
		Name: ident,
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: append(funcLit.Type.Params.List, fakeFields...),
			},
		},
		Body: funcLit.Body,
	}

	// Then, create the fake func type for the fake decl for type resolution.
	// Create fake func signature type from func lit signature.
	// Anonymous functions do not have receiver or type parameters. For more detail: https://go.dev/ref/spec#Function_literals
	sig := pass.TypesInfo.TypeOf(funcLit).(*types.Signature)
	if sig.Recv() != nil || sig.RecvTypeParams() != nil || sig.TypeParams() != nil {
		panic(fmt.Sprintf("receiver or type parameters of an anonymous function at %s:%d.%d is not nil",
			pos.Filename, pos.Line, pos.Column))
	}

	// Extend the parameter list for the types as well.
	paramTypes := make([]*types.Var, sig.Params().Len()+len(fakeParams))
	for i := 0; i < sig.Params().Len(); i++ {
		paramTypes[i] = sig.Params().At(i)
	}
	for i := 0; i < len(fakeParams); i++ {
		paramTypes[sig.Params().Len()+i] = fakeParams[i].Obj
	}

	fakeSig := types.NewSignatureType(nil /* recv */, nil /* recvTypeParams */, nil, /* typeParams */
		types.NewTuple(paramTypes...), sig.Results(), sig.Variadic())
	fakeFuncType := types.NewFunc(funcLit.Pos(), pass.Pkg, ident.Name, fakeSig)

	return funcDecl, fakeFuncType
}
