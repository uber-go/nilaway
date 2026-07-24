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

// Composing a struct-returning call directly into a parameter or receiver boundary: the callee's
// per-field return summary must bind to the parameter (or receiver) the returned value flows into.

package returnlocal

type composedLeaf struct {
	ptr *int
}

type composedNode struct {
	child *composedLeaf
}

// A struct-returning call used directly as an argument is composed into the parameter boundary.
func giveComposedForParam() *composedNode { return &composedNode{} }

func readComposedParam(n *composedNode) *int {
	return n.child.ptr //want "accessed field `ptr`"
}

func mComposedParam() *int {
	return readComposedParam(giveComposedForParam())
}

// The same return-to-boundary composition applies to a method receiver.
func giveComposedForReceiver() *composedNode { return &composedNode{} }

func (n *composedNode) readComposedReceiver() *int {
	return n.child.ptr //want "accessed field `ptr`"
}

func mComposedReceiver() *int {
	return giveComposedForReceiver().readComposedReceiver()
}
