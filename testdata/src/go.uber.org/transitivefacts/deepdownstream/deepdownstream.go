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

// Package deepdownstream is the final hop of the four-package transitive-facts chain. It imports
// ONLY outer, which imports only middle, which imports upstream: the fact about upstream's S.Get
// therefore has to be forwarded twice, the second time by a package that never imported the
// package the fact originated in.
package deepdownstream

import "go.uber.org/transitivefacts/outer"

func useDeepTransitive() {
	// Three-hop flow: upstream -> middle -> outer -> deepdownstream.
	print(*outer.MakeWrapper().Inner.Get()) //want "result 0 of `Get\\(\\)` dereferenced"
}
