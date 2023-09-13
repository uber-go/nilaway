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

// This package aims to test nilability behavior for simple cases in anonymous functions.
// <nilaway anonymous function enable>
package anonymousfunction

func testPassingArgsExplicityToAnonFuncs() {
	// Now we test uses of variables in the closure in anonymous functions. This is a simple test
	// such that all used variables are explicitly passed as params in to the anonymous functions.

	// Passing nilable variables as parameters.
	var t1 *int
	func(t *int) {
		print(*t) //want "unassigned variable `t1`"
	}(t1)

	func(t *int) {
		if t != nil {
			print(*t)
		}
	}(t1)

	// Pass a nonnil struct as parameter
	t2 := A{}
	func(t *A) *int {
		return t.a
	}(&t2)

	// Pass nilable struct as parameter.
	var t3 *A
	func(t *A) *int {
		return t.a //want "unassigned variable `t3`"
	}(t3)

	// test nested anonymous function

	var t4 *int
	func(t *int) {
		var t2 *int
		func(n1 *int, n2 *int) {
			print(*n1) //want "unassigned variable `t4`"
			print(*n2) //want "unassigned variable `t2`"
		}(t, t2)
	}(t4)

	c := 1
	t5 := &c // t5 is nonnil now
	func(t *int) {
		print(*t)
		var t2 *int
		// the following error is coming from n2 but not from n1
		func(n1 *int, n2 *int) {
			print(*n1) // this should be ok
			print(*n2) //want "unassigned variable `t2`"
			if n2 != nil {
				print(*n2) // this is ok
			}
		}(t, t2)
	}(t5)
}

type B struct{}

func (b *B) testOverWriteReceiver() {
	b = nil
	func() {
		print(*b) //want "literal `nil`"
	}()
}

func (b *B) testPassReceiver() {
	func(nb *B) {
		print(*nb) //want "unassigned variable `b`"
	}(b)
}

func (b *B) testGuardReceiver() {
	if b == nil {
		return
	}
	func(nb *B) {
		print(*nb) // this is ok
	}(b)
}

func nilReceiverTestDriver() {
	// This calls B's methods with a nil receiver to test handling receivers in closure.
	var b *B
	b.testPassReceiver()
	b.testGuardReceiver()
}

func (a *A) testNonNilReceiver() {
	func(na *A) {
		print(*na) // this is also ok
	}(&A{})
}
