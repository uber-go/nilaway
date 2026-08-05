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

// Package upstream is the root of a three-package chain (upstream <- middle <- downstream) that
// exercises _transitive_ propagation of NilAway's inference facts: the nilability of S.Get's
// return is established here, never mentioned in package middle, and consumed in package
// downstream, which does NOT import this package directly (methods are callable on values
// obtained through middle's API without naming this package).
//
// Under drivers that inherit all package facts across the action graph (analysistest,
// singlechecker, golangci-lint), the chain works and the error is reported in downstream. Under
// modular drivers that exchange facts via x/tools' internal/facts encoding (go vet/unitchecker,
// bazel/nogo), imported package facts are no longer re-exported since x/tools v0.12.0
// (golang/tools@d75c38746e "internal/facts: don't reexport imported facts unnecessarily"), so
// this package's fact never reaches downstream and the error there is silently missed.
package upstream

// S is a struct whose Get method below is the carrier of the transitively-needed inference fact.
type S struct{}

// Get returns nil directly, so its return site is determined nilable in _this_ package's
// exported InferredMap package fact. Nothing in this package or in middle dereferences it, so
// the determination reaches a use site only if facts propagate transitively.
func (s *S) Get() *int {
	return nil
}

// NewS returns a non-nil S so that calling methods on the chain is otherwise safe.
func NewS() *S {
	return &S{}
}
