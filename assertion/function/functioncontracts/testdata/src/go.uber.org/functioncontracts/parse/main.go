// want package:".*"

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

// <nilaway contract enable>
package parse

// contract(nonnil -> nonnil)
func f1(x *int) *int {
	if x == nil {
		return x
	}
	return new(int)
}

// contract(nonnil -> true)
func f2(x *int) bool {
	if x == nil {
		return false
	}
	return true
}

// contract(nonnil -> false)
func f3(x *int) bool {
	if x == nil {
		return true
	}
	return false
}

// contract(_, nonnil -> nonnil, true)
func multipleValues(key string, deft *int) (*int, bool) {
	m := map[string]*int{}
	x, _ := m[key]
	if x != nil {
		return x, true
	}
	if deft != nil {
		return deft, true
	}
	return nil, false
}

// contract(_, nonnil -> nonnil, true)
// contract(nonnil, _ -> nonnil, true)
func multipleContracts(x *int, y *int) (*int, bool) {
	if x == nil && y == nil {
		return nil, false
	}
	return new(int), true
}

// This contract `// contract(nonnil -> nonnil)` does not hold for the function because the
// function has no param or return. Only a contract in its own line should be parsed, not even `//
// contract(nonnil -> nonnil)`.
func contractCommentInOtherLine() {}

// contract(nonnil -> nonnil)
func ExportedFromParse(x *int) *int {
	if x != nil {
		return new(int)
	}
	return nil
}
