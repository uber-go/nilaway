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

// Package app is the consumer side of the cross-package return-shape tests: the deep dereferences
// land here, so the diagnostics are asserted with //want in this package. See package lib.
package app

import (
	"go.uber.org/structinitv2/returnshape/lib"
	"go.uber.org/structinitv2/returnshape/mid"
)

// Param tie: b ties to t; t.Mid.Child is nil, so the deep deref flags.
func useForwardParam() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := lib.ForwardParam(t)
	print(b.Mid.Child.Ptr) //want "uninitialized field `Child`"
}

// The tie carries t's real (non-nil) shape; no flag.
func useForwardParamSafe() {
	t := &lib.Outer{Mid: &lib.Node{Child: &lib.Leaf{}}}
	b := lib.ForwardParam(t)
	print(b.Mid.Child.Ptr)
}

// ForwardParamNilField nils the field before forwarding; the deref must observe the post-call nil.
func useForwardParamNilField() {
	t := &lib.Node{Child: &lib.Leaf{}}
	b := lib.ForwardParamNilField(t)
	print(b.Child.Ptr) //want "field `Child` of param 0 of `ForwardParamNilField`"
}

// Receiver forwarding: b ties to the receiver t; t.Child is nil.
func useRecv() {
	t := &lib.Node{}
	b := t.Self()
	print(b.Child.Ptr) //want "uninitialized field `Child`"
}

// Field-projection param tie: b ties to t.In; t.In.Child is nil.
func useForwardParamProjection() {
	t := &lib.Wrap{In: &lib.Inner{}}
	b := lib.ForwardParamProjection(t)
	print(b.Child.Ptr) //want "uninitialized field `Child`"
}

// Transitive param tie: the tie reaches back to the caller's argument t, whose Mid.Child is nil.
func useForwardParamTransitive() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := lib.ForwardParamTransitive(t)
	print(b.Mid.Child.Ptr) //want "uninitialized field `Child`"
}

// The transitive tie carries t's real (non-nil) shape; no flag.
func useForwardParamTransitiveSafe() {
	t := &lib.Outer{Mid: &lib.Node{Child: &lib.Leaf{}}}
	b := lib.ForwardParamTransitive(t)
	print(b.Mid.Child.Ptr)
}

// Ambiguous multi-return forwarder: the result could be either parameter, so the tie bails — a
// documented under-report, NOT flagged.
func useForwardParamAmbiguous() {
	t := &lib.Outer{Mid: &lib.Node{}}
	w := &lib.Outer{Mid: &lib.Node{}}
	b := lib.ForwardParamAmbiguous(t, w, true)
	print(b.Mid.Child.Ptr)
}

// Cross-package transitive tie: the tie reaches the caller's argument t two package hops away.
func useForwardParamCrossPkg() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := mid.ForwardParamCrossPkg(t)
	print(b.Mid.Child.Ptr) //want "uninitialized field `Child`"
}

// Mixed sometimes constructs (Mid.Child nil) and sometimes forwards its param. The forwarding
// summary is dropped, but the construct branch's own nil field is still a genuine true positive.
func useMixed() {
	t := &lib.Outer{Mid: &lib.Node{Child: &lib.Leaf{}}}
	b := lib.Mixed(t, true)
	print(b.Mid.Child.Ptr) //want "field `Mid.Child` of result 0 of `Mixed`"
}

// MixedSafe's construct branch is safe but it also forwards its param, so the forwarding tie is
// dropped: even though t.Mid.Child is nil, b must NOT be tied to t, so this is NOT flagged.
func useMixedSafe() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := lib.MixedSafe(t, true)
	print(b.Mid.Child.Ptr)
}

// Spreading a multi-result call (`TwoOut()`) into a forwarder gives the tie a non-lvalue argument,
// so it must bail. The deref is a sound under-report; the point is that analysis completes.
func useForwardFirstParamSpread() {
	b := lib.ForwardFirstParam(lib.TwoOut())
	print(b.Mid.Child.Ptr)
}
