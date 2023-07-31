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
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"runtime/debug"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Nonnil global variables should be forced to provide a nonnil instantiation value at their declaration."

// Result is the result struct for the Analyzer.
type Result struct {
	// FullTriggers is the slice of full triggers generated from the assertion analysis.
	FullTriggers []annotation.FullTrigger
	// Errors is the slice of errors if errors happened during analysis. We put the errors here as
	// part of the result of this sub-analyzer so that the upper-level analyzers can decide what
	// to do with them.
	Errors []error
}

// Analyzer checks if the nonnill global variables are initialized.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_global_var_analyzer",
	Doc:        _doc,
	Run:        run,
	ResultType: reflect.TypeOf((*Result)(nil)).Elem(),
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

func run(pass *analysis.Pass) (result interface{}, _ error) {
	// As a last resort, we recover from a panic when running the analyzer, convert the panic to
	// an error and return.
	defer func() {
		if r := recover(); r != nil {
			// Deferred functions are executed after a result is generated, so here we modify the
			// return value `result` in-place.
			e := fmt.Errorf("INTERNAL PANIC: %s\n%s", r, string(debug.Stack()))
			if retResult, ok := result.(Result); ok {
				retResult.Errors = append(retResult.Errors, e)
			} else {
				result = Result{Errors: []error{e}}
			}
		}
	}()

	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	if !conf.IsPkgInScope(pass.Pkg) {
		return Result{}, nil
	}

	var fullTriggers []annotation.FullTrigger
	for _, file := range pass.Files {
		if util.DocContainsIgnore(file.Doc) || !conf.IsFileInScope(file) {
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

	return Result{FullTriggers: fullTriggers}, nil
}
