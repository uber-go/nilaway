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
	"fmt"
	"go/ast"
	"reflect"
	"runtime/debug"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Collect relevant struct fields being assigned and/or accessed from within each function to later allow creation of triggers applicable to only those fields"

// Result is the result struct for the Analyzer.
type Result struct {
	// Context stores struct fields accessed (e.g., assignments) from within a function.
	Context *FieldContext
	// Errors is the slice of errors if errors happened during analysis. We put the errors here as
	// part of the result of this sub-analyzer so that the upper-level analyzers can decide what
	// to do with them.
	Errors []error
}

// Analyzer collects struct fields accessed (e.g., assignments) from within a function
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_struct_field_analyzer",
	Doc:        _doc,
	Run:        run,
	ResultType: reflect.TypeOf((*Result)(nil)).Elem(),
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

	fieldContext := &FieldContext{fieldMap: make(relevantFieldsMap)}

	if !config.PackageIsInScope(pass.Pkg) {
		return Result{Context: fieldContext}, nil
	}

	for _, file := range pass.Files {
		if util.DocContainsIgnore(file.Doc) || !config.FileIsInScope(file) {
			continue
		}

		for _, decl := range file.Decls {
			if funcDecl, ok := decl.(*ast.FuncDecl); ok {
				fieldContext.processFunc(funcDecl, pass)
			}
		}
	}

	return Result{Context: fieldContext}, nil
}
