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

// The closure variables of a function literal are passed to it as extra arguments. For a variadic
// function literal, they must come before the variadic arguments, so that every argument still
// flows to the right parameter.

type variadicApp struct{ home string }

// variadicRoots is the reproducer from issue #532: a variadic function literal that captures a
// pointer typed variable (here the receiver). This used to make the anonymous function analyzer
// panic and hide every other diagnostic in the package. The literal is now analyzed like a named
// variadic function, which reports indexing the variadic parameter without a length check.
func (a *variadicApp) variadicRoots() []string {
	join := func(parts ...string) string { return a.home + parts[0] } //want "read directly from variadic parameter `parts` sliced into"

	return []string{join("x")}
}

// variadicRootsChecked is the same, with the length check.
func (a *variadicApp) variadicRootsChecked() []string {
	join := func(parts ...string) string {
		if len(parts) == 0 {
			return a.home
		}
		return a.home + parts[0]
	}

	return []string{join(), join("x"), join("x", "y")}
}

func variadicUseRoots() {
	a := &variadicApp{}
	_ = a.variadicRoots()
	_ = a.variadicRootsChecked()
}

// variadicNilClosureVar checks that a nil flow through a closure variable of a variadic function
// literal is reported for every form of call: without variadic arguments, with one, with several,
// and with a slice unpacked by `...`.
func variadicNilClosureVar(xs []string) {
	var a *variadicApp
	noArgs := func(parts ...string) string {
		return a.home //want "unassigned variable `a` passed as arg `a`"
	}
	oneArg := func(parts ...string) string {
		return a.home //want "unassigned variable `a` passed as arg `a`"
	}
	twoArgs := func(parts ...string) string {
		return a.home //want "unassigned variable `a` passed as arg `a`"
	}
	unpacked := func(parts ...string) string {
		return a.home //want "unassigned variable `a` passed as arg `a`"
	}
	func(parts ...string) {
		print(a.home) //want "unassigned variable `a` passed as arg `a`"
	}("x", "y")

	_ = noArgs()
	_ = oneArg("x")
	_ = twoArgs("x", "y")
	_ = unpacked(xs...)
}

// variadicNonnilClosureVar checks the same calls with a nonnil closure variable.
func variadicNonnilClosureVar(xs []string) {
	a := &variadicApp{}
	join := func(parts ...string) string {
		return a.home
	}

	_ = join()
	_ = join("x")
	_ = join("x", "y")
	_ = join(xs...)
}

// variadicArgsWithClosureVar checks that the regular and the variadic arguments still flow to
// their own parameters when the closure variables are passed as well.
func variadicArgsWithClosureVar() {
	a := &variadicApp{}
	i := 1
	f := func(n *int, ps ...*int) {
		print(a.home)
		print(*n) //want "literal `nil` passed as arg `n`"
		for _, p := range ps {
			print(*p) //want "literal `nil` passed as arg `ps`"
		}
	}
	f(nil)
	f(&i, &i, nil)

	g := func(n *int, ps ...*int) {
		print(a.home)
		print(*n)
		for _, p := range ps {
			print(*p)
		}
	}
	g(&i)
	g(&i, &i, &i)
}

// variadicCapturedSlice checks that a captured slice is not mistaken for the variadic parameter.
// A captured slice (or string) used to be accepted as the last parameter of the variadic
// signature, so the analysis did not panic, but treated it as the variadic parameter instead.
func variadicCapturedSlice() {
	i := 1
	ps := []*int{&i}
	f := func(xs ...string) {
		print(*ps[0])
	}
	f("x", "y")

	var nilPs []*int
	g := func(xs ...string) {
		print(*nilPs[0]) //want "unassigned variable `nilPs` passed as arg `nilPs`"
	}
	g("x")
}

type variadicT struct{}

func (variadicT) variadicMethod(ps ...*int) {
	for _, p := range ps {
		print(*p)
	}
}

// variadicMethodExpr checks that the slice unpacked by `...` is also recognized in a call through
// a method expression, where the receiver is not one of the arguments of the method: unpacking a
// nil slice passes no arguments, as in the method call below it.
func variadicMethodExpr() {
	var ps []*int
	variadicT.variadicMethod(variadicT{}, ps...)
	variadicT{}.variadicMethod(ps...)
}
