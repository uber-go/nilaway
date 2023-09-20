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

// Package inference implements the inference algorithm in NilAway to automatically infer the
// nilability of the annotation sites.
package inference

import (
	"encoding/gob"
	"fmt"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion/function/assertiontree"
	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/analysis"
)

// Engine is the structure responsible for running the inference: it contains methods to run
// various tasks for the inference and stores an internal map that can be obtained by calling
// Engine.InferredMapWithDiagnostics.
type Engine struct {
	pass *analysis.Pass
	// inferredMap is the internal inferred map that the engine writes to, it is initialized on the
	// construction of the engine and populated by the "Observe*" methods of the engine. Users
	// should use the Engine.InferredMapWithDiagnostics() method to obtain the current inferred map.
	inferredMap *InferredMap
	// conflicts stores conflicts encountered during the observations. It will be
	// converted to diagnostics and returned along with the current inferred map whenever users
	// request it.
	conflicts *ConflictList
	// controlledTriggersBySite stores the set of controlled triggers for each site if the site
	// controls any triggers. This field is for internal use in the struct only and should not be
	// accessed elsewhere.
	controlledTriggersBySite map[primitiveSite]map[annotation.FullTrigger]bool
}

// NewEngine constructs an inference engine that is ready to run inference.
func NewEngine(pass *analysis.Pass) *Engine {
	return &Engine{
		pass:        pass,
		inferredMap: newInferredMap(),
		conflicts:   &ConflictList{},
	}
}

// InferredMapWithDiagnostics returns the current inferred annotation map and a slice of diagnostics
// generated during inference.
func (e *Engine) InferredMapWithDiagnostics() (*InferredMap, []analysis.Diagnostic) {
	return e.inferredMap, e.conflicts.Diagnostics()
}

// ObserveUpstream imports all information from upstream dependencies. Specifically, it iterates
// over the direct imports of the passed pass's package, using the Facts mechanism to observe any
// InferredMap's that were computed by multi-package inference for that imported package.
// We copy the information not just into Mapping, but also into UpstreamMapping.
// As more information is observed through a call to ObservePackage, it will be
// added to Mapping but not UpstreamMapping, then, on a call to Export, only the information
// present in Mapping but not UpstreamMapping is exported to ensure minimization of output.
func (e *Engine) ObserveUpstream() {
	for _, packageFact := range e.pass.AllPackageFacts() {
		importedMap, ok := packageFact.Fact.(*InferredMap)
		if !ok {
			continue
		}
		importedMap.Range(func(site primitiveSite, val InferredVal) bool {
			switch val := val.(type) {
			case *DeterminedVal:
				// Fix as an Explained site any sites that `otherMap` knows are explained
				// This can yield an overconstrainedConflict if the current map disagrees on the
				// value of the site.
				e.observeSiteExplanation(site, val.Bool)
			case *UndeterminedVal:
				// Observe all forward implications from this site
				for implicateSite, implicateAssertion := range val.Implicates {
					e.observeImplication(site, implicateSite, implicateAssertion)
				}
				// Observe all backward implications from this site
				for implicantSite, implicantAssertion := range val.Implicants {
					e.observeImplication(implicantSite, site, implicantAssertion)
				}
			}
			return true
		})
	}

	// copy imported maps into upstreamMapping field
	e.inferredMap.Range(func(site primitiveSite, val InferredVal) bool {
		e.inferredMap.upstreamMapping[site] = val.copy()
		return true
	})
}

// ObserveAnnotations does one of two things. If the inferenceType is FullInfer, then it reads
// ONLY those annotations that are "set" (a separate flag for both nilability and deep nilability)
// in an annotation.Val - corresponding to syntactically provided annotations but not default
// annotations. Otherwise, it reads ALL values from the map pkgAnnotations including
// non-syntactically present annotations that simply arose from defaults.
// In this latter case, the subsequent calls to observeAssertion below cannot determine any local
// annotation sites, because they're all already determined, but they can yield failures.
func (e *Engine) ObserveAnnotations(pkgAnnotations *annotation.ObservedMap, mode ModeOfInference) {
	pkgAnnotations.Range(func(key annotation.Key, isDeep bool, val bool) {
		site := newPrimitiveSite(key, isDeep)
		if val {
			e.observeSiteExplanation(site, TrueBecauseAnnotation{Pos: site.Pos})
		} else {
			e.observeSiteExplanation(site, FalseBecauseAnnotation{Pos: site.Pos})
		}
	}, mode != NoInfer)
}

// ObservePackage observes all the annotations and assertions computed locally about the current
// package. The assertions are sorted based on whether they are already known to trigger without
// reliance on annotation sites, such as `x` in `x = nil; x.f`, which will generate
// `SingleAssertionFailure`s, whether they rely on only a single annotation site, determining that
// annotation site as a <Val>BecauseShallowConstraint by a call to observeSiteExplanation if
// necessary, or whether they rely on two annotation sites, in which case they result in a call to
// observeImplication. Before all assertions are sorted and handled thus, the annotations read for
// the package are iterated over and observed via calls to observeSiteExplanation as a <Val>BecauseAnnotation.
func (e *Engine) ObservePackage(pkgFullTriggers []annotation.FullTrigger) {
	// Separate out triggers with UseAsNonErrorRetDependentOnErrorRetNilability consumer from other triggers.
	// This is needed since whether UseAsNonErrorRetDependentOnErrorRetNilability triggers should be fired
	// is dependent on their corresponding UseAsErrorRetWithNilabilityUnknown triggers. By this separation,
	// we can process all other triggers, including UseAsErrorRetWithNilabilityUnknown, first, and once
	// their nilability status is known, then filter out the unnecessary UseAsNonErrorRetDependentOnErrorRetNilability
	// triggers, and run the pkg inference process again only for the remainder triggers.
	// Steps 1--3 below depict this approach in more detail.
	nonErrRetTriggers := make(map[annotation.FullTrigger]bool, 0)
	var otherTriggers []annotation.FullTrigger
	for _, t := range pkgFullTriggers {
		if _, ok := t.Consumer.Annotation.(annotation.UseAsNonErrorRetDependentOnErrorRetNilability); ok {
			nonErrRetTriggers[t] = true
		} else {
			otherTriggers = append(otherTriggers, t)
		}
	}

	// Step 1: build the inference map based on `otherTriggers` and incorporate those assertions into the `inferredAnnotationMap`
	e.buildPkgInferenceMap(otherTriggers)

	// Step 2: run error return handling procedure to filter out redundant triggers based on the error contract, and
	// keep only those UseAsNonErrorRetDependentOnErrorRetNilability triggers that are not deleted.
	// Call FilterTriggersForErrorReturn to filter triggers for error return handling -- inter-procedural and full-inference mode
	_, delTriggers := assertiontree.FilterTriggersForErrorReturn(
		pkgFullTriggers,
		func(p *annotation.ProduceTrigger) assertiontree.ProducerNilability {
			kind := p.Annotation.Kind()
			if kind == annotation.Conditional || kind == annotation.DeepConditional {
				site := p.Annotation.UnderlyingSite()
				if site == nil {
					panic(fmt.Sprintf("no underlying site found for conditional trigger %v", p))
				}

				isDeep := kind == annotation.DeepConditional
				primitive := newPrimitiveSite(site, isDeep)
				if val, ok := e.inferredMap.Load(primitive); ok {
					switch vType := val.(type) {
					case *DeterminedVal:
						if !vType.Bool.Val() {
							return assertiontree.ProducerIsNonNil
						}
					case *UndeterminedVal:
						// Consider the producer site as non-nil, if the determined value is non-nil, i.e.,
						// `!vType.Bool.Val()`, or the site is undetermined. Undetermined sites are assumed "non-nil" here based on the following:
						// (a) the inference algorithm does not propagate non-nil values forward, and
						// (b) the processing of the sites under question, error return sites, are allowed to be processed first in step 1 above
						//
						// This above assumption works favorably in most of the cases, except the one demonstrated in
						// `testdata/errorreturn/inference/downstream.go`, for instance, where it leads to a false negative.
						return assertiontree.ProducerIsNonNil
					}
				}
			}

			// In all other cases, return ProducerNilabilityUnknown to indicate that all we
			// know at this point is that `p` is nilable, which means that it could be nil but
			// is not guaranteed to be always nil nor non-nil.
			return assertiontree.ProducerNilabilityUnknown
		})

	// remove deleted triggers from nonErrRetTriggers
	for _, t := range delTriggers {
		delete(nonErrRetTriggers, t)
	}

	// Step 3: run the inference building process for only the remaining UseAsNonErrorRetDependentOnErrorRetNilability triggers, and collect assertions
	e.buildPkgInferenceMap(maps.Keys(nonErrRetTriggers))
}

func (e *Engine) buildPkgInferenceMap(triggers []annotation.FullTrigger) {
	// Map each site to all the triggers controlled by the site
	controlledTgsBySite := map[primitiveSite]map[annotation.FullTrigger]bool{}
	for _, trigger := range triggers {
		if !trigger.Controlled() {
			continue
		}
		// controller is an CallSiteParamAnnotationKey, which must be enclosed in a ArgPass
		// consumer, which Kind() method returns Conditional which is not deep. Thus, we pass false
		// here.
		site := newPrimitiveSite(*trigger.Controller, false)
		ts, ok := controlledTgsBySite[site]
		if !ok {
			ts = map[annotation.FullTrigger]bool{}
			controlledTgsBySite[site] = ts
		}
		ts[trigger] = true
	}
	e.controlledTriggersBySite = controlledTgsBySite

	for _, trigger := range triggers {
		// As the initial status, the controlled triggers are skipped and NilAway just pretends not
		// to see them. Those controlled triggers will be activated and encoded into the inference
		// map when the sites controlling them are assigned to proper values.
		if trigger.Controlled() {
			continue
		}
		e.buildFromSingleFullTrigger(trigger)
	}
}

func (e *Engine) buildFromSingleFullTrigger(trigger annotation.FullTrigger) {
	primitiveAssertion := fullTriggerAsPrimitive(e.pass, trigger)

	pKind, cKind := trigger.Producer.Annotation.Kind(), trigger.Consumer.Annotation.Kind()
	pSite, cSite := trigger.Producer.Annotation.UnderlyingSite(), trigger.Consumer.Annotation.UnderlyingSite()
	// NilAway does not know that (kind == Conditional || DeepConditional) => (site != nil),
	// so we have to add some redundant checks in the corresponding cases to give some hints.
	// TODO: remove this redundant check .
	switch {
	case pKind == annotation.Always && cKind == annotation.Always:
		// Producer always produces nilable value -> consumer always consumes nonnil value.
		// We simply generate a failure for this case.
		e.conflicts.AddSingleAssertionConflict(e.pass, trigger)

	case pKind == annotation.Always && (cKind == annotation.Conditional || cKind == annotation.DeepConditional):
		// Producer always produces nilable value -> consumer unknown.
		// We propagate nilable to this consumer site.
		if cSite == nil {
			panic("trigger is conditional but the underlying site is nil")
		}
		site := newPrimitiveSite(cSite, cKind == annotation.DeepConditional)
		e.observeSiteExplanation(site, TrueBecauseShallowConstraint{
			ExternalAssertion: primitiveAssertion,
		})

	case (pKind == annotation.Conditional || pKind == annotation.DeepConditional) && (cKind == annotation.Always):
		// Producer unknown -> consumer always consumes nonnil value.
		// We propagate nonnil to the producer site.
		if pSite == nil {
			panic("trigger is conditional but the underlying site is nil")
		}
		site := newPrimitiveSite(pSite, pKind == annotation.DeepConditional)
		e.observeSiteExplanation(site, FalseBecauseShallowConstraint{
			ExternalAssertion: primitiveAssertion,
		})

	case (pKind == annotation.Conditional || pKind == annotation.DeepConditional) &&
		(cKind == annotation.Conditional || cKind == annotation.DeepConditional):
		// Producer unknown -> consumer unknown.
		// We store this implication in our map as UndeterminedBool.
		if pSite == nil || cSite == nil {
			panic("trigger is conditional but the underlying site is nil")
		}
		producer := newPrimitiveSite(pSite, pKind == annotation.DeepConditional)
		consumer := newPrimitiveSite(cSite, cKind == annotation.DeepConditional)

		e.observeImplication(producer, consumer, primitiveAssertion)
	}
}

// observeSiteExplanation augments inferred map with a definite value for the passed
// site `site` - the definite value being given as the ExplainedBool `siteExplained`. Any conflicts
// encountered during the inference are stored internally and will be available when the inferred
// map is retrieved via `Engine.InferredMapWithDiagnostics`.There are three cases for what can happen when this
// call is made. If the site is not already mapped to an InferredVal of any kind, then a mapping to
// an DeterminedVal for the passed ExplainedBool is simply added - indicating that we now we have
// fixed the value of this site. If the site is already mapped to an DeterminedVal, then we check
// if that ExplainedBool agrees with the passed one. If it does, then the call is a no-op. If it
// does not, then we have discovered a site that is overconstrained to be both true and false by
// the program being analyzed, so we generate an overconstrainedConflict and append it to the
// internal failure list. Finally, if we discover that the site targeted by this call is currently
// mapped to an UndeterminedVal then we update the mapping to a definite DeterminedVal in accordance
// with the passed ExplainedBool, _and_ we walk the graph (forward if determining the site to be
// true (nilable), backwards if determining the site to be false (nonnil)), recursively calling
// observeSiteExplanation to determine all sites that must be determined from our knowledge of this
// call in the context of the current implication graph.
func (e *Engine) observeSiteExplanation(site primitiveSite, siteExplained ExplainedBool) {
	val, ok := e.inferredMap.Load(site)
	if !ok {
		e.storeDeterminedAndActivateControlledTriggers(site, siteExplained)
		return
	}
	if val == nil {
		panic(fmt.Sprintf("nil value stored in inferred map for site %v", site))
	}

	// If value exists in the annotation map, there are two cases:
	// (1) a determined value (*DeterminedVal) exists: we check if the new value agrees with the
	//     existing value and create failure if not.
	// (2) an undetermined value (*UndeterminedVal) exists: this site is now determined, and
	//     we should propagate this value to its implicates and implicants.
	switch v := val.(type) {
	case *DeterminedVal:
		if v.Bool.Val() == siteExplained.Val() {
			// No-op if the site is already mapped to an DeterminedVal that agrees with the
			// passed new value.
			return
		}

		// Otherwise, this site is overconstrained to be both nilable and nonnil. We create an
		// overconstrainedConflict and add it to the conflict list.
		trueExplanation, falseExplanation := v.Bool, siteExplained
		if !v.Bool.Val() {
			trueExplanation, falseExplanation = falseExplanation, trueExplanation
		}
		e.conflicts.AddOverconstraintConflict(trueExplanation, falseExplanation, e.pass)

		// Even though we have a conflict, we still need to make sure to activate any controlled
		// triggers that are waiting on this site, so that we would not miss processing any
		// triggers.
		e.activateControlledTriggers(site, siteExplained)

	case *UndeterminedVal:
		e.storeDeterminedAndActivateControlledTriggers(site, siteExplained)

		// Propagate the nilability of this site to its downstream constraints (for nilable value)
		// or its upstream constraints (for nonnil value).
		if siteExplained.Val() {
			for implicateSite, implicateAssertion := range v.Implicates {
				e.observeSiteExplanation(implicateSite, TrueBecauseDeepConstraint{
					InternalAssertion: implicateAssertion,
					DeeperExplanation: siteExplained,
				})
			}
		} else {
			for implicantSite, implicantAssertion := range v.Implicants {
				e.observeSiteExplanation(implicantSite, FalseBecauseDeepConstraint{
					InternalAssertion: implicantAssertion,
					DeeperExplanation: siteExplained,
				})
			}
		}

	}
}

// storeDeterminedAndActivateControlledTriggers stores the determined value for a site in the
// inference map and if the site has proper value, then all the triggers controlled by this site
// are also activated and will be used to build the inference map.
func (e *Engine) storeDeterminedAndActivateControlledTriggers(site primitiveSite, siteExplained ExplainedBool) {
	e.inferredMap.StoreDetermined(site, siteExplained)
	e.activateControlledTriggers(site, siteExplained)
}

// activateControlledTriggers checks if the site has proper value and activates all the triggers
// controlled by the site `site` if so. This method should be called whenever a site is determined
// to be a new value.
func (e *Engine) activateControlledTriggers(site primitiveSite, siteExplained ExplainedBool) {
	if controlledTgs, ok := e.controlledTriggersBySite[site]; ok && siteExplained.Val() {
		for tg := range controlledTgs {
			e.buildFromSingleFullTrigger(tg)
		}
	}
}

// observeImplication augments the inferred map with a new implication discovered as
// the result of an assertion. In particular, we note that all assertions discovered as FullTriggers
// by the assertions or affiliations analyzer are of the form `nilable X -> nilable Y`, so this
// method just takes the `producerSite` X and the `consumerSite` Y to represent the assertion/implication.
//
// There are then two types of significant mutations to the underlying inferred map that can be
// made because of this implication:
//
//   - If either one of the sites is currently determined (i.e. already determined to have a
//     definite nilability as a result of past observations) and that boolean value constrains the
//     other site (for producers, this means it is true (nilable) and for consumers, this means it
//     is false (nonnil)) then we add a definite value for the other site as determined by the
//     fixed boolean site and this assertion, via a call to observeSiteExplanation. Note that this
//     implication yields no useful information if either the existing producer is nonnil or the
//     existing consumer is nilable. Therefore, this method does nothing in such cases.
//
//   - If both sites are undetermined (i.e. both are under-constrained nodes in the implication graph)
//     then we simply ensure that this assertion is present as en edge between them.
func (e *Engine) observeImplication(
	producerSite,
	consumerSite primitiveSite,
	assertion primitiveFullTrigger,
) {
	// When we observe an implication between the producer site (PS) and consumer site (CS), we
	// check their existing values in the inferred map (denoted as P and C) and behave accordingly:
	// * If either P or C is determined, the other site will be determined. Note that we do not
	//   need to continue processing after either case since we are handling the same implication
	//   (i.e., no need to handle P => C and then C => P again). This also ensures that we do not
	//   report duplicate errors when P and C are both determined and have conflicting nilabilities.
	//   Specifically:
	//     * P is nilable => C must be nilable
	//     * P is nonnil => this implication does not yield more information which can be safely discarded
	//     * C is nilable => this implication does not yield more information which can be safely discarded
	//     * C is nonnil => P must be nonnil
	//
	// * If _both_ P and C are "undetermined or does not exist", we should create an implication
	//    edge between PS and CS.

	// Nilable (true) producer => Nilable (true) consumer. We do not care about "ok" here since
	// the "ok" in the type assertion below implies this "ok == true".
	producer, _ := e.inferredMap.Load(producerSite)
	if v, ok := producer.(*DeterminedVal); ok {
		if v.Bool.Val() {
			e.observeSiteExplanation(consumerSite, TrueBecauseDeepConstraint{
				InternalAssertion: assertion,
				DeeperExplanation: v.Bool,
			})
		}
		return
	}

	// Nonnil (false) consumer => Nonnil (false) producer. We do not care about "ok" here since
	// the "ok" in the type assertion below implies this "ok == true".
	consumer, _ := e.inferredMap.Load(consumerSite)
	if v, ok := consumer.(*DeterminedVal); ok {
		if !v.Bool.Val() {
			e.observeSiteExplanation(producerSite, FalseBecauseDeepConstraint{
				InternalAssertion: assertion,
				DeeperExplanation: v.Bool,
			})
		}
		return
	}

	// If we reach here, it means that the existing values for the producer and consumer are
	// undetermined (or non-existent), so we can simply add an implication edge in the graph.
	e.inferredMap.StoreImplication(producerSite, consumerSite, assertion)
}

// GobRegister must be called in an `init` function before attempting to run any procedure that can
// deal with InferredAnnotationMaps as Facts. If not, gob encoding/decoding will be unable to handle
// the data structures.
// The called function RegisterName maintains an internal mapping to ensure that the
// association between names and structs is bijective
func GobRegister() {
	var curr rune
	nextStr := func() string {
		out := string(curr)
		curr++
		if curr > 255 {
			panic("ERROR: too many strings requested")
		}
		return out
	}

	gob.RegisterName(nextStr(), &DeterminedVal{})
	gob.RegisterName(nextStr(), &UndeterminedVal{})
	gob.RegisterName(nextStr(), FalseBecauseShallowConstraint{})
	gob.RegisterName(nextStr(), FalseBecauseDeepConstraint{})
	gob.RegisterName(nextStr(), FalseBecauseAnnotation{})
	gob.RegisterName(nextStr(), TrueBecauseShallowConstraint{})
	gob.RegisterName(nextStr(), TrueBecauseDeepConstraint{})
	gob.RegisterName(nextStr(), TrueBecauseAnnotation{})

	gob.RegisterName(nextStr(), annotation.PtrLoadPrestring{})
	gob.RegisterName(nextStr(), annotation.MapAccessPrestring{})
	gob.RegisterName(nextStr(), annotation.MapWrittenToPrestring{})
	gob.RegisterName(nextStr(), annotation.SliceAccessPrestring{})
	gob.RegisterName(nextStr(), annotation.ChanAccessPrestring{})
	gob.RegisterName(nextStr(), annotation.FldAccessPrestring{})
	gob.RegisterName(nextStr(), annotation.UseAsErrorResultPrestring{})
	gob.RegisterName(nextStr(), annotation.FldAssignPrestring{})
	gob.RegisterName(nextStr(), annotation.GlobalVarAssignPrestring{})
	gob.RegisterName(nextStr(), annotation.ArgPassPrestring{})
	gob.RegisterName(nextStr(), annotation.InterfaceResultFromImplementationPrestring{})
	gob.RegisterName(nextStr(), annotation.MethodParamFromInterfacePrestring{})
	gob.RegisterName(nextStr(), annotation.UseAsReturnPrestring{})
	gob.RegisterName(nextStr(), annotation.SliceAssignPrestring{})
	gob.RegisterName(nextStr(), annotation.ArrayAssignPrestring{})
	gob.RegisterName(nextStr(), annotation.PtrAssignPrestring{})
	gob.RegisterName(nextStr(), annotation.MapAssignPrestring{})
	gob.RegisterName(nextStr(), annotation.DeepAssignPrimitivePrestring{})
	gob.RegisterName(nextStr(), annotation.ParamAssignDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.FuncRetAssignDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.VariadicParamAssignDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.FieldAssignDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.GlobalVarAssignDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.LocalVarAssignDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.ChanSendPrestring{})

	gob.RegisterName(nextStr(), annotation.TriggerIfNilablePrestring{})
	gob.RegisterName(nextStr(), annotation.TriggerIfDeepNilablePrestring{})
	gob.RegisterName(nextStr(), annotation.ProduceTriggerTautologyPrestring{})
	gob.RegisterName(nextStr(), annotation.ProduceTriggerNeverPrestring{})
	gob.RegisterName(nextStr(), annotation.PositiveNilCheckPrestring{})
	gob.RegisterName(nextStr(), annotation.NegativeNilCheckPrestring{})
	gob.RegisterName(nextStr(), annotation.ConstNilPrestring{})
	gob.RegisterName(nextStr(), annotation.NoVarAssignPrestring{})
	gob.RegisterName(nextStr(), annotation.FuncParamPrestring{})
	gob.RegisterName(nextStr(), annotation.VariadicFuncParamPrestring{})
	gob.RegisterName(nextStr(), annotation.TrustedFuncNilablePrestring{})
	gob.RegisterName(nextStr(), annotation.TrustedFuncNonnilPrestring{})
	gob.RegisterName(nextStr(), annotation.FldReadPrestring{})
	gob.RegisterName(nextStr(), annotation.FuncReturnPrestring{})
	gob.RegisterName(nextStr(), annotation.MethodReturnPrestring{})
	gob.RegisterName(nextStr(), annotation.MethodResultReachesInterfacePrestring{})
	gob.RegisterName(nextStr(), annotation.InterfaceParamReachesImplementationPrestring{})
	gob.RegisterName(nextStr(), annotation.GlobalVarReadPrestring{})
	gob.RegisterName(nextStr(), annotation.MapReadPrestring{})
	gob.RegisterName(nextStr(), annotation.SliceReadPrestring{})
	gob.RegisterName(nextStr(), annotation.ArrayReadPrestring{})
	gob.RegisterName(nextStr(), annotation.PtrReadPrestring{})
	gob.RegisterName(nextStr(), annotation.ChanRecvPrestring{})
	gob.RegisterName(nextStr(), annotation.FuncParamDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.VariadicFuncParamDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.FuncReturnDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.FldReadDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.LocalVarReadDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.GlobalVarReadDeepPrestring{})
	gob.RegisterName(nextStr(), annotation.GuardMissingPrestring{})
	gob.RegisterName(nextStr(), annotation.UseAsFldOfReturnPrestring{})
	gob.RegisterName(nextStr(), annotation.ArgFldPassPrestring{})
	gob.RegisterName(nextStr(), annotation.ParamFldReadPrestring{})
	gob.RegisterName(nextStr(), annotation.UnassignedFldPrestring{})
	gob.RegisterName(nextStr(), annotation.FldEscapePrestring{})
	gob.RegisterName(nextStr(), annotation.LocatedPrestring{})
	gob.RegisterName(nextStr(), annotation.UseAsErrorRetWithNilabilityUnknownPrestring{})
	gob.RegisterName(nextStr(), annotation.UseAsNonErrorRetDependentOnErrorRetNilabilityPrestring{})
	gob.RegisterName(nextStr(), annotation.MethodRecvPrestring{})
	gob.RegisterName(nextStr(), annotation.RecvPassPrestring{})
	gob.RegisterName(nextStr(), annotation.MethodRecvDeepPrestring{})
}
