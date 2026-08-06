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
*/
package contracts

var dummy bool

func simpleCond(m map[any]*int) *int {
	var v *int
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

func testVarDecl(m map[any]*int) *int {
	var v, ok = m[0]
	if !ok {
		panic(0)
	}
	return v
}

func threeWay(m map[any]*int) *int {
	var v *int
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

func overridesOk1(m map[any]*int) *int {
	v, ok := m[0]

	if dummy {
		v, ok = m[1]
	}

	if !ok {
		panic(0)
	}

	return v
}

func overridesOk2(m map[any]*int) *int {
	v, ok := m[0]

	if dummy {
		v = new(int)
	}

	if !ok {
		panic(0)
	}

	return v
}

func overridesNotOk1(m map[any]*int) *int {
	v, ok := m[0]

	if dummy {
		ok = true
	}

	if !ok {
		panic(0)
	}

	return v
}

func overridesNotOk2(m map[any]*int) *int {
	v, ok := m[0]

	if dummy {
		v = nil
	}

	if !ok {
		panic(0)
	}

	return v
}

func threeWayOneConcrete(m map[any]*int) *int {
	var v *int
	var ok bool
	if dummy {
		if dummy {
			v, ok = m[1]
		} else {
			ok = false
			v = new(int)
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

// badMergeCaller consumes badMerge's result with a real dereference, demanding it nonnil under
// inference. It is declared *before* badMerge: this makes each unsafe return site in badMerge
// report its own nil flow at the dereference below (declaring the consumer after the producer
// would collapse them into a single grouped diagnostic).
func badMergeCaller(m map[any]*int) {
	print(*badMerge(m)) //want "returned from `badMerge.*` in position 0" "returned from `badMerge.*` in position 0" "returned from `badMerge.*` in position 0" "returned from `badMerge.*` in position 0"
}

func badMerge(m map[any]*int) *int {
	var v *int
	var ok1 bool
	var ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[1]
	}

	switch getInt() {
	case getInt():
		return v
	case getInt():
		if ok1 {
			return v
		}
	case getInt():
		if ok2 {
			return v
		}
	case getInt():
		if ok1 && ok2 {
			return v
		}
	case getInt():
		if ok1 || ok2 {
			return v
		}
	}
	return new(int)
}

func testCheckInNeitherThenNeitherParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckInNeitherThenLeftParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
	}
	if !ok1 {
		return new(int)
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckInNeitherThenRightParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
	}
	if !ok2 {
		return new(int)
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckInNeitherThenBothParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
	}
	if !ok1 || !ok2 {
		return new(int)
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckOnlyInLeftThenNeitherParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	} else {
		v, ok2 = m[0]
	}
	func(any) {}(ok2)
	return v
}

func testCheckOnlyInLeftThenLeftParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	} else {
		v, ok2 = m[0]
	}
	if !ok1 {
		return new(int)
	}
	func(any) {}(ok2)
	return v
}

func testCheckOnlyInLeftThenRightParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	} else {
		v, ok2 = m[0]
	}
	if !ok2 {
		return new(int)
	}
	return v
}

func testCheckOnlyInLeftThenBothParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	} else {
		v, ok2 = m[0]
	}
	if !ok1 || !ok2 {
		return new(int)
	}
	return v
}

func testCheckOnlyInRightThenNeitherParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	func(any) {}(ok1)
	return v
}

func testCheckOnlyInRightThenLeftParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	if !ok1 {
		return new(int)
	}
	return v
}

func testCheckOnlyInRightThenRightParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	if !ok2 {
		return new(int)
	}
	func(any) {}(ok1)
	return v
}

func testCheckOnlyInRightThenBothParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	if !ok1 || !ok2 {
		return new(int)
	}
	return v
}

func testCheckInBothParallel(m map[any]*int) *int {
	var v *int
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	} else {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	return v
}

func testCheckInNeitherThenNeitherSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckInNeitherThenLeftSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok1 {
		return new(int)
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckInNeitherThenRightSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok2 {
		return new(int)
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckInNeitherThenBothSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok1 || !ok2 {
		return new(int)
	}
	func(any, any) {}(ok1, ok2)
	return v
}

func testCheckOnlyInLeftThenNeitherSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	func(any) {}(ok2)
	return v
}

func testCheckOnlyInLeftThenLeftSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok1 {
		return new(int)
	}
	func(any) {}(ok2)
	return v
}

func testCheckOnlyInLeftThenRightSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok2 {
		return new(int)
	}
	return v
}

func testCheckOnlyInLeftThenBothSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	}
	if dummy2 {
		v, ok2 = m[0]
	}
	if !ok1 || !ok2 {
		return new(int)
	}
	return v
}

func testCheckOnlyInRightThenNeitherSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	func(any) {}(ok1)
	return v
}

func testCheckOnlyInRightThenLeftSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	if !ok1 {
		return new(int)
	}
	return v
}

func testCheckOnlyInRightThenRightSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	if !ok2 {
		return new(int)
	}
	func(any) {}(ok1)
	return v
}

func testCheckOnlyInRightThenBothSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	if !ok1 || !ok2 {
		return new(int)
	}
	return v
}

func testCheckInBothSeries(m map[any]*int) *int {
	v := new(int)
	var ok1, ok2 bool
	if dummy {
		v, ok1 = m[0]
		if !ok1 {
			return new(int)
		}
	}
	if dummy2 {
		v, ok2 = m[0]
		if !ok2 {
			return new(int)
		}
	}
	return v
}

// The following callers consume the results of the functions above with real dereferences. This
// makes the result of each function above demanded as nonnil under inference, such that the
// functions with insufficient "ok" guarding report errors (at the dereference sites below), while
// the correctly guarded functions must stay silent (negative coverage).
func callAndDerefResults(m map[any]*int) {
	print(*simpleCond(m))
	print(*testVarDecl(m))
	print(*threeWay(m))
	print(*overridesOk1(m))
	print(*overridesOk2(m))
	print(*overridesNotOk1(m)) //want "returned from `overridesNotOk1.*` in position 0"
	print(*overridesNotOk2(m)) //want "returned from `overridesNotOk2.*` in position 0"
	print(*threeWayOneConcrete(m))

	print(*testCheckInNeitherThenNeitherParallel(m)) //want "returned from `testCheckInNeitherThenNeitherParallel.*` in position 0"
	print(*testCheckInNeitherThenLeftParallel(m))    //want "returned from `testCheckInNeitherThenLeftParallel.*` in position 0"
	print(*testCheckInNeitherThenRightParallel(m))   //want "returned from `testCheckInNeitherThenRightParallel.*` in position 0"
	print(*testCheckInNeitherThenBothParallel(m))
	print(*testCheckOnlyInLeftThenNeitherParallel(m)) //want "returned from `testCheckOnlyInLeftThenNeitherParallel.*` in position 0"
	print(*testCheckOnlyInLeftThenLeftParallel(m))    //want "returned from `testCheckOnlyInLeftThenLeftParallel.*` in position 0"
	print(*testCheckOnlyInLeftThenRightParallel(m))
	print(*testCheckOnlyInLeftThenBothParallel(m))
	print(*testCheckOnlyInRightThenNeitherParallel(m)) //want "returned from `testCheckOnlyInRightThenNeitherParallel.*` in position 0"
	print(*testCheckOnlyInRightThenLeftParallel(m))
	print(*testCheckOnlyInRightThenRightParallel(m)) //want "returned from `testCheckOnlyInRightThenRightParallel.*` in position 0"
	print(*testCheckOnlyInRightThenBothParallel(m))
	print(*testCheckInBothParallel(m))

	print(*testCheckInNeitherThenNeitherSeries(m)) //want "returned from `testCheckInNeitherThenNeitherSeries.*` in position 0"
	print(*testCheckInNeitherThenLeftSeries(m))    //want "returned from `testCheckInNeitherThenLeftSeries.*` in position 0"
	print(*testCheckInNeitherThenRightSeries(m))   //want "returned from `testCheckInNeitherThenRightSeries.*` in position 0"
	print(*testCheckInNeitherThenBothSeries(m))
	print(*testCheckOnlyInLeftThenNeitherSeries(m)) //want "returned from `testCheckOnlyInLeftThenNeitherSeries.*` in position 0"
	print(*testCheckOnlyInLeftThenLeftSeries(m))    //want "returned from `testCheckOnlyInLeftThenLeftSeries.*` in position 0"
	print(*testCheckOnlyInLeftThenRightSeries(m))
	print(*testCheckOnlyInLeftThenBothSeries(m))
	print(*testCheckOnlyInRightThenNeitherSeries(m)) //want "returned from `testCheckOnlyInRightThenNeitherSeries.*` in position 0"
	print(*testCheckOnlyInRightThenLeftSeries(m))
	print(*testCheckOnlyInRightThenRightSeries(m)) //want "returned from `testCheckOnlyInRightThenRightSeries.*` in position 0"
	print(*testCheckOnlyInRightThenBothSeries(m))
	print(*testCheckInBothSeries(m))
}

// Now, we add a test for a FP case, which should be handled when we have user-defined contracts
// in NilAway .
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
