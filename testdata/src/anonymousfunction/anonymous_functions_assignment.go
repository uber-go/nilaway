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
package anonymousfunction

func testSimpleAssignment() {
	var t1 *int
	a := func() {
		print(*t1) //want "unassigned variable `t1`"
	}
	a() // at this call t1 is nil

	var t2 *int
	b := func() {
		print(*t1)
		print(*t2) //want "function parameter `t2` dereferenced"
	}

	i := 1
	t1 = &i
	t2 = nil

	b() // at this call t1 is not nil but t2 is nil
}

func testAssignOneToOneAssingments() {
	var t1 *int
	a, b := func(t *int) {
		print(*t) //want "unassigned variable `t1`"
	}, func(t2 *int) {
		print(*t2) //want "unassigned variable `t1`"
	}
	a(t1)
	b(t1)

}

func testNestedAnonymousFuncAssignment() {
	var t1 *int
	var t2 *int
	a := func() {
		print(*t1) //want "unassigned variable `t1`"
		var t2 *int
		print(*t2) //want "unassigned variable `t2`"
		b := func() {
			print(*t1) //want "unassigned variable `t1`"
			print(*t2) //want "unassigned variable `t2`"
			if t2 != nil {
				print(*t2) // this is ok
			}
		}
		b() // at this call t1 and t2 are both nil
	}
	i := 1
	t2 = &i
	print(*t2)
	a() // at this call t1 is nil
}

func testImplicitAndExplicitArguments() {
	var t1 *int
	var t2 *int
	f := func(t *int) {
		print(*t) //want "unassigned variable `t2`"
		// the following erros are for t3 and p2
		g := func(p *int) {
			print(*t)
			print(*p)  //want "unassigned variable `t1`"
			print(*t2) //want "unassigned variable `t2`"
		}
		i := 1
		t = &i
		g(t1)
	}

	f(t2)
}

func testShadowedClosureVariable() {
	var a *int
	i := 1
	b := &i

	func() {
		print(*a) //want "unassigned variable `a`"
		func(a *int) {
			print(*a) // this is actually ok since `a` is shadowed and we are passing a nonnil argument at the call site.
		}(b)
	}()
}

// nonnil(i)
func testWithNamedReturn(i *int) (r *int) {
	print(*i) // safe
	r = i
	k := func() {
		print(*r) // safe
	}
	k()
	return
}
