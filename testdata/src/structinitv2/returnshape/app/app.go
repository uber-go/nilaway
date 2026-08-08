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

// Transitive forwarding resolves against this caller's argument.
func useForwardParamTransitive() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := lib.ForwardParamTransitive(t)
	print(b.Mid.Child.Ptr) //want "field `Mid.Child` of result 0 of `ForwardParamTransitive`"
}

// A safe caller does not inherit another call's nil argument.
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

// Cross-package forwarding resolves against this caller's argument.
func useForwardParamCrossPkg() {
	t := &lib.Outer{Mid: &lib.Node{}}
	b := mid.ForwardParamCrossPkg(t)
	print(b.Mid.Child.Ptr) //want "field `Mid.Child` of result 0 of `ForwardParamCrossPkg`"
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

func useForwardParamProjectionInlineNil() {
	print(lib.ForwardParamProjection(&lib.Wrap{}).Child.Ptr) //want "uninitialized field `In`"
}

func useForwardParamProjectionInlineSafe() {
	print(lib.ForwardParamProjection(&lib.Wrap{In: &lib.Inner{Child: &lib.Leaf{}}}).Child.Ptr)
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

func useReceiverProjectionInlineNil() {
	print((&lib.Wrap{}).ReceiverProjection().Child.Ptr) //want "uninitialized field `In`"
}

func useReceiverProjectionInlineSafe() {
	print((&lib.Wrap{In: &lib.Inner{Child: &lib.Leaf{}}}).ReceiverProjection().Child.Ptr)
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

// A whole-value projection remains caller-sensitive through a forwarding call.
func useForwardProjectionTransitiveNil() {
	t := &lib.Wrap{}
	print(lib.ForwardProjectionTransitive(t).Child.Ptr) //want "uninitialized field `In`"
}

func useForwardProjectionTransitiveSafe() {
	t := &lib.Wrap{In: &lib.Inner{Child: &lib.Leaf{}}}
	print(lib.ForwardProjectionTransitive(t).Child.Ptr)
}

func useForwardProjectionTransitiveInlineNil() {
	print(lib.ForwardProjectionTransitive(&lib.Wrap{}).Child.Ptr) //want "uninitialized field `In`"
}

func useForwardProjectionTransitiveInlineSafe() {
	print(lib.ForwardProjectionTransitive(&lib.Wrap{In: &lib.Inner{Child: &lib.Leaf{}}}).Child.Ptr)
}

// The shallow projection source also survives a cross-package forwarding hop.
func useForwardProjectionCrossPkgNil() {
	t := &lib.Wrap{}
	print(mid.ForwardProjectionCrossPkg(t).Child.Ptr) //want "uninitialized field `In`"
}

func useForwardProjectionCrossPkgSafe() {
	t := &lib.Wrap{In: &lib.Inner{Child: &lib.Leaf{}}}
	print(mid.ForwardProjectionCrossPkg(t).Child.Ptr)
}

// The no-poison guarantee survives transitive composition through `return g(args)`.
func usePairViaCallNoPoison() {
	ptr := 1
	requested := &lib.Leaf{Ptr: &ptr}
	create := lib.NewPairViaCall(nil, requested)
	print(*create.Requested.Ptr)
	existing := &lib.Leaf{Ptr: &ptr}
	update := lib.NewPairViaCall(existing, requested)
	print(*update.Existing.Ptr)
}

// The composed field-level source keeps the true positive at its own call site.
func usePairViaCallBad() {
	ptr := 1
	requested := &lib.Leaf{Ptr: &ptr}
	bad := lib.NewPairViaCall(nil, requested)
	print(*bad.Existing.Ptr) //want "field `Existing` of result 0"
}

// The no-poison guarantee also survives a cross-package forwarding hop.
func useForwardPairNoPoison() {
	ptr := 1
	requested := &lib.Leaf{Ptr: &ptr}
	create := mid.ForwardPair(nil, requested)
	print(*create.Requested.Ptr)
	existing := &lib.Leaf{Ptr: &ptr}
	update := mid.ForwardPair(existing, requested)
	print(*update.Existing.Ptr)
}

// Cross-package field sources keep the bad call visible.
func useForwardPairBad() {
	requested := &lib.Leaf{}
	bad := mid.ForwardPair(nil, requested)
	print(*bad.Existing.Ptr) //want "field `Existing` of result 0"
}
