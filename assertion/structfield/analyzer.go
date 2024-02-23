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

// Package structfield implements a sub-analyzer that collects struct fields accessed within a
// function to aid the analysis of the main function analyzer.
package structfield

import (
	"go/ast"
	"reflect"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Collect relevant struct fields being assigned and/or accessed from within each function to later allow creation of triggers applicable to only those fields"

// Analyzer collects struct fields accessed (e.g., assignments) from within a function.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_struct_field_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[*FieldContext])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

func run(pass *analysis.Pass) (*FieldContext, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	fieldContext := &FieldContext{fieldMap: make(relevantFieldsMap)}

	if !conf.IsPkgInScope(pass.Pkg) {
		return fieldContext, nil
	}

	for _, file := range pass.Files {
		if !conf.IsFileInScope(file) {
			continue
		}

		for _, decl := range file.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				fieldContext.processFunc(funcDecl, pass)
			}
		}
	}

	return fieldContext, nil
}
