//  Copyright (c) 2024 Uber Technologies, Inc.
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

// Package preprocess hosts preprocessing logic for the input (e.g., CFGs etc.) to make it more
// amenable to analysis.
package preprocess

import "golang.org/x/tools/go/analysis"

// Preprocessor handles different preprocessing logic for different types of input.
type Preprocessor struct {
	pass *analysis.Pass
}

// New returns a new Preprocessor.
func New(pass *analysis.Pass) *Preprocessor {
	return &Preprocessor{pass: pass}
}
