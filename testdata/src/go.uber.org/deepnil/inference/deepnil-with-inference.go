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

var dummy bool

var globalNil *int = nil

func retNil() *int {
	return nil
}

func retNilSometimes() *int {
	if dummy {
		return nil
	}
	return new(int)
}

func testLocalDeepAssignNil(i int) {
	switch i {
	case 0:
		m := make(map[int]*int)
		m[0] = nil
		if v, ok := m[0]; ok {
			_ = *v //want "literal `nil` dereferenced"
		}
		if m[0] != nil {
			_ = *m[0]
		}

	case 1:
		m := make(map[int]*int)
		m[0] = globalNil
		if v, ok := m[0]; ok && v != nil {
			_ = *v
		} else {
			_ = *v //want "global variable `globalNil` dereferenced"
		}

	case 2:
		m := make(map[int]*int)
		m[i] = nil
		if v, ok := m[i]; ok {
			_ = *v //want "deep read from local variable `m` dereferenced"
		}
		// m[i] is not recognized as a stable expression, hence an error is reported here.
		if m[i] != nil {
			_ = *m[i] //want "deep read from local variable `m` lacking guarding"
		}

	case 3:
		m := make(map[int]*int)
		m[i] = retNilSometimes()
		if v, ok := m[i]; ok && v != nil {
			_ = *v
		} else {
			_ = *v //want "deep read from local variable `m` lacking guarding"
		}

	case 4:
		sl := make([]*int, 1)
		sl[0] = nil
		_ = *sl[0] //want "literal `nil` dereferenced"

		sl[0] = new(int)
		_ = *sl[0]

	case 5:
		sl := make([]*int, 1)
		sl[i] = nil
		_ = *sl[i] //want "deep read from local variable `sl` dereferenced"

	case 6:
		sl := make([]*int, 1)
		sl[0] = retNil()
		_ = *sl[0] //want "result 0 of `ret.*` dereferenced"

	case 7:
		sl := make([]*int, 1)
		sl[i] = retNilSometimes()
		_ = *sl[i] //want "deep read from local variable `sl` dereferenced"

	case 8:
		ch := make(chan *int)
		ch <- nil
		_ = *(<-ch) //want "deep read from local variable `ch` dereferenced"
	}
}
