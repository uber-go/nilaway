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

func run(pass *analysis.Pass) ([]annotation.FullTrigger, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return nil, nil
	}

	var fullTriggers []annotation.FullTrigger
	for _, file := range pass.Files {
		if !conf.IsFileInScope(file) {
			continue
		}

		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.VAR {
				continue
			}
			for _, spec := range genDecl.Specs {
				fullTriggers = append(fullTriggers, analyzeValueSpec(pass, spec.(*ast.ValueSpec))...)
			}
		}
	}

	return fullTriggers, nil
}
