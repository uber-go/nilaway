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
These tests cover nil checks on the same variable spread across separate statements (issue 378).
Variables that are written between the checks, ranged into, address-taken, or assigned inside a
closure stay reported.
*/
package nilcheck

// nilable(x, y)
func issue378(x, y *int) int {
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

// nilable(x, y)
func minimal(x, y *int) int {
	if x == nil && y == nil {
		return 0
	}
	if x == nil && y != nil {
		return 2
	}
	return *x
}

type guarded struct {
	n int
}

// nilable(y)
func (g *guarded) receiverCheck(y *int) int {
	if g == nil && y == nil {
		return 0
	}
	if g == nil && y != nil {
		return 2
	}
	return g.n
}

// The nil receiver is what makes `g` nilable inside receiverCheck.
func callReceiverCheck() {
	var g *guarded
	_ = g.receiverCheck(nil)
}

// nilable(x, y, z)
func reassignedBetweenChecks(x, y, z *int) int {
	if x == nil {
		return 0
	}
	x = z
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

func takeAddress(p **int) {}

// nilable(x, y)
func addressTaken(x, y *int) int {
	if x == nil {
		return 0
	}
	takeAddress(&x)
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(x, y)
func rangeAssigned(x, y *int, zs []*int) int {
	if x == nil {
		return 0
	}
	for _, x = range zs {
	}
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(x, y, z)
func assignedInClosure(x, y, z *int) int {
	if x == nil {
		return 0
	}
	func() {
		x = z
	}()
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

type counter []int

func (c *counter) reset() {
	*c = nil
}

// nilable(c, y)
func implicitAddressTaken(c counter, y *int) int {
	if c == nil {
		return 0
	}
	c.reset()
	if c == nil {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(x, y)
func unguardedJoin(x, y *int, flag bool) int {
	if flag {
		if x == nil {
			return 0
		}
	}
	if x == nil {
		return *y //want "dereferenced"
	}
	return 1
}

// nilable(x, y)
func insideLoop(x, y *int) int {
	total := 0
	for i := 0; i < 3; i++ {
		if x == nil && y == nil {
			return 0
		}
		if x == nil && y != nil {
			return 2
		}
		total += *x
	}
	return total
}

// nilable(x)
func singleCheckStillReports(x *int) int {
	return *x //want "dereferenced"
}
