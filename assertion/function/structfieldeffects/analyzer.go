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
// boundary summary: the field paths each function reads of its parameters and of the results its
// callers consume.
package structfieldeffects

import (
	"reflect"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

const _doc = "Compute the per-package struct-field boundary summary: the field paths each function " +
	"reads of its parameters and of the results its callers consume."

// Analyzer computes the package-level ParamFieldEffects boundary summary.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_struct_field_effects_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[*ParamFieldEffects])(nil)),
	Requires:   []*analysis.Analyzer{config.Analyzer},
}

func run(p *analysis.Pass) (*ParamFieldEffects, error) {
	pass := analysishelper.NewEnhancedPass(p)
	conf := pass.ResultOf[config.Analyzer].(*config.Config)
	if !conf.ExperimentalStructInitV2Enable || !conf.IsPkgInScope(pass.Pkg) {
		return &ParamFieldEffects{}, nil
	}
	return ComputeParamFieldEffects(pass), nil
}
