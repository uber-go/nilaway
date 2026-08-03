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

// Package lib is the producer side of the zero-value return tests.
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

// ReturnZeroValue returns a zero-value struct (`var x Outer; return x`), so Mid is nil. The value
// form is summarized; the naked form below is not.
func ReturnZeroValue() Outer {
	var x Outer
	return x
}

// NakedRet is a documented under-report: a naked named return has no result expression, so its shape
// is not summarized and the cross-package deref is NOT flagged.
func NakedRet() (x Outer) {
	return
}
