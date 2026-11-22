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

// This package aims to test that a type switch guard with a binding, such as `switch n := v.(type)`,
// inside an anonymous function is analyzed like any other code. go/types records no object for the
// guard identifier itself (each clause gets its own implicit object), which used to make the
// closure collection panic.
package anonymousfunction

func walk(f func(any) bool) {
	f(nil)
}

func typeSwitchInCallback() {
	walk(func(v any) bool {
		var t *int
		switch n := v.(type) {
		case *int:
			print(*t) //want "unassigned variable `t`"
			print(n)
		case string:
			print(len(n))
		}
		return true
	})
}

func typeSwitchOnCapturedVar(v any) {
	func() {
		var t *int
		switch n := v.(type) {
		case *int:
			print(*t) //want "unassigned variable `t`"
			print(n)
		default:
			print(n)
		}
	}()
}

// nestedBlockDecl tests a nil variable declared in a nested block of a function literal that is
// passed as a callback. Such a variable belongs to the literal and must not be treated as a closure
// variable, otherwise its nil assignment is not tracked.
func nestedBlockDecl() {
	walk(func(v any) bool {
		if v == nil {
			var t *int
			print(*t) //want "unassigned variable `t`"
		}
		return true
	})
}
