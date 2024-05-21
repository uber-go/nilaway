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

// <nilaway anonymous function enable>
package anonymousfunc

// For testing purposes, we will add a comment at the same line of each anonymous function
// declaration so that we know for each *ast.FuncLit node what is the expected set of
// variables from closure. In the analyzer we used ast.Inspect - a depth-first search - over each
// function literal to collect the closure. Therefore, in the test file, we need to specify the
// expected variables in the depth-first order in the comments. If a func lit node does not use any
// closure variables, either write no comments for it or write nothing after "expect_closure".

type A struct {
	a *A
	b *int
}

func (a *A) foo() {
	func() { // expect_closure: a
		c := a.b
		print(*c)
	}()
}

func noClosure() {
	// For function literals that do not use closure variables, either write no comments or leave
	// the list of closure variables empty after "expect_closure".
	func() { // expect_closure:
		func() { // expect_closure:
			print("test")
		}()

		var i *int
		func() { // expect_closure: i
			print(*i)
		}()
	}()
}

func test() {
	a := &A{}
	func() { // expect_closure: a
		print(a.a.b) // we should only collect the first 'a', but not the second `a` and `b`
	}()
}

func test2() {
	a := 1
	func() { // expect_closure: a
		print(a) // a is from closure
		a := 2
		print(a) // a is not from closure
	}()
}

func testd() {
	i := 1
	a := &i       // This must be nonnil
	k := func() { // expect_closure: a
		print(*a)
		print(*a)
	}

	func() { // expect_closure: a
		print(*a)
		j := 0
		a = &j
		print(*a)
	}()

	k()
	// We reset the ptr a to be nilable.
	a = nil
	// Now we call the function.
	k()

	a = &i // This must be nonnil

	b := &i
	j := func(a *int) { // expect_closure: b
		// We reset the ptr a to be nilable.
		a = nil
		print(*b)
	}

	// Now we call the function.
	j(a)

	print(*a) // Now, we dereference a

	func() { // expect_closure: a b
		print(*a)
		print(*b)
	}()

	func() { // expect_closure: i a b
		c := &i
		func() { // expect_closure: a b c
			print(*a)
			print(*b)
			print(*c)
		}()
		print(*c)
	}()

	func() { // expect_closure: i a
		func() { // expect_closure: i a
			c := &i
			print(*a)
			print(*c)
		}()
	}()

	return
}
