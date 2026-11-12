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

// Nil checks spread across separate statements on a struct parameter under struct-init-v2. The
// field guards and the stable-variable guards draw nonces from the same generator, so neither pass
// deletes the tags of the other: the parameter dereference is pruned, the unchecked field is not.
package distributedguards

// nilable(p)
type S struct {
	p *int
}

// nilable(s, y)
func readField(s *S, y *int) int {
	if s == nil && y == nil {
		return 0
	}
	if s == nil && y != nil {
		return 2
	}
	return *s.p //want "dereferenced"
}
