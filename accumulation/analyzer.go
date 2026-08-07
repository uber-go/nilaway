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

// Package accumulation coordinates the entire workflow and collects the annotations, full triggers,
// and then runs inference to generate and return all potential diagnostics for upper-level
// analyzers to report.
package accumulation

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/diagnostic"
	"go.uber.org/nilaway/inference"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Read the assertions and annotations from this package as Results from the corresponding" +
	" Analyzers, and read the annotations from upstream dependencies as Facts, then match them" +
	" against each other to obtain a list of triggered assertions that a later analyzer will report" +
	" as errors"

// Analyzer here is the accumulator that combines assertions and annotations to generate a list of
// triggered assertions that will become errors in the next Analyzer
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_accumulation_analyzer",
	Doc:        _doc,
	Run:        run,
	FactTypes:  []analysis.Fact{new(inference.InferredMap)},
	Requires:   []*analysis.Analyzer{config.Analyzer, assertion.Analyzer, annotation.Analyzer, diagnostic.NoLintAnalyzer},
	ResultType: reflect.TypeOf(([]analysis.Diagnostic)(nil)),
}

// run is the primary driver function for NilAway's analysis.
//
// It starts off by receiving results, if present, from each of the analyzers depended upon:
// assertions, annotations, and affiliations.
//
// It then merges the results of the assertions and affiliations analyzers, which both output lists
// of FullTriggers keyed by function declarations.
//
// Before we proceed to the inference stage, we create an empty inference engine, observe (load)
// any information from analyses of upstream dependencies, and load any manual annotations for the
// current (local) package. Assertions are then observed one by one to determine any further sites
// that must be determined from this package's constraints. All remaining assertions and
// undetermined sites are exported later, possibly to be determined by downstream packages.
//
// Lastly, we export the _incremental_ information we have gathered from the analysis of local
// package for use by downstream packages.
func run(p *analysis.Pass) (result interface{}, _ error) {
	pass := analysishelper.NewEnhancedPass(p)
	// As a last resort, we recover from a panic when running the analyzer, convert the panic to
	// a diagnostic and return.
	defer func() {
		if r := recover(); r != nil {
			// Deferred functions are executed after a result is generated, so here we modify the
			// return value `result` in-place.
			// Diagnostics with invalid positions (<= 0) will be silently suppressed, so here we use 1.
			d := analysis.Diagnostic{
				Pos:     1,
				Message: fmt.Sprintf("%s: %s\n%s", config.InternalPanicPrefix, r, string(debug.Stack())),
			}
			if diagnostics, ok := result.([]analysis.Diagnostic); ok {
				result = append(diagnostics, d)
			} else {
				result = []analysis.Diagnostic{d}
			}
		}
	}()

	conf := pass.ResultOf[config.Analyzer].(*config.Config)
	if !conf.IsPkgInScope(pass.Pkg) {
		// Must return a typed nil since the driver is using reflection to retrieve the result.
		return ([]analysis.Diagnostic)(nil), nil
	}

	assertionsResult := pass.ResultOf[assertion.Analyzer].(*analysishelper.Result[[]annotation.FullTrigger])
	annotationsResult := pass.ResultOf[annotation.Analyzer].(*analysishelper.Result[*annotation.ObservedMap])
	if err := errors.Join(annotationsResult.Err, assertionsResult.Err); err != nil {
		// For now, if there are any errors in the sub-analyzers, we directly emit diagnostics on the
		// errors. However, in the future we could implement error recovery and make use of the partial
		// information to continue the analysis.
		// Diagnostics with invalid positions (<= 0) will be silently suppressed, so here we use 1.
		return []analysis.Diagnostic{{Pos: 1, Message: fmt.Sprintf("INTERNAL ERROR(s):\n%s", err)}}, nil
	}

	diagnosticEngine := diagnostic.NewEngine(pass)

	// Create an inference engine and observe (load) information from upstream dependencies (i.e.,
	// mappings between annotation sites and their inferred values).
	inferenceEngine := inference.NewEngine(pass, diagnosticEngine)
	inferenceEngine.ObserveUpstream()

	// First observe all syntactically specified annotations from annotationsResult.
	inferenceEngine.ObserveAnnotations(annotationsResult.Res)

	// TODO: This is a suppression added for handling of struct field assignments. We plan to add
	//  object sensitivity to NilAway in the future, which will allow us to be more precise in struct fields'
	//  handling. Remove this suppression once we have the object sensitivity implemented (issue #339).
	for i := range assertionsResult.Res {
		t := &assertionsResult.Res[i]
		if _, ok := t.Consumer.Annotation.(*annotation.FldAssign); ok {
			// Create a fresh producer for this consumer so the shared producer is not
			// mutated.
			t.Producer = &annotation.ProduceTrigger{
				Annotation: &annotation.ProduceTriggerNever{},
				Expr:       t.Producer.Expr,
			}
		}
	}

	// Incorporate assertions from this package one-by-one into the inferredAnnotationMap, possibly
	// determining local and upstream sites in the process. This is guaranteed not to determine any
	// sites unless we really have a reason they have to be determined.
	inferenceEngine.ObservePackage(assertionsResult.Res)
	inferredMap := inferenceEngine.InferredMap()
	diagnostics := diagnosticEngine.Diagnostics(conf.GroupErrorMessages)

	// Export the _incremental_ information from this inferred map for analysis of downstream
	// packages via the Fact mechanism (which [uses gob encoding under the hood]). The custom
	// GobEncode / GobDecode methods of InferredAnnotationMap ensure that only incremental
	// information is encoded and exported - KEY for minimizing facts size. Note that we should
	// _never_ export nil maps / pointers due to [gob encoding]: "Nil pointers are not permitted,
	// as they have no value.".
	//
	// [uses gob encoding under the hood]: https://pkg.go.dev/golang.org/x/tools/go/analysis#hdr-Modular_analysis_with_Facts
	// [gob encoding]: https://pkg.go.dev/encoding/gob#hdr-Basics
	inferredMap.Export(pass)

	return diagnostics, nil
}

// This is required to use interface types in facts - see the implementation of GobRegister for the
// relevant interface implementations that could not be Gob encoded without this call
func init() {
	inference.GobRegister()
}
