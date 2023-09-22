//  Copyright (c) 2023 Uber Technologies, Inc.
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

// This package aims to test deep nilability in the inference mode.

package inference

func testLocalDeepAssignNil(i int) {
	m := make(map[int]*string)
	sl := make([]*int, 1)

	switch i {
	case 0:
		m[0] = nil
		if v, ok := m[0]; ok {
			_ = *v //want "literal `nil` dereferenced"
		}

	case 1:
		m[0] = nil
		if v, ok := m[0]; ok && v != nil {
			_ = *v
		}

	case 2:
		m[i] = nil
		if v, ok := m[i]; ok {
			_ = *v //want "deep read from local variable `m` dereferenced"
		}

	case 3:
		m[i] = nil
		if v, ok := m[i]; ok && v != nil {
			_ = *v
		}

	case 4:
		sl[0] = nil
		_ = *sl[0] //want "literal `nil` dereferenced"
	}
}
