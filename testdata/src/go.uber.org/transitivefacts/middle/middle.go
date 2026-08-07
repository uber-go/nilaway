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

// Package middle is the intermediate hop of the transitive-facts chain. It re-exposes
// upstream.S through its own API but crucially never calls, guards, or otherwise mentions
// upstream's S.Get method. Because InferredMap.Export is incremental (it exports only
// information not already present in upstream maps), this package's fact therefore carries
// nothing about S.Get: its return nilability lives solely in upstream's fact, and downstream can
// only learn it if the driver propagates facts transitively.
package middle

import "go.uber.org/transitivefacts/upstream"

// Make wraps upstream.NewS, making upstream.S reachable from this package's API so that the
// downstream package can call its Get method without importing upstream directly.
func Make() *upstream.S {
	return upstream.NewS()
}

// NilableFunc returns nil directly, so its return site is determined nilable in _this_ package's
// own incremental map. It serves as the one-hop control for the experiment: even fact-pruning
// drivers report its unchecked dereference in downstream, proving that direct-import facts still
// flow while the two-hop fact about upstream's S.Get does not.
func NilableFunc() *int {
	return nil
}
