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

// This package tests fixpoint convergence of the backpropagation algorithm.
// The format for specifying expected values is as follows:
// 		expect_fixpoint: <roundCount> <stableRoundCount> <number of triggers>

package backprop

func testSimple() { // expect_fixpoint: 2 1 1
	var x *int
	_ = *x
}

func testEmptyBody() { // expect_fixpoint: 2 1 0
}

func testPanic() { // expect_fixpoint: 1 1 0
	panic("some error")
}

type A struct {
	ptr    *int
	aptr   *A
	newPtr *A
}

// This is a simple test to check the effectiveness of the optimization added via the `struct field analyzer` that enables NilAway to
// only create triggers for those fields of the struct that are being actively assigned (implying a potential side effect) in the function.
// This approach creates fewer number of triggers allowing NilAway to converge quicker without losing precision.
func testStructFieldAnalyzerEffect() { // expect_fixpoint: 4 2 40
	a := &A{}
	for dummy() {
		switch dummy() {
		case dummy():
			a.f1()
		case dummy():
			a.f2()
		case dummy():
			a.f3()
		case dummy():
			a.f4()
		case dummy():
			a.f5()
		case dummy():
			a.f6()
		case dummy():
			a.f7()
		case dummy():
			a.f8()
		case dummy():
			a.f9()
		case dummy():
			a.f10()
		}
	}
}

func (*A) f1()  {}
func (*A) f2()  {}
func (*A) f3()  {}
func (*A) f4()  {}
func (*A) f5()  {}
func (*A) f6()  {}
func (*A) f7()  {}
func (*A) f8()  {}
func (*A) f9()  {}
func (*A) f10() {}

func dummy() bool {
	return true
}

// test assignment in infinite loop

func testInfiniteLoop() { // expect_fixpoint: 3 1 2
	a := &A{}
	for {
		a = a.aptr
	}
}

// test repeated map assignment in a finite loop

type mapType map[string]interface{}

func Get(m mapType, key string) interface{} {
	return m[key]
}

func testAssignmentInLoop(m mapType, key string) { // expect_fixpoint: 6 2 4
	var value interface{}
	value = m
	for len(key) > 0 {
		switch v := value.(type) {
		case mapType:
			value = Get(v, key)
		case map[string]interface{}:
			value = v[key]
		}
	}
}

// test map access with nested non-builtin call expression in the index expression

type MessageBlock struct{}

func (m *MessageBlock) Messages() []*int {
	return []*int{new(int)}
}

func testNonBuiltinNestedIndex(msgSet []*MessageBlock) { // expect_fixpoint: 3 1 4
	for _, msgBlock := range msgSet {
		_ = *msgBlock.Messages()[len(msgBlock.Messages())-1]
	}
}

// test for validating that only the necessary number of triggers are created, and
// no extra triggers (e.g., deep triggers) are created.

func foo(x *int) *int { // expect_fixpoint: 2 1 1
	if x == nil {
		return nil
	}
	return new(int)
}

func testContract() { // expect_fixpoint: 2 1 2
	a1 := new(int)
	b1 := foo(a1)
	print(*b1)
}

func foo2(a *A) *A { // expect_fixpoint: 2 1 15
	if a == nil {
		return nil
	}
	return a.aptr
}

func testContract2() { // expect_fixpoint: 2 1 8
	b1 := foo2(&A{})
	print(*b1)
}

type myString []*string

// nilable(s[])
func (s *myString) testNamedType() { // expect_fixpoint: 2 1 3
	x := *s
	_ = *x[0]
}

func testNestedPointer() { // expect_fixpoint: 4 2 4
	a1 := &A{}
	for i := 0; i < 10; i++ {
		a2 := &a1
		(*a2).ptr = new(int)
		*a2 = nil
	}
}
