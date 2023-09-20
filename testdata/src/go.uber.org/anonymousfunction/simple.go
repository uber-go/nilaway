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
	a *int
	b *A
	c *A
	d *int
}

func retNilable() *int {
	return nil
}

func simple() {
	// Here we test nilability analysis _inside_ the anonymous functions, where no interactions
	// happen between the anonymous functions and the outside world.
	aNonnilPtr := &A{}
	// ERROR_GROUP: the two errors reporting dereference of `aNonnilPtr.a` are grouped together and reported on the below line.
	print(*(aNonnilPtr.a)) //want "it is annotated"

	func() {
		var t *int
		print(*t) //want "unassigned variable `t`"
		t2 := 1
		ptr := &t2
		print(*ptr)
		t3 := retNilable()
		print(*t3) //want "result 0 of `retNilable.*` dereferenced"
		var aPtr *A
		print(*aPtr) //want "unassigned variable `aPtr`"
		aNonnilPtr := &A{}
		print(*(aNonnilPtr.a)) // (error here is grouped with the error at line marked with `ERROR_GROUP`)
		// A.c is marked as nonnil, so it is ok to dereference.
		print(aNonnilPtr.c.a)
	}()

	a := func() {
		var t *int
		print(*t) //want "unassigned variable `t`"
	}
	print(a)

	var f func() = func() {
		var t *int
		print(*t) //want "unassigned variable `t`"
	}

	f()
}

// test nested anonymous functions
func nestedFunc() {
	func() {
		func() {
			var t *int
			print(*t) //want "unassigned variable `t`"
		}()
		a := func() {
			var t *int
			print(*t) //want "unassigned variable `t`"
		}
		print(a)
	}()

	func() {
		func() {
			func() {
				var t *int
				print(*t) //want "unassigned variable `t`"
			}()
		}()
	}()
}

var a = func() {
	var t *int
	print(*t) //want "unassigned variable `t`"
}
