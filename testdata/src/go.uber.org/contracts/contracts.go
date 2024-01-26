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
This package aims to test some behavior of contracts - other tests for specific behavior appears
in other packages such as `maps` and `erroreturn`.

<nilaway no inference>
*/
package contracts

var dummy bool

func simpleCond(m map[any]any) any {
	var v any
	var ok bool
	if dummy {
		v, ok = m[0]
	} else {
		v, ok = m[1]
	}

	if !ok {
		panic(0)
	}

	return v
}

func testVarDecl(m map[any]any) any {
	var v, ok = m[0]
	if !ok {
		panic(0)
	}
	return v
}

func threeWay(m map[any]any) any {
	var v any
	var ok bool
	if dummy {
		if dummy {
			v, ok = m[1]
		} else {
			v, ok = m[2]
		}
	} else {
		v, ok = m[0]
	}

	if !ok {
		panic(0)
	}

	return v
}

func overridesOk1(m map[any]any) any {
	v, ok := m[0]

	if dummy {
		v, ok = m[1]
	}

	if !ok {
		panic(0)
	}

	return v
}

func overridesOk2(m map[any]any) any {
	v, ok := m[0]

	if dummy {
		v = 0
	}

	if !ok {
		panic(0)
	}

	return v
}

func overridesNotOk1(m map[any]any) any {
	v, ok := m[0]

	if dummy {
		ok = true
	}

	if !ok {
		panic(0)
	}

	return v //want "returned"
}

func overridesNotOk2(m map[any]any) any {
	v, ok := m[0]

	if dummy {
		v = nil
	}

	if !ok {
		panic(0)
	}

	return v //want "returned"
}

func threeWayOneConcrete(m map[any]any) any {
	var v any
	var ok bool
	if dummy {
		if dummy {
			v, ok = m[1]
		} else {
			ok = false
			v = 0
		}
	} else {
		v, ok = m[0]
	}

	if !ok {
		panic(0)
	}

	return v
}

var getInt func() int

var dummy2 bool

func badMerge(m map[any]any) any {
	var v any
	var ok1 bool
	var ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[1]
	}

	switch getInt() {
	case getInt():
		return v //want "returned"
	case getInt():
		if ok1 {
			return v //want "returned"
		}
	case getInt():
		if ok2 {
			return v //want "returned"
		}
	case getInt():
		if ok1 && ok2 {
			return v
		}
	case getInt():
		if ok1 || ok2 {
			return v //want "returned"
		}
	}
	return 0
}

func testCheckInNeitherThenNeitherParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
	}
	func(any, any) {}(ok1, ok2)
	return v //want "returned"
}

func testCheckInNeitherThenLeftParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
	}
	if !ok1 {
		return 0
	}
	func(any, any) {}(ok1, ok2)
	return v //want "returned"
}

func testCheckInNeitherThenRightParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
	}
	if !ok2 {
		return 0
	}
	func(any, any) {}(ok1, ok2)
	return v //want "returned"
}

func testCheckInNeitherThenBothParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
	}
	if !ok1 || !ok2 {
		return 0
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckOnlyInLeftThenNeitherParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	} else {
		v, ok2 = m[0]
	}
	func(any) {}(ok2)
	return v //want "returned"
}

func testCheckOnlyInLeftThenLeftParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	} else {
		v, ok2 = m[0]
	}
	if !ok1 {
		return 0
	}
	func(any) {}(ok2)
	return v //want "returned"
}

func testCheckOnlyInLeftThenRightParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	} else {
		v, ok2 = m[0]
	}
	if !ok2 {
		return 0
	}
	return v
}

func testCheckOnlyInLeftThenBothParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	} else {
		v, ok2 = m[0]
	}
	if !ok1 || !ok2 {
		return 0
	}
	return v
}

func testCheckOnlyInRightThenNeitherParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	func(any) {}(ok1)
	return v //want "returned"
}

func testCheckOnlyInRightThenLeftParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	if !ok1 {
		return 0
	}
	return v
}

func testCheckOnlyInRightThenRightParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	if !ok2 {
		return 0
	}
	func(any) {}(ok1)
	return v //want "returned"
}

func testCheckOnlyInRightThenBothParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	if !ok1 || !ok2 {
		return 0
	}
	return v
}

func testCheckInBothParallel(m map[any]any) any {
	var v any
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	return v
}

func testCheckInNeitherThenNeitherSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	func(any, any) {}(ok1, ok2)
	return v //want "returned"
}

func testCheckInNeitherThenLeftSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok1 {
		return 0
	}
	func(any, any) {}(ok1, ok2)
	return v //want "returned"
}

func testCheckInNeitherThenRightSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok2 {
		return 0
	}
	func(any, any) {}(ok1, ok2)
	return v //want "returned"
}

func testCheckInNeitherThenBothSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok1 || !ok2 {
		return 0
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckOnlyInLeftThenNeitherSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	func(any) {}(ok2)
	return v //want "returned"
}

func testCheckOnlyInLeftThenLeftSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok1 {
		return 0
	}
	func(any) {}(ok2)
	return v //want "returned"
}

func testCheckOnlyInLeftThenRightSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok2 {
		return 0
	}
	return v
}

func testCheckOnlyInLeftThenBothSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok1 || !ok2 {
		return 0
	}
	return v
}

func testCheckOnlyInRightThenNeitherSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	func(any) {}(ok1)
	return v //want "returned"
}

func testCheckOnlyInRightThenLeftSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	if !ok1 {
		return 0
	}
	return v
}

func testCheckOnlyInRightThenRightSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	if !ok2 {
		return 0
	}
	func(any) {}(ok1)
	return v //want "returned"
}

func testCheckOnlyInRightThenBothSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	if !ok1 || !ok2 {
		return 0
	}
	return v
}

func testCheckInBothSeries(m map[any]any) any {
	var v any = 0
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return 0
		}
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return 0
		}
	}
	return v
}

// Now, we add a test for a FP case, which should be handled when we have user-defined contracts
// in NilAway .
// nilable(ptr) nilable(result 0)
func imply(ptr *int) *int {
	if ptr == nil {
		return nil
	}
	// Returns a nonil ptr
	a := 1
	return &a
}

func implyCall() {
	var s *int = nil // this is nilable
	if c := imply(s); c != nil {
		// "c != nil" implies "s != nil", but NilAway does not know this and reports the next line
		print(*s) //want "literal `nil` dereferenced"
	}
}
