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

	"go.uber.org/nilaway/annotation"
)

// An InferredVal is the information that we export about an annotation site after having
// witnessed an assertion about that site. If that assertion, and any other we've observed here and
// in upstream packages, tie the site to a necessary nilness or nonnilness, then an `DeterminedVal`
// will be used - which just wraps an `ExplainedBool` indicating the chain of implications FROM
// a definitely-produces-nil site TO this site forcing it to be nilable, or the chain of implications
// FROM this site TO a consumes-as-nonnil site forcing it to be nonnil. If we witness an assertion
// involving this annotation site, but it does not force the site to be nilable or nonnil, then an
// `UndeterminedVal` is used - which just annotates this site with a list of assertions from it
// to other sites (its implicates), and from other sites to it (its implicants).
//
// In summary, InferredAnnotationVals are the values in the maps InferredMap that we export
// to indicate the state of multi-package inference between packages.
type InferredVal interface {
	isInferredVal()
	copy() InferredVal
}

// An DeterminedVal placed on annotation site X in an InferredMap indicates that site X
// must be either nilable or nonnil (see the enclosed ExplainedBool for which one and why) as a result
// of information that we've observed either in this package or an upstream package. If conflicting
// ExplainedBoolVals are ever calculated for the same site, this produces an overconstrainedConflict.
type DeterminedVal struct {
	// Bool marks the reason why this value is determined, e.g., because the annotation says so,
	// or because it is being passed to a nonnil site.
	Bool ExplainedBool
}

func (e *DeterminedVal) isInferredVal() {}

func (e *DeterminedVal) copy() InferredVal { return &DeterminedVal{Bool: e.Bool} }

// An UndeterminedVal placed on annotation site X in an InferredMap indicates that we
// have witnessed at least one assertion about site X, but those assertions do not fix a nilability to
// the site. If X is the producer of the assertion, then the assertion (along with its consumer) will
// be added to the list `Implicates` of this UndeterminedVal, and if X is the consumer, then the
// assertion (along with its producer) will be added to the list `Implicants`.
//
// As multi-package inference proceeds, these UndeterminedVal's serve to define the known implication
// graph between underconstrained annotation sites. That graph will continue to grow across packages until
// one of its nodes is forced to be nilable or nonnil due to a definite nil production or nonnil consumption,
// at which point a walk of the graph will be performed from that definite site to also fix
// (as ExplainedBoolVals) any other sites that become definite as a result. In this case of a definite nil
// production discovered, the graph will be walked "forwards" - i.e. using the "Implicates" pointers -
// assigning true (nilable) to every site discovered, and in the case of a definite nonnil consumption
// discovered, the graph will be walked "backwards" - i.e. using the "Implicants" pointers - assigning
// false (nonnil) to every site discovered.
type UndeterminedVal struct {
	// Implicants stores upstream constraints to this site.
	Implicants SitesWithAssertions
	// Implicates stores downstream constraints from this site.
	Implicates SitesWithAssertions
}

func (e *UndeterminedVal) copy() InferredVal {
	copySitesWithAssertions := func(s SitesWithAssertions) SitesWithAssertions {
		out := make(SitesWithAssertions)
		for site, trigger := range s {
			out[site] = trigger
		}
		return out
	}

	return &UndeterminedVal{
		Implicants: copySitesWithAssertions(e.Implicants),
		Implicates: copySitesWithAssertions(e.Implicates),
	}
}

// SitesWithAssertions is a type that allows us to encode a set of annotation sites annotated with
// an assertion for each one. This is used in an UndeterminedVal to specify the implicants and
// implicates of a site along with the triggers that brought about those implications.
type SitesWithAssertions map[PrimitiveSite]PrimitiveFullTrigger

func newSitesWithAssertions() SitesWithAssertions {
	return make(SitesWithAssertions)
}

func (s SitesWithAssertions) addSiteWithAssertion(site PrimitiveSite, assertion PrimitiveFullTrigger) {
	s[site] = assertion
}

func (e *UndeterminedVal) isInferredVal() {}

// inferredValDiff determines the incremental information that should be exported
// if `old` was already observed as the InferredVal for a given AnnotationSite,
// and `new` is determined to be a new value that should now replace that old value.
// If the `new` value does not supersede the old value, i.e. overwriting one DeterminedVal
// with a CONFLICTING DeterminedVal, a panic is thrown to indicate failure in programming logic.
// Some notable cases here include replacing an UndeterminedVal with an DeterminedVal,
// in which case we just export the DeterminedVal as the incremental update, and
// replacing an UndeterminedVal with an UndeterminedVal with more edges, in which case
// we just export an UndeterminedVal containing exactly the new edges - this ensures that
// merging the incremental update and the existing upstream information yields the value `new`
// for sue by downstream packages.
// Additionally, a boolean flag is returned indicating whether there is new information to be
// incrementally exported at all!
// Summarizing output behavior: if `new` does not supersede `old`, the function panics
// if `new` strictly supersedes `old`, (diff, true) is returned
// if `new` offers the same information as `old`, (garbage, false) is returned.
func inferredValDiff(newVal, oldVal InferredVal) (InferredVal, bool) {
	noSupersede := func() {
		panic(fmt.Sprintf("ERROR: new value %s does not supersede old value %s", newVal, oldVal))
	}

	sitesWithAssertionsDiff := func(new, old SitesWithAssertions) (SitesWithAssertions, bool) {
		diff := make(SitesWithAssertions)
		diffNonempty := false
		for site, trigger := range new {
			if _, oldPresent := old[site]; !oldPresent {
				diff[site] = trigger
				diffNonempty = true
			}
		}
		return diff, diffNonempty
	}

	switch val := newVal.(type) {
	case *DeterminedVal:
		switch old := oldVal.(type) {
		case *DeterminedVal:
			if val.Bool.Val() != old.Bool.Val() {
				noSupersede()
			}
			return nil, false
		case *UndeterminedVal:
			return val, true
		}
	case *UndeterminedVal:
		switch old := oldVal.(type) {
		case *DeterminedVal:
			noSupersede()
		case *UndeterminedVal:
			implicants, implicantsDiffs := sitesWithAssertionsDiff(val.Implicants, old.Implicants)
			implicates, implicatesDiffs := sitesWithAssertionsDiff(val.Implicates, old.Implicates)
			return &UndeterminedVal{
				Implicants: implicants,
				Implicates: implicates,
			}, implicantsDiffs || implicatesDiffs
		}
	}
	panic(fmt.Sprintf("ERROR: unrecognized InferredAnnotationVals: %T, %T", newVal, oldVal))
}

// Implication represents an implication edge that nilable flows in the direction of and nonnil
// flows in the opposite direction of. For example, if a producer is nilable, then the consumer
// must be nilable, but not vice versa. Similarly, if a consumer is nonnil, then the producer must
// be nonnil, but not vice versa. The Controller field is the site that is responsible for if this
// implication edge is considered or not during inference of the graph. If the Controlled field is
// false, then the implication edge is considered regardless of the value of the controller site.
// If the Controlled is true, then the implication edge is only considered if the controller site
// is nilable. The IsProducerValid and IsConsumerValid fields are used to determine if the producer
// and consumer are valid to use for the given implication edge. The HasParamProducer and
// HasRetConsumer fields are used to determine if the producer and consumer originally
// correspond to FuncParam producer and UseAsReturn consumer, respectively. The CtrtParamIndex
// determines which parameter of the function is involved in the contract. The CtrtRetIndex
// determines which return value of the function is involved in the contract.
type Implication struct {
	Producer         PrimitiveSite
	Consumer         PrimitiveSite
	Assertion        PrimitiveFullTrigger
	Controller       PrimitiveSite
	Controlled       bool
	IsProducerValid  bool
	IsConsumerValid  bool
	HasParamProducer bool
	HasRetConsumer   bool
	CtrtParamIndex   int
	CtrtRetIndex     int
}

// SetProducerFromKey sets the producer site of the implication edge to the site corresponding
// to the given key.
func (i *Implication) SetProducerFromKey(primitiver *Primitivizer, key annotation.Key, isDeep bool) {
	i.Producer = primitiver.site(key, isDeep)
}

// SetConsumerFromKey sets the consumer site of the implication edge to the site corresponding
// to the given key.
func (i *Implication) SetConsumerFromKey(primitiver *Primitivizer, key annotation.Key, isDeep bool) {
	i.Consumer = primitiver.site(key, isDeep)
}

// SetControllerFromKey sets the controller site of the implication edge to the site corresponding
// to the given key.
func (i *Implication) SetControllerFromKey(primitiver *Primitivizer, key annotation.Key, isDeep bool) {
	i.Controller = primitiver.site(key, isDeep)
}

// AllSites returns Producer, Consumer, and Controller as a slice.
func (i *Implication) AllSites() []PrimitiveSite {
	return []PrimitiveSite{i.Producer, i.Consumer, i.Controller}
}

// String returns a string representation of the implication edge for debugging purposes.
func (i *Implication) String() string {
	return fmt.Sprintf(
		"Producer: %s,\n"+
			"Consumer: %s,\n"+
			"Assertion: %v,\n"+
			"Controller: %s,\n"+
			"Controlled: %t,\n"+
			"IsProducerValid: %t,\n"+
			"IsConsumerValid: %t,\n"+
			"HasParamProducer: %t,\n"+
			"HasRetConsumer: %t,\n"+
			"CtrtParamIndex: %d,\n"+
			"CtrtRetIndex: %d",
		i.Producer.String(),
		i.Consumer.String(),
		i.Assertion,
		i.Controller.String(),
		i.Controlled,
		i.IsProducerValid,
		i.IsConsumerValid,
		i.HasParamProducer,
		i.HasRetConsumer,
		i.CtrtParamIndex,
		i.CtrtRetIndex,
	)
}
