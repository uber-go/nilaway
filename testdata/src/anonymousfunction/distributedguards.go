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

// Nil checks spread across separate statements inside function literals. Parameters of the literal
// are stable, captured variables are not, because a sibling closure can write them between checks.
package anonymousfunction

func funcLitParams() {
	f := func(x, y *int) int {
		if x == nil && y == nil {
			return 0
		}
		if x == nil && y != nil {
			return 2
		}
		return *x
	}
	f(nil, nil)
}

func funcLitCapture() {
	var x, y *int
	reset := func() {
		x = nil
	}
	func() {
		if x == nil {
			return
		}
		reset()
		if x == nil {
			print(*y) //want "unassigned variable `y`"
		}
	}()
}

// nilable(result 0)
func nilableInt() *int {
	return nil
}

func funcLitLocals() {
	func() int {
		x := nilableInt()
		y := nilableInt()
		if x == nil && y == nil {
			return 0
		}
		if x == nil && y != nil {
			return 2
		}
		return *x
	}()
}
