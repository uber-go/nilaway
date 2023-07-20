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
	"reflect"
	"runtime/debug"

	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Read the annotations for each struct, interface, and function in this package, returning" +
	" the results so that they may be matched against assertions by an accumulator"

// Result is the result struct for the Analyzer.
type Result struct {
	// AnnotationMap is the map generated from reading the annotations in the source code.
	AnnotationMap *ObservedMap
	// Errors is the slice of errors if errors happened during analysis. We put the errors here as
	// part of the result of this sub-analyzer so that the upper-level analyzers can decide what
	// to do with them.
	Errors []error
}

// Analyzer here is the analyzer than reads annotations and passes them onto the accumulator to
// be matched against assertions
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_annotation_analyzer",
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

	if !config.PackageIsInScope(pass.Pkg) {
		return Result{AnnotationMap: new(ObservedMap)}, nil
	}

	return Result{AnnotationMap: newObservedMap(pass, pass.Files)}, nil
}
