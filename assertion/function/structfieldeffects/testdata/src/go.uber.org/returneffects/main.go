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

// Package returneffects is the fixture for concrete return-effect collection.
package returneffects

type Leaf struct {
	Ptr *int
}

type Node struct {
	Child *Leaf
}

type Outer struct {
	Mid   *Node
	Value Node
}

type Rec struct {
	Self *Rec
	Ptr  *int
}

func safeOuter() *Outer { // expect_effects:
	return &Outer{
		Mid:   &Node{Child: &Leaf{Ptr: new(int)}},
		Value: Node{Child: &Leaf{Ptr: new(int)}},
	}
}

func inlinePointer() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return &Outer{}
}

func inlineValue() Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return Outer{Mid: nil, Value: Node{Child: nil}}
}

func builtinNew() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return new(Outer)
}

func nestedAllocation() *Outer { // expect_effects: return_effects:0:Mid.Child return_effects:0:Value.Child
	return &Outer{Mid: &Node{}}
}

func unknownInitializer(mid *Node) *Outer { // expect_effects: return_effects:0:Value.Child
	return &Outer{
		Mid:   mid,
		Value: Node{},
	}
}

func shadowedNil() *Outer { // expect_effects:
	nil := &Node{Child: &Leaf{Ptr: new(int)}}
	return &Outer{
		Mid:   nil,
		Value: Node{Child: &Leaf{Ptr: new(int)}},
	}
}

func directForward() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return inlinePointer()
}

func genericConcrete[T any]() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return &Outer{}
}

func genericForward() *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return genericConcrete[int]()
}

func pair() (*Outer, *Node) { // expect_effects: return_effects:1:Child
	return safeOuter(), &Node{}
}

func cycleA(recurse bool) *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	if recurse {
		return cycleB(recurse)
	}
	return inlinePointer()
}

func cycleB(recurse bool) *Outer { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return cycleA(recurse)
}

func nestedFunctionReturn() *Outer { // expect_effects:
	_ = func() *Outer { return &Outer{} }
	return safeOuter()
}

func recursiveAllocation() *Rec { // expect_effects: return_effects:0:Ptr
	return &Rec{Self: &Rec{}}
}

func dynamicCall(f func() *Outer) *Outer { // expect_effects:
	return f()
}

func multiResult() (*Outer, error) { // expect_effects:
	return safeOuter(), nil
}

func multiResultWithNil() (Outer, error) { // expect_effects: return_effects:0:Mid return_effects:0:Value.Child
	return inlineValue(), nil
}

func multiResultSpread() (Outer, error) { // expect_effects:
	return multiResultWithNil()
}
