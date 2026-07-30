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

package slices

// Aliases of slice types must behave like the aliased slice type itself: indexing a nilable
// alias-of-slice value gets the same slice-access check (aliases are materialized as
// *types.Alias since Go 1.23, so they must be explicitly resolved when classifying the operand).
type sliceAlias = []int

var sliceAliasDummy bool

func mkSliceAlias() sliceAlias {
	if sliceAliasDummy {
		return nil
	}
	return []int{1}
}

func testSliceAliasIndex() int {
	s := mkSliceAlias()
	if s != nil {
		return s[0]
	}
	t := mkSliceAlias()
	return t[0] //want "sliced into"
}
