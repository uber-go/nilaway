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

// Package middle re-exposes upstream.Handler in parameter position only, and says nothing itself
// about the nilability of Handle's parameter.
package middle

import "go.uber.org/transitivefacts/iface/upstream"

// Invoke takes an upstream.Handler, making the interface implementable by packages that do not
// import upstream.
func Invoke(h upstream.Handler) {
	upstream.Run(h)
}
