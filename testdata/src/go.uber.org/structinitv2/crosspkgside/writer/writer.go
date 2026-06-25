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

// Package writer defines struct types and functions that mutate a field of their parameter. It is
// imported by the sibling app package to exercise the cross-package parameter-side-effect boundary.
package writer

// Leaf is the dereference target.
type Leaf struct {
	Ptr *int
}

// Node holds the nilable field that is mutated across the package boundary.
type Node struct {
	Child *Leaf
}

// WriteNil sets the Child field of its parameter to nil.
func WriteNil(x *Node) {
	x.Child = nil
}

// Outer nests a Node, giving a depth-2 field path (Mid.Child).
type Outer struct {
	Mid *Node
}

// WriteDeepNil sets the nested Mid.Child field of its parameter to nil.
func WriteDeepNil(o *Outer) {
	o.Mid.Child = nil
}
