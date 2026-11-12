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

// <nilaway anonymous function enable>
package anonymousfunc

// For a variadic function literal, the fake parameters for the closure variables are inserted
// right before the variadic parameter, which must remain the last one.

type variadicApp struct{ home string }

func (a *variadicApp) variadicRoots() []string {
	join := func(parts ...string) string { // expect_closure: a
		return a.home + parts[0]
	}
	return []string{join("x")}
}

func testVariadic() {
	i := 1
	a := &i
	s := "s"
	var ps []*int

	func(xs ...*int) { // expect_closure: a
		print(*a, len(xs))
	}(a, a)

	func(n int, xs ...string) { // expect_closure: a s ps
		print(*a, n, s, len(xs), len(ps))
	}(1, "x")

	func(...string) { // expect_closure: a
		print(*a)
	}()

	func(xs ...string) { // expect_closure:
		print(len(xs))
	}()

	func(xs ...*int) { // expect_closure: a
		func() { // expect_closure: a xs
			print(*a, len(xs))
		}()
	}()
}
