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
These tests aim to establish proper handling of the nilness of global variables

<nilaway no inference>
*/
package globalvars

// nilable(nilable)
var nilable *int
var nonnil = new(int)

func readFromGlobals() *int {
	if true {
		return nilable //want "returned"
	} else {
		return nonnil
	}
}

// nilable(a)
func writeToGlobals(a, b *int) {
	switch 0 {
	case 1:
		nilable = a
	case 2:
		nilable = b
	case 3:
		nonnil = a //want "assigned"
	default:
		nonnil = b
	}
}

// nilable(deepnilable[]), nonnil(deepnilable)
var deepnilable []*int //want "assigned into the global variable"

// nonnil(deepnonnil)
var deepnonnil []*int //want "assigned into the global variable"

func readDeepFromGlobals() *int {
	if true {
		return deepnilable[0] //want "returned"
	} else {
		return deepnonnil[0]
	}
}

// nilable(a)
func writeDeepToGlobals(a, b *int) {
	switch 0 {
	case 1:
		deepnilable[0] = a
	case 2:
		deepnilable[0] = b
	case 3:
		deepnonnil[0] = a //want "assigned"
	default:
		deepnonnil[0] = b
	}
}
