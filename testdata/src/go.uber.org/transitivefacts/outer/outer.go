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

// Package outer is the third hop of the transitive-facts chain (upstream <- middle <- outer <-
// deepdownstream). It imports ONLY middle, so upstream's fact reaches it by forwarding rather
// than directly; re-exposing middle.Wrapper keeps upstream.S reachable through its own API, so
// outer must forward a fact it itself only received by forwarding.
package outer

import "go.uber.org/transitivefacts/middle"

// MakeWrapper re-exposes middle.Wrapper, and with it the upstream.S reachable through its
// exported field.
func MakeWrapper() *middle.Wrapper {
	return middle.NewWrapper()
}
