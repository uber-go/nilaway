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

// Package lib is the producer side of the cross-package return-shape tests: its constructors return
// values whose deep field nilability is dereferenced only by the sibling app package. The struct
// types are distinct at each level (Outer -> Node -> Leaf) so the deep paths are non-recursive.
package lib

// Leaf is the deepest struct; its Ptr is the dereference target.
type Leaf struct {
	Ptr *int
}

// Node nests a Leaf.
type Node struct {
	Child *Leaf
}

// Outer nests a Node, giving the depth-2 field path Mid.Child.
type Outer struct {
	Mid *Node
}

// ReturnDeepNil returns a value whose Mid is set but whose nested Mid.Child is left nil.
func ReturnDeepNil() *Outer {
	x := &Outer{Mid: &Node{}}
	return x
}

// ReturnDeepNonNil is the negative control: it initializes Mid.Child, so the same deref is safe.
func ReturnDeepNonNil() *Outer {
	x := &Outer{Mid: &Node{Child: &Leaf{}}}
	return x
}

// ReturnForwardedConstructor forwards a constructor's result directly (`return g()`), so it is as
// deep as ReturnDeepNil's own return shape.
func ReturnForwardedConstructor() *Outer {
	return ReturnDeepNil()
}
