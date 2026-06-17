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

package trustedfunc

import "encoding/json"

// testJSONUnmarshal exercises the `ErrorReturnNonnilArgs` hook: `json.Unmarshal(data, &v)` populates
// `v`, so the pointee is treated as non-nil once the error return is checked to be nil.
func testJSONUnmarshal(data []byte) {
	// `err != nil` early return: pointee is non-nil on the fallthrough (error-is-nil) path.
	var v1 *int
	if err := json.Unmarshal(data, &v1); err != nil {
		return
	}
	print(*v1) // safe

	// `err == nil` positive check: pointee is non-nil inside the block.
	var v2 *int
	err := json.Unmarshal(data, &v2)
	if err == nil {
		print(*v2) // safe
	}

	// Error return not checked at all: no guarantee.
	var v3 *int
	json.Unmarshal(data, &v3)
	print(*v3) //want "unassigned variable `v3` dereferenced"

	// Error return discarded into the blank identifier: no guarantee.
	var v4 *int
	_ = json.Unmarshal(data, &v4)
	print(*v4) //want "unassigned variable `v4` dereferenced"

	// Dereference on the error path (`err != nil`): pointee is not guarded here.
	var v5 *int
	if err := json.Unmarshal(data, &v5); err != nil {
		print(*v5) //want "unassigned variable `v5` dereferenced"
	}
}
