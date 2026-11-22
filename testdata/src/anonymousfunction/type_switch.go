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
package anonymousfunction

func typeSwitchWalk(f func(any) bool) { f(nil) }

// typeSwitchInFuncLit is the reproducer from issue #531: a type switch with a binding inside a
// function literal. The binding `n` in the switch header does not denote a variable by itself
// (the type checker declares a separate implicit variable for each case clause instead), which
// used to make the closure collection panic and hide every other diagnostic in the package.
func typeSwitchInFuncLit() {
	typeSwitchWalk(func(v any) bool {
		switch n := v.(type) {
		case string:
			_ = len(n)
		}

		return true
	})
}

type typeSwitchStringer interface{ String() string }

// typeSwitchBindingInFuncLit checks that the binding is analyzed as a local variable of the
// function literal, not as a closure variable: in the default arm it keeps the static type of the
// operand and can be nil.
func typeSwitchBindingInFuncLit(v typeSwitchStringer) {
	func() {
		switch x := v.(type) {
		case error:
			println(x.Error())
		default:
			println(x.String()) //want "called `String"
		}
	}()
}

func useTypeSwitchBindingInFuncLit() {
	typeSwitchBindingInFuncLit(nil)
}

// typeSwitchNilFlowFromClosure checks that nil flows through closure variables are still tracked
// inside a function literal that contains a type switch with a binding.
func typeSwitchNilFlowFromClosure(v any) {
	var t *int
	func() {
		switch n := v.(type) {
		case string:
			_ = len(n)
			print(*t) //want "unassigned variable `t`"
		}
	}()
}
