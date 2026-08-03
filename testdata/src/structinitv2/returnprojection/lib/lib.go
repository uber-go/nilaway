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

// Inner and Wrap give a field-projection return target.
type Inner struct {
	Child *Leaf
}

// Wrap nests an Inner so ReturnProjection can return the projection x.In.
type Wrap struct {
	In *Inner
}

// ReturnProjection returns a field projection (`return x.In`); the result is an *Inner whose Child
// is left nil.
func ReturnProjection() *Inner {
	x := &Wrap{In: &Inner{}}
	return x.In
}

// ReturnProjectionSafe is the negative control for ReturnProjection: Child is initialized.
func ReturnProjectionSafe() *Inner {
	x := &Wrap{In: &Inner{Child: &Leaf{}}}
	return x.In
}

// Rec is a self-referential type: paths through Self are recursive and skipped past the first
// unrolling, so MkRec's top-level Ptr is tracked but Self.Ptr is not.
type Rec struct {
	Ptr  *int
	Self *Rec
}

// MkRec constructs a recursive value with Self set but Ptr (and Self.Ptr) nil.
func MkRec() *Rec {
	return &Rec{Self: &Rec{}}
}
