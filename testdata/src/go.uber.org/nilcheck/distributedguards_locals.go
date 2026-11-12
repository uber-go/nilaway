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

/*
These tests cover nil checks spread across separate statements (issue 378) on local variables.
A write between the checks resets what an earlier check established, so the dereference after
the write stays reported.
*/
package nilcheck

// nilable(result 0)
func nilableInt() *int {
	return nil
}

// nilable(result 0, result 1)
func twoNilableInts() (*int, *int) {
	return nil, nil
}

func localsSupported() int {
	x := nilableInt()
	y := nilableInt()
	if x == nil && y == nil {
		return 0
	}
	if x != nil && y == nil {
		return 1
	}
	if x == nil && y != nil {
		return 2
	}
	return *x
}

func varDeclared() int {
	var x, y = nilableInt(), nilableInt()
	if x == nil && y == nil {
		return 0
	}
	if x == nil && y != nil {
		return 2
	}
	return *x
}

// nilable(y)
func writeBetweenChecks(y *int) int {
	x := nilableInt()
	if x == nil {
		return 0
	}
	x = nilableInt()
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(y)
func writeInLoop(y *int) int {
	x := nilableInt()
	if x == nil {
		return 0
	}
	total := 0
	for i := 0; i < 3; i++ {
		if x == nil {
			total += *y //want "dereferenced"
		}
		x = nilableInt()
	}
	return total
}

// nilable(y)
func tupleWriteBetweenChecks(y *int) int {
	x, z := twoNilableInts()
	if x == nil || z == nil {
		return 0
	}
	x, z = twoNilableInts()
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(y)
func mapReadBetweenChecks(m map[int]*int, y *int) int {
	x := nilableInt()
	if x == nil {
		return 0
	}
	x, ok := m[0]
	if x == nil && ok {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(y)
func channelReceiveBetweenChecks(ch chan *int, y *int) int {
	x := nilableInt()
	if x == nil {
		return 0
	}
	x, ok := <-ch
	if x == nil && ok {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(xs[], y)
func rangedLocal(xs []*int, y *int) int {
	x := nilableInt()
	if x == nil {
		return 0
	}
	for _, x = range xs {
	}
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(y)
func addressTakenLocal(y *int) int {
	x := nilableInt()
	if x == nil {
		return 0
	}
	takeAddress(&x)
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(y)
func closureWrittenLocal(y *int) int {
	x := nilableInt()
	if x == nil {
		return 0
	}
	func() { x = nilableInt() }()
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}
