//  Copyright (c) 2026 Uber Technologies, Inc.
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

// Package structfieldeffects implements a sub-analyzer that computes the package-level struct-field
// boundary summary: the field paths each function writes or reads on its parameters and the result
// fields its callers consume.
package structfieldeffects

import (
	"cmp"
	"fmt"
	"go/types"
	"reflect"
	"slices"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/objectpath"
)

const _doc = "Compute the per-package struct-field boundary summary: the field paths each function " +
	"writes or reads on its parameters and the result fields its callers consume."

// Analyzer computes the package-level ParamFieldEffects boundary summary.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_struct_field_effects_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[*ParamFieldEffects])(nil)),
	FactTypes:  []analysis.Fact{new(ParamFieldReadsPackageFact)},
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

func run(p *analysis.Pass) (*ParamFieldEffects, error) {
	pass := analysishelper.NewEnhancedPass(p)
	conf := pass.ResultOf[config.Analyzer].(*config.Config)
	if !conf.ExperimentalStructInitV2Enable || !conf.IsPkgInScope(pass.Pkg) {
		return &ParamFieldEffects{}, nil
	}

	packageSummary, forwardingEdges, callees := computeParamFieldEffects(pass)
	if err := importUsedParamReads(pass, packageSummary.ParamReads, callees); err != nil {
		return nil, err
	}

	closeParamFieldSets(packageSummary.ParamWrites, forwardingEdges)
	closeParamFieldSets(packageSummary.ParamReads, forwardingEdges)
	fact := &ParamFieldReadsPackageFact{}
	encoder := &objectpath.Encoder{}
	for funcObj := range packageSummary.ParamReads {
		if funcObj.Pkg() != pass.Pkg || !funcObj.Exported() {
			continue
		}
		// Export only parameter reads. Return reads remain local because callers infer them
		// from their dereferences of results, opposite the direction of fact propagation.
		reads := packageSummary.ParamReads.sortedPaths(funcObj)
		if len(reads) == 0 {
			continue
		}
		path, err := encoder.For(funcObj)
		if err != nil {
			return nil, fmt.Errorf("create object path for exported function %s: %w", funcObj, err)
		}
		fact.Functions = append(fact.Functions, FunctionParamFieldReads{FunctionObjectPath: path, ParamReads: reads})
	}
	if len(fact.Functions) > 0 {
		// Functions were collected by ranging a map. Sorting makes the serialized fact
		// deterministic and maintains the ordering required by BinarySearchFunc on import.
		slices.SortFunc(fact.Functions, func(left, right FunctionParamFieldReads) int {
			return cmp.Compare(left.FunctionObjectPath, right.FunctionObjectPath)
		})
		pass.ExportPackageFact(fact)
	}

	return packageSummary, nil
}

func importUsedParamReads(pass *analysishelper.EnhancedPass, paramReads fieldEffects, callees map[*types.Func]bool) error {
	calleesByPackage := make(map[*types.Package][]*types.Func)
	for callee := range callees {
		if callee.Pkg() != nil && callee.Pkg() != pass.Pkg {
			calleesByPackage[callee.Pkg()] = append(calleesByPackage[callee.Pkg()], callee)
		}
	}

	packages := make([]*types.Package, 0, len(calleesByPackage))
	for pkg := range calleesByPackage {
		packages = append(packages, pkg)
	}

	encoder := &objectpath.Encoder{}
	for _, pkg := range packages {
		var fact ParamFieldReadsPackageFact
		if !pass.ImportPackageFact(pkg, &fact) {
			continue
		}
		for _, callee := range calleesByPackage[pkg] {
			path, err := encoder.For(callee)
			if err != nil {
				return fmt.Errorf("create object path for imported function %s: %w", callee, err)
			}
			index, found := slices.BinarySearchFunc(fact.Functions, path, func(entry FunctionParamFieldReads, path objectpath.Path) int {
				return cmp.Compare(entry.FunctionObjectPath, path)
			})
			if found {
				seedImportedParamReads(paramReads, callee, fact.Functions[index].ParamReads)
			}
		}
	}
	return nil
}
