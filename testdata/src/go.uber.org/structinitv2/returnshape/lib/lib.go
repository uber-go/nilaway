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

// ForwardParam forwards its parameter to its result (`return x`); the result's shape is the caller's
// argument, resolved per-call.
func ForwardParam(x *Outer) *Outer {
	return x
}

// ForwardParamNilField forwards its parameter but first nils a field of it, so a caller must observe
// the post-call value of the forwarded field.
func ForwardParamNilField(x *Node) *Node {
	x.Child = nil
	return x
}

// Self forwards its receiver to its result.
func (n *Node) Self() *Node {
	return n
}

// ForwardParamProjection forwards its parameter at a field projection (`return x.In`), so the caller
// ties to arg.In.
func ForwardParamProjection(x *Wrap) *Inner {
	return x.In
}

// ForwardParamTransitive returns a call to ForwardParam passing its own parameter, so it too
// forwards param 0 to result 0; a caller's deref observes its own argument shape.
func ForwardParamTransitive(y *Outer) *Outer {
	return ForwardParam(y)
}

// ForwardParamAmbiguous forwards a different parameter on each return site. Because the result could
// be either parameter, the caller-side tie bails — a documented under-report.
func ForwardParamAmbiguous(y *Outer, z *Outer, pick bool) *Outer {
	if pick {
		return ForwardParam(y)
	}
	return ForwardParam(z)
}

// Mixed constructs on one branch (Mid.Child nil) and forwards its parameter on the other, so both
// the return-shape and param-forwarding summaries are dropped. The construct branch's own nil field
// is still observed by a caller — a true positive the drop must not suppress.
func Mixed(x *Outer, pick bool) *Outer {
	if pick {
		return &Outer{Mid: &Node{}}
	}
	return x
}

// MixedSafe is the false-positive guard: the construct branch is safe while the other forwards a
// parameter, so the param-forwarding summary is dropped and a caller passing a nil-field argument is
// not wrongly flagged.
func MixedSafe(x *Outer, pick bool) *Outer {
	if pick {
		return &Outer{Mid: &Node{Child: &Leaf{}}}
	}
	return x
}

// TwoOut returns a struct and a bool so a caller can spread it into a forwarder's parameters
// (`ForwardFirstParam(TwoOut())`).
func TwoOut() (*Outer, bool) {
	return &Outer{Mid: &Node{}}, true
}

// ForwardFirstParam forwards its first parameter. Called as `ForwardFirstParam(TwoOut())`, the tie's
// candidate argument is the multi-result call itself, which is not a stable lvalue, so the tie bails.
func ForwardFirstParam(x *Outer, ok bool) *Outer {
	return x
}
