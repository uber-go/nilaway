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

/*
This file tests the contract of "ok" form for user defined functions.

<nilaway no inference>
*/
package contracts

func test() {
	m := map[*int]*int{}
	if v, ok := m[new(int)]; ok {
		print(*v) // no error reported because NilAway handles "OK" form for a map.
	}
	if v, ok := foo(new(int)); ok {
		print(*v) // However, this line still throws a false positive because NilAway does not handle "OK" form for a user-defined function.
	}
}

// nilable(result 0)
func foo(x *int) (*int, bool) {
	if x == nil {
		return nil, false
	}
	return new(int), true
}
