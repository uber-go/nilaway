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

package inference

import (
	"fmt"
	"go/token"
)

// An ExplainedBool is a boolean value, wrapped by a "reason" that we came to the conclusion it should
// have that value. ExplainedBools are used as labels on annotation sites once their state (true for
// nilable, false for nonnil) is established, with the explanations serving primarily to generate
// error messages in the case that conflicting labels are required for a given site. The currently used
// explanations are:
// - <Val>BecauseShallowConstraint: Applied to site X when X was half of an assertion where the other half was fixed as a definite site of nil production or nonnil consumption
// - <Val>BecauseDeepConstraint: Applied to site X when X was half of an assertion where the other half was fixed, but through a deeper chain of assertions
// - <Val>BecauseAnnotation: Applied to site X when a syntactic annotation was discovered on X
type ExplainedBool interface {
	fmt.Stringer

	Val() bool
	Position() token.Position
	TriggerReprs() (producer fmt.Stringer, consumer fmt.Stringer)
	DeeperReason() ExplainedBool
}

// ExplainedTrue is a common embedding in all instances of ExplainedBool that wrap the value `true`
type ExplainedTrue struct{}

// Val for an ExplainedTrue always returns `true` (this is the point of an ExplainedTrue)
func (ExplainedTrue) Val() bool {
	return true
}

// ExplainedFalse is a common embedding in all instances of ExplainedBool that wrap the value `false`
type ExplainedFalse struct{}

// Val for an ExplainedFalse always returns `false` (this is the point of an ExplainedFalse)
func (ExplainedFalse) Val() bool {
	return false
}

// TrueBecauseShallowConstraint is used as the label for site Y when an assertion of the form
// `nilable X -> nilable Y` is discovered and the trigger for `nilable X` always fires (i.e. yields
// nilable) - for example because it is the literal nil or an unguarded map read. In all cases, this
// constrains the site Y to be nilable, so we label Y with `ExplainedTrue` as a
// `TrueBecauseShallowConstraint`, wrapped along with the assertion that we discovered to yield the
// truth.
type TrueBecauseShallowConstraint struct {
	ExplainedTrue
	ExternalAssertion primitiveFullTrigger
}

func (t TrueBecauseShallowConstraint) String() string {
	return fmt.Sprintf(
		"NILABLE because it describes the value %s, and that value is %s, where it is NILABLE",
		t.ExternalAssertion.ConsumerRepr, t.ExternalAssertion.ProducerRepr)
}

// Position is the position of underlying site.
func (t TrueBecauseShallowConstraint) Position() token.Position {
	return t.ExternalAssertion.Position
}

// TriggerReprs returns the compact representation structs for the producer and consumer.
func (t TrueBecauseShallowConstraint) TriggerReprs() (fmt.Stringer, fmt.Stringer) {
	return t.ExternalAssertion.ProducerRepr, t.ExternalAssertion.ConsumerRepr
}

// DeeperReason returns another ExplainedBool that marks the deeper reason of this constraint.
// It is only nonnil for deep constraints.
func (t TrueBecauseShallowConstraint) DeeperReason() ExplainedBool {
	return nil
}

// FalseBecauseShallowConstraint is used as the label for site X when an assertion of the form
// `nilable X -> nilable Y` is discovered and the trigger for `nilable Y` always fires (i.e. yields
// nonnil) - for example because it is the dereferenced as a pointer or passed to a field access. In
// all cases, this constrains the site X to be nonnil, so we label X with `ExplainedFalse` as a
// `FalseBecauseShallowConstraint`, wrapped along with the assertion that we discovered to yield the
// falsehood.
type FalseBecauseShallowConstraint struct {
	ExplainedFalse
	ExternalAssertion primitiveFullTrigger
}

func (f FalseBecauseShallowConstraint) String() string {
	return fmt.Sprintf(
		"NONNIL because it describes the value %s, and that value is %s, where it must be NONNIL",
		f.ExternalAssertion.ProducerRepr, f.ExternalAssertion.ConsumerRepr)
}

// Position is the position of underlying site.
func (f FalseBecauseShallowConstraint) Position() token.Position {
	return f.ExternalAssertion.Position
}

// TriggerReprs returns the compact representation structs for the producer and consumer.
func (f FalseBecauseShallowConstraint) TriggerReprs() (fmt.Stringer, fmt.Stringer) {
	return f.ExternalAssertion.ProducerRepr, f.ExternalAssertion.ConsumerRepr
}

// DeeperReason returns another ExplainedBool that marks the deeper reason of this constraint.
// It is only nonnil for deep constraints.
func (f FalseBecauseShallowConstraint) DeeperReason() ExplainedBool {
	return nil
}

// TrueBecauseDeepConstraint is used as the label for a site Y when an assertion of the form
// `nilable X -> nilable Y` is discovered along with some reason for X to be nilable, besides it
// necessarily being so because it always fires. This reason could be any ExplainedTrue - such as
// `TrueBecauseAnnotation`, `TrueBecauseShallowConstraint`, or another `TrueBecauseDeepConstraint`.
type TrueBecauseDeepConstraint struct {
	ExplainedTrue
	InternalAssertion primitiveFullTrigger
	DeeperExplanation ExplainedBool
}

func (t TrueBecauseDeepConstraint) String() string {
	return fmt.Sprintf(
		"NILABLE because it describes the value %s, and that value is %s, where it is %s",
		t.InternalAssertion.ConsumerRepr, t.InternalAssertion.ProducerRepr, t.DeeperExplanation.String())
}

// Position is the position of underlying site.
func (t TrueBecauseDeepConstraint) Position() token.Position {
	return t.InternalAssertion.Position
}

// TriggerReprs returns the compact representation structs for the producer and consumer.
func (t TrueBecauseDeepConstraint) TriggerReprs() (fmt.Stringer, fmt.Stringer) {
	return t.InternalAssertion.ProducerRepr, t.InternalAssertion.ConsumerRepr
}

// DeeperReason returns another ExplainedBool that marks the deeper reason of this constraint.
// It is only nonnil for deep constraints.
func (t TrueBecauseDeepConstraint) DeeperReason() ExplainedBool {
	return t.DeeperExplanation
}

// FalseBecauseDeepConstraint is used as the label for a site X when an assertion of the form
// `nilable X -> nilable Y` is discovered along with some reason for Y to be nonnil, besides it
// necessarily being so because it always fires. This reason could be any ExplainedFalse - such as
// `FalseBecauseAnnotation`, `FalseBecauseShallowConstraint`, or another `FalseBecauseDeepConstraint`.
type FalseBecauseDeepConstraint struct {
	ExplainedFalse
	InternalAssertion primitiveFullTrigger
	DeeperExplanation ExplainedBool
}

func (f FalseBecauseDeepConstraint) String() string {
	return fmt.Sprintf(
		"NONNIL because it describes the value %s, and that value is %s, where it must be %s",
		f.InternalAssertion.ProducerRepr, f.InternalAssertion.ConsumerRepr, f.DeeperExplanation.String())
}

// Position is the position of underlying site.
func (f FalseBecauseDeepConstraint) Position() token.Position {
	return f.InternalAssertion.Position
}

// TriggerReprs returns the compact representation structs for the producer and consumer.
func (f FalseBecauseDeepConstraint) TriggerReprs() (fmt.Stringer, fmt.Stringer) {
	return f.InternalAssertion.ProducerRepr, f.InternalAssertion.ConsumerRepr
}

// DeeperReason returns another ExplainedBool that marks the deeper reason of this constraint.
// It is only nonnil for deep constraints.
func (f FalseBecauseDeepConstraint) DeeperReason() ExplainedBool {
	return f.DeeperExplanation
}

// TrueBecauseAnnotation is used as the label for a site X on which a literal annotation "//nilable(x)"
// has been discovered - forcing that site to be nilable.
type TrueBecauseAnnotation struct {
	ExplainedTrue
	AnnotationPos token.Position
}

func (TrueBecauseAnnotation) String() string {
	return "NILABLE because it is annotated as so"
}

// Position is the position of underlying site.
func (t TrueBecauseAnnotation) Position() token.Position {
	return t.AnnotationPos
}

// TriggerReprs simply returns nil, nil since this constraint is the result of an annotation.
func (TrueBecauseAnnotation) TriggerReprs() (fmt.Stringer, fmt.Stringer) {
	return nil, nil
}

// DeeperReason returns another ExplainedBool that marks the deeper reason of this constraint.
// It is only nonnil for deep constraints.
func (TrueBecauseAnnotation) DeeperReason() ExplainedBool {
	return nil
}

// FalseBecauseAnnotation is used as the label for a site X on which a literal annotation "//nonnil(x)"
// has been discovered - forcing that site to be nonnil.
type FalseBecauseAnnotation struct {
	ExplainedFalse
	AnnotationPos token.Position
}

func (FalseBecauseAnnotation) String() string {
	return "NONNIL because it is annotated as so"
}

// Position is the position of underlying site.
func (f FalseBecauseAnnotation) Position() token.Position {
	return f.AnnotationPos
}

// TriggerReprs simply returns nil, nil since this constraint is the result of an annotation.
func (FalseBecauseAnnotation) TriggerReprs() (fmt.Stringer, fmt.Stringer) {
	return nil, nil
}

// DeeperReason returns another ExplainedBool that marks the deeper reason of this constraint.
// It is only nonnil for deep constraints.
func (f FalseBecauseAnnotation) DeeperReason() ExplainedBool {
	return nil
}
