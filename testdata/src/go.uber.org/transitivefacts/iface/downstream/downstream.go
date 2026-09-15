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

// Package downstream implements upstream.Handler without importing upstream: the interface is
// only ever named by middle, which it does import. That Handle is called with a nil argument was
// established two hops away, in upstream, so the dereference below is only flagged if that fact
// is forwarded through middle.
package downstream

import "go.uber.org/transitivefacts/iface/middle"

type derefHandler struct{}

func (derefHandler) Handle(x *int) {
	print(*x) //want "function parameter `x` dereferenced"
}

func useInterface() {
	middle.Invoke(derefHandler{})
}
