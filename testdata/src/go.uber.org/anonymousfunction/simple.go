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

// We currently do not parse annotations for anonymous functions, and there is no simple way to
// do that, so we rely on the inference engine in the tests. A few tricks are used to make
// sure inference engine will correctly infer a site as nilable or nonnil.
// (1) nilable: we will use an uninitialized variable, which will definitely be inferred as
//     nilable, to enforce nilable values.
// (2) nonnil: we will use a simple dereference `print(*ptr)` to enforce nonnil values.

// nilable(a, b)
// nonnil(c)
// nonnil(d)
type A struct {
	// ERROR_GROUP: two nilable flows in the program, so NilAway will be reporting a grouped error message with two consumption sites for the following line
	a *int //want "value read from the field `a`"
	b *A
	c *A
	d *int
}

func retNilable() *int { // want "returned from the function `retNilable` in position 0"
	return nil
}

func simple() {
	// Here we test nilability analysis _inside_ the anonymous functions, where no interactions
	// happen between the anonymous functions and the outside world.
	aNonnilPtr := &A{}
	print(*(aNonnilPtr.a)) // ERROR_GROUP: consumption site 1: reported at A.a declaration

	func() {
		var t *int
		print(*t) // want "Value read from a variable that was never assigned"
		t2 := 1
		ptr := &t2
		print(*ptr)
		t3 := retNilable()
		print(*t3) // (this deref is reported at retNilable declaration)
		var aPtr *A
		print(*aPtr) // want "Value read from a variable that was never assigned"
		aNonnilPtr := &A{}
		print(*(aNonnilPtr.a)) // ERROR_GROUP: consumption site 2: reported at A.a declaration
		// A.c is marked as nonnil, so it is ok to dereference.
		print(aNonnilPtr.c.a)
	}()

	a := func() {
		var t *int
		print(*t) // want "Value read from a variable that was never assigned"
	}
	print(a)

	var f func() = func() {
		var t *int
		print(*t) // want "Value read from a variable that was never assigned"
	}

	f()
}

// test nested anonymous functions
func nestedFunc() {
	func() {
		func() {
			var t *int
			print(*t) // want "Value read from a variable that was never assigned"
		}()
		a := func() {
			var t *int
			print(*t) // want "Value read from a variable that was never assigned"
		}
		print(a)
	}()

	func() {
		func() {
			func() {
				var t *int
				print(*t) // want "Value read from a variable that was never assigned"
			}()
		}()
	}()
}

var a = func() {
	var t *int
	print(*t) // want "Value read from a variable that was never assigned"
}
