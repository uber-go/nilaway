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

// Package downstream is the final hop of the transitive-facts chain. It imports ONLY middle —
// not upstream. Calling Get on the value returned by middle.Make() is legal without importing
// upstream, but knowing that Get returns nilable requires upstream's inference fact, which must
// survive transitive re-export through middle for the dereference below to be flagged.
package downstream

import "go.uber.org/transitivefacts/middle"

func useTransitive() {
	// Two-hop flow: Get's return was determined nilable in upstream, whose fact must travel
	// upstream -> middle -> downstream. Drivers pruning imported package facts miss this error.
	print(*middle.Make().Get()) //want "result 0 of `Get\\(\\)` dereferenced"
}

func useOneHop() {
	// One-hop control: NilableFunc's nilability is in middle's own fact, which every driver
	// reads directly. This error is reported even by fact-pruning drivers.
	print(*middle.NilableFunc()) //want "result 0 of `NilableFunc\\(\\)` dereferenced"
}
