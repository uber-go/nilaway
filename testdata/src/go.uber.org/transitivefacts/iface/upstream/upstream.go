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

// Package upstream is the root of a transitive-facts chain in which the fact travels through an
// interface that only ever appears in _parameter_ position downstream of here. Values of an
// interface type cannot be obtained from middle's API, but an importer can still implement it,
// which is why such interfaces stay on the forwarded API surface (see primitivizer.upstreamAPISurface).
package upstream

// Handler is implemented by packages that never import this one, reaching it through middle.
type Handler interface {
	Handle(x *int)
}

// Run establishes, in _this_ package's fact, that Handle's parameter is nilable.
func Run(h Handler) {
	h.Handle(nil)
}
