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

// Package global implements a sub-analyzer to create full triggers for global variables.
package global

import (
	"go/ast"
	"go/token"
	"reflect"
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Nonnil global variables should be forced to provide a nonnil instantiation value at their declaration."

// Analyzer checks if the nonnill global variables are initialized.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_global_var_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[[]annotation.FullTrigger])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

func run(p *analysis.Pass) ([]annotation.FullTrigger, error) {
	pass := analysishelper.NewEnhancedPass(p)
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return nil, nil
	}

	var fullTriggers []annotation.FullTrigger
	for _, file := range pass.Files {
		if !conf.IsFileInScope(file) {
			continue
		}
		initFuncDecls := getInitFuncDecls(pass.Fset, file)

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				continue
			}
			for _, spec := range genDecl.Specs {
				fullTriggers = append(fullTriggers, analyzeValueSpec(pass, spec.(*ast.ValueSpec), initFuncDecls)...)
			}
		}
	}

	return fullTriggers, nil
}

// getInitFuncDecls searches for init (and TestMain in _test.go files) declarations and all
// related functions in the given *ast.File.
// It returns a slice of *ast.FuncDecl representing these functions and all functions called
// directly or indirectly from them, avoiding infinite recursion in case of cyclic calls.
func getInitFuncDecls(fset *token.FileSet, file *ast.File) []*ast.FuncDecl {
	if file == nil {
		return nil
	}
	funcDecls := make(map[string]*ast.FuncDecl)
	for _, decl := range file.Decls {
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			funcDecls[funcDecl.Name.Name] = funcDecl
		}
	}

	var initFuncDecls []*ast.FuncDecl
	// visitedFuncs tracks processed functions to prevent infinite recursion and duplicate processing
	visitedFuncs := make(map[string]struct{})
	var findRelatedFuncs func(*ast.FuncDecl)
	findRelatedFuncs = func(funcDecl *ast.FuncDecl) {
		if _, visited := visitedFuncs[funcDecl.Name.Name]; visited {
			return
		}
		visitedFuncs[funcDecl.Name.Name] = struct{}{}
		// Some functions are implemented outside Go (for example, in assembly or through
		// go:linkname) and therefore have no body. They cannot contain assignments for us to
		// inspect, and passing their typed-nil body to ast.Inspect causes a panic.
		if funcDecl.Body == nil {
			return
		}
		initFuncDecls = append(initFuncDecls, funcDecl)
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			if callExpr, ok := n.(*ast.CallExpr); ok {
				if ident, ok := callExpr.Fun.(*ast.Ident); ok {
					if funcDecl, exists := funcDecls[ident.Name]; exists {
						findRelatedFuncs(funcDecl)
					}
				}
			}
			return true
		})
	}

	isTestFile := strings.HasSuffix(fset.Position(file.Pos()).Filename, "_test.go")
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if funcDecl.Name.Name == "init" || (isTestFile && funcDecl.Name.Name == "TestMain" && funcDecl.Recv == nil) {
			findRelatedFuncs(funcDecl)
			visitedFuncs = make(map[string]struct{})
		}
	}
	return initFuncDecls
}
