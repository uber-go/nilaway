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
	"go.uber.org/nilaway/util/orderedmap"
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
	Implicants *orderedmap.OrderedMap[primitiveSite, primitiveFullTrigger]
	// Implicates stores downstream constraints from this site.
	Implicates *orderedmap.OrderedMap[primitiveSite, primitiveFullTrigger]
}

func (e *UndeterminedVal) isInferredVal() {}
