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

package inference

import (
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/asthelper"
	"golang.org/x/tools/go/analysis"
)

// ModeOfInference is effectively an enum indicating the possible ways that we may conduct inference
// for NilAway
type ModeOfInference int

const (
	// NoInfer implies that all annotations sites are determined by syntactic annotations if present
	// and default otherwise
	NoInfer ModeOfInference = iota

	// FullInfer implies that no annotation site will be fixed before a sequence of assertions demands
	// it - this is the fully sound and complete version of inference: implication graphs are shared
	// between packages
	FullInfer
)

// DetermineMode searches the files in this package for docstrings that indicate
// inference should be entirely suppressed (returns NoInfer). By default, if no such
// docstring is found, multi-package inference is used (returns FullInfer).
func DetermineMode(pass *analysis.Pass) ModeOfInference {
	for _, file := range pass.Files {
		if asthelper.DocContains(file.Doc, config.NilAwayNoInferString) {
			return NoInfer
		}
	}
	return FullInfer
}
