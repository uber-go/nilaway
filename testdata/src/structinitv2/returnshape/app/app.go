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
	"structinitv2/returnshape/lib"
	"structinitv2/returnshape/mid"
)

// A forwarded parameter keeps this call's argument shape.
func useForwardParam() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := lib.ForwardParam(t)
	print(b.Mid.Child.Ptr) //want "field `Mid.Child` of result 0 of `ForwardParam`"
}

// A safe call does not inherit another call's nil argument.
func useForwardParamSafe() {
	t := &lib.Outer{Mid: &lib.Node{Child: &lib.Leaf{}}}
	b := lib.ForwardParam(t)
	print(b.Mid.Child.Ptr)
}

// A forwarded parameter observes post-call field writes.
func useForwardParamNilField() {
	t := &lib.Node{Child: &lib.Leaf{}}
	b := lib.ForwardParamNilField(t)
	print(b.Child.Ptr) //want "field `Child` of result 0 of `ForwardParamNilField`"
}

// Receiver forwarding uses the receiver as its source.
func useRecv() {
	t := &lib.Node{}
	b := t.Self()
	print(b.Child.Ptr) //want "field `Child` of result 0 of `Self`"
}

// A projected result uses the argument field as its source.
func useForwardParamProjection() {
	t := &lib.Wrap{In: &lib.Inner{}}
	b := lib.ForwardParamProjection(t)
	print(b.Child.Ptr) //want "field `Child` of result 0 of `ForwardParamProjection`"
}

// Transitive parameter sources are not resolved.
func useForwardParamTransitive() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := lib.ForwardParamTransitive(t)
	print(b.Mid.Child.Ptr)
}

// The transitive tie carries t's real (non-nil) shape; no flag.
func useForwardParamTransitiveSafe() {
	t := &lib.Outer{Mid: &lib.Node{Child: &lib.Leaf{}}}
	b := lib.ForwardParamTransitive(t)
	print(b.Mid.Child.Ptr)
}

// Ambiguous parameter sources remain silent.
func useForwardParamAmbiguous() {
	t := &lib.Outer{Mid: &lib.Node{}}
	w := &lib.Outer{Mid: &lib.Node{}}
	b := lib.ForwardParamAmbiguous(t, w, true)
	print(b.Mid.Child.Ptr)
}

// Cross-package transitive parameter sources are not resolved.
func useForwardParamCrossPkg() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := mid.ForwardParamCrossPkg(t)
	print(b.Mid.Child.Ptr)
}

// Mixed construct/forward results retain concrete effects but no parameter source.
func useMixed() {
	t := &lib.Outer{Mid: &lib.Node{Child: &lib.Leaf{}}}
	b := lib.Mixed(t, true)
	print(b.Mid.Child.Ptr) //want "field `Mid.Child` of result 0 of `Mixed`"
}

// A dropped source must not tie the safe constructed result to its argument.
func useMixedSafe() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := lib.MixedSafe(t, true)
	print(b.Mid.Child.Ptr)
}

// Tuple-spread arguments cannot be resolved and remain silent.
func useForwardFirstParamSpread() {
	b := lib.ForwardFirstParam(lib.TwoOut())
	print(b.Mid.Child.Ptr)
}

// A nil argument at one call must not affect another call's result.
func usePairNoPoison() {
	ptr := 1
	requested := &lib.Leaf{Ptr: &ptr}
	create := lib.NewPair(nil, requested)
	print(*create.Requested.Ptr)
	existing := &lib.Leaf{Ptr: &ptr}
	update := lib.NewPair(existing, requested)
	print(*update.Existing.Ptr)
}

// A nil argument affects its own call result.
func usePairBad() {
	ptr := 1
	requested := &lib.Leaf{Ptr: &ptr}
	bad := lib.NewPair(nil, requested)
	print(*bad.Existing.Ptr) //want "field `Existing` of result 0"
}

// A copied field's descendants map to the argument's descendants.
func usePairDeepBad() {
	requested := &lib.Leaf{}
	existing := &lib.Leaf{}
	bad := lib.NewPair(existing, requested)
	print(*bad.Existing.Ptr) //want "uninitialized field `Ptr`"
}

// A copied field observes the argument's post-call value.
func usePairDeepPostCallBad() {
	ptr := 1
	requested := &lib.Leaf{Ptr: &ptr}
	existing := &lib.Leaf{Ptr: &ptr}
	bad := lib.NewPairAfterNil(existing, requested)
	print(*bad.Existing.Ptr) //want "field `Ptr` of param 0 of `NewPairAfterNil`"
}

// A nil projected field makes this call's shallow result nil.
func useForwardParamProjectionNil() {
	t := &lib.Wrap{}
	print(lib.ForwardParamProjection(t).Child.Ptr) //want "uninitialized field `In`"
}

// A safe projection call does not inherit another call's nil source.
func useForwardParamProjectionShallowSafe() {
	t := &lib.Wrap{In: &lib.Inner{Child: &lib.Leaf{}}}
	print(lib.ForwardParamProjection(t).Child.Ptr)
}

// Receiver field projections use the same shallow result binding.
func useReceiverProjectionNil() {
	t := &lib.Wrap{}
	print(t.ReceiverProjection().Child.Ptr) //want "uninitialized field `In`"
}

func useReceiverProjectionSafe() {
	t := &lib.Wrap{In: &lib.Inner{Child: &lib.Leaf{}}}
	print(t.ReceiverProjection().Child.Ptr)
}

// Calls at different locations use distinct sites even when otherwise identical.
func useIdenticalCallsStayDistinct() {
	t := &lib.Wrap{}
	first := t.ReceiverProjection()
	print(first.Child.Ptr) //want "uninitialized field `In`"
	t.In = &lib.Inner{Child: &lib.Leaf{}}
	second := t.ReceiverProjection()
	print(second.Child.Ptr)
}
