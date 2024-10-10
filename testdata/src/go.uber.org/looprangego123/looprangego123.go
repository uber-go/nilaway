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

// This package aims to test nilability behavior for `range` in loops.

// <nilaway no inference>
package looprange

import (
	"maps"
	"slices"
)

func testIter() {
	i := 42
	for element := range slices.Values([]*int{&i, &i, nil}) {
		print(*element) // FN: we do not really handle iterators for now, the elements from iterators are assumed to be nonnil.
	}
	for k, v := range maps.All(map[string]*int{"abc": &i, "def": nil}) {
		print(k)
		print(*v) // FN: we do not really handle iterators for now, the elements from iterators are assumed to be nonnil.
	}
}
