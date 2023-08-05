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
	"fmt"
	"reflect"
	"runtime/debug"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion"
	"go.uber.org/nilaway/assertion/function/assertiontree"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/inference"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Read the assertions and annotations from this package as Results from the corresponding" +
	" Analyzers, and read the annotations from upstream dependencies as Facts, then match them" +
	" against each other to obtain a list of triggered assertions that a later analyzer will report" +
	" as errors"

// Analyzer here is the accumulator that combines assertions and annotations to generate a list of
// triggered assertions that will become errors in the next Analyzer
var Analyzer = &analysis.Analyzer{
	Name: "nilaway_accumulation_analyzer",
	Doc:  _doc,
	Run:  run,
	FactTypes: []analysis.Fact{
		new(inference.InferredMap),
	},
	Requires:   []*analysis.Analyzer{config.Analyzer, assertion.Analyzer, annotation.Analyzer},
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
// current (local) package. Then, we start the inference depending on the mode:
//
// - Mode inference.NoInfer: No inference
// We simply check all assertions against the manual annotations and upstream values (which can
// possibly determine upstream values but cannot determine the already-determined local
// values) and report errors if there are any.
//
// - Mode inference.FullInfer: Multi-Package Inference
// Assertions are observed one by one to determine any further sites that must be determined from
// this package's constraints. This is the extent of determination done, and all remaining
// assertions and undetermined sites remain are exported later, possibly to be determined by
// downstream packages.
//
// Lastly, we export the _incremental_ information we have gathered from the analysis of local
// package for use by downstream packages.
func run(pass *analysis.Pass) (result interface{}, _ error) {
	// As a last resort, we recover from a panic when running the analyzer, convert the panic to
	// a diagnostic and return.
	defer func() {
		if r := recover(); r != nil {
			// Deferred functions are executed after a result is generated, so here we modify the
			// return value `result` in-place.
			// Diagnostics with invalid positions (<= 0) will be silently suppressed, so here we use 1.
			d := analysis.Diagnostic{Pos: 1, Message: fmt.Sprintf("INTERNAL PANIC: %s\n%s", r, string(debug.Stack()))}
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

	assertionsResult := pass.ResultOf[assertion.Analyzer].(assertion.Result)
	annotationsResult := pass.ResultOf[annotation.Analyzer].(annotation.Result)
	var errs []error
	for _, resultErrs := range [...][]error{assertionsResult.Errors, annotationsResult.Errors} {
		errs = append(errs, resultErrs...)
	}

	// For now, if there are any errors in the sub-analyzers, we directly emit diagnostics on the
	// errors. However, in the future we could implement error recovery and make use of the partial
	// information to continue the analysis.
	if len(errs) != 0 {
		return errorsToDiagnostics(errs), nil
	}

	// Determine inference type based on comments in package doc string.
	mode := inference.DetermineMode(pass)

	// Create an engine and observe (load) information from upstream dependencies (i.e., mappings
	// between annotation sites and their inferred values).
	engine := inference.NewEngine(pass)
	engine.ObserveUpstream()

	// First observe all annotations from annotationsResult (observes only syntactic annotations
	// for FullInfer mode, otherwise all annotations for NoInfer)
	engine.ObserveAnnotations(annotationsResult.AnnotationMap, mode)

	var (
		inferredMap *inference.InferredMap
		diagnostics []analysis.Diagnostic
	)
	switch mode {
	case inference.FullInfer:
		// Incorporate assertions from this package one-by-one into the inferredAnnotationMap, possibly
		// determining local and upstream sites in the process. This is guaranteed not to determine any
		// sites unless we really have a reason they have to be determined.
		engine.ObservePackage(assertionsResult.FullTriggers)
		inferredMap, diagnostics = engine.InferredMapWithDiagnostics()

	case inference.NoInfer:
		// In non-inference case - use the classical assertionNode.CheckErrors method to determine error outputs
		inferredMap, diagnostics = engine.InferredMapWithDiagnostics()
		diagnostics = append(diagnostics, buildDiagnostics(pass, checkErrors(assertionsResult.FullTriggers, inferredMap))...)

	default:
		panic("Invalid mode for running NilAway")
	}

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

// errorsToDiagnostics converts the internal errors to a slice of analysis.Diagnostic to be reported.
func errorsToDiagnostics(errs []error) []analysis.Diagnostic {
	diagnostics := make([]analysis.Diagnostic, len(errs))
	for i, err := range errs {
		// Diagnostics with invalid positions (<= 0) will be silently suppressed, so here we use 1.
		diagnostics[i] = analysis.Diagnostic{Pos: 1, Message: "INTERNAL ERROR: " + err.Error()}
	}
	return diagnostics
}

// buildDiagnostics takes a list of FullTriggers, which are assumed to already have been checked
// and are known to fail, and returns a slice of the appropriate diagnostics for all the triggers.
func buildDiagnostics(pass *analysis.Pass, triggers []annotation.FullTrigger) []analysis.Diagnostic {
	conflicts := inference.ConflictList{NoGrouping: true}
	for _, trigger := range triggers {
		conflicts.AddSingleAssertionConflict(pass, trigger)
	}
	return conflicts.Diagnostics()
}

// checkErrors iterates over a set of full triggers, checking each one against a given annotation
// map to see if it fails and if so appending it to the returned list.
func checkErrors(triggers []annotation.FullTrigger, annMap annotation.Map) []annotation.FullTrigger {
	// Filter triggers for error return handling -- inter-procedural and annotations-based (no inference).
	// (Note that since we are using FilterTriggersForErrorReturn as a preprocessing step here, we can directly use its
	// first output `filteredTriggers` to check and report errors. The second output of raw `deleted triggers` is not
	// needed in this situation, and hence suppressed with a blank identifier `_`)
	filteredTriggers, _ := assertiontree.FilterTriggersForErrorReturn(
		triggers,
		func(p *annotation.ProduceTrigger) assertiontree.ProducerNilability {
			if !p.Annotation.CheckProduce(annMap) {
				return assertiontree.ProducerIsNonNil
			}
			// ProducerNilabilityUnknown is returned here since all we know at this point is that `p` is nilable,
			// which means that it could be nil, but is not guaranteed to be always nil
			return assertiontree.ProducerNilabilityUnknown
		},
	)

	var triggered []annotation.FullTrigger
	for _, trigger := range filteredTriggers {
		// Skip checking any full triggers we created by duplicating from contracted functions
		// to the caller function.
		if !trigger.CreatedFromDuplication && trigger.Check(annMap) {
			triggered = append(triggered, trigger)
		}
	}
	return triggered
}

// This is required to use interface types in facts - see the implementation of GobRegister for the
// relevant interface implementations that could not be Gob encoded without this call
func init() {
	inference.GobRegister()
}
