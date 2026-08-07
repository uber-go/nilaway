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
	"bytes"
	"encoding/gob"
	"errors"

	"github.com/klauspost/compress/s2"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/orderedmap"
)

// An InferredMap is the state accumulated by multi-package inference. It's
// field `Mapping` maps a set of known annotation sites to InferredAnnotationVals - which can
// be either a fixed bool value along with explanation for why it was fixed - an DeterminedVal
// - or an UndeterminedVal indicating that site's place in the known implication graph
// between underconstrained sites. The set of sites mapped to UndeterminedBoolVals is guaranteed
// to be closed under following `Implicant`s and `Implicate`s pointers.
//
// Additionally, a field upstreamMapping is stored indicating a stable copy of the information
// gleaned from upstream packages. Both mapping and upstreamMapping are initially populated
// with the same informations, but observation functions (observeSiteExplanation and observeImplication)
// add information only to Mapping. On export, iterations combined with calls to
// inferredValDiff on shared keys is used to ensure that only
// information present in `Mapping` but not `UpstreamMapping` is exported, except for the upstream
// sites that must be forwarded for transitive fact propagation (see Export).
type InferredMap struct {
	primitive       *primitivizer
	upstreamMapping map[primitiveSite]InferredVal
	mapping         *orderedmap.OrderedMap[primitiveSite, InferredVal]
}

// newInferredMap returns a new, empty InferredMap.
func newInferredMap(primitive *primitivizer) *InferredMap {
	return &InferredMap{
		primitive:       primitive,
		upstreamMapping: make(map[primitiveSite]InferredVal),
		mapping:         orderedmap.New[primitiveSite, InferredVal](),
	}
}

// AFact allows InferredAnnotationMaps to be imported and exported via the Facts mechanism.
func (*InferredMap) AFact() {}

// Load returns the value stored in the map for an annotation site, or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (i *InferredMap) Load(site primitiveSite) (value InferredVal, ok bool) {
	return i.mapping.Load(site)
}

// StoreDetermined sets the inferred value for an annotation site.
func (i *InferredMap) StoreDetermined(site primitiveSite, value ExplainedBool) {
	i.mapping.Store(site, &DeterminedVal{Bool: value})
}

// StoreImplication stores an implication edge between the `from` and `to` annotation sites in the
// graph with the assertion for error reporting.
func (i *InferredMap) StoreImplication(from primitiveSite, to primitiveSite, assertion primitiveFullTrigger) {
	// First create UndeterminedVal in the map if it does not exist yet.
	for _, site := range [...]primitiveSite{from, to} {
		if _, ok := i.mapping.Load(site); !ok {
			i.mapping.Store(site, &UndeterminedVal{
				Implicates: orderedmap.New[primitiveSite, primitiveFullTrigger](),
				Implicants: orderedmap.New[primitiveSite, primitiveFullTrigger](),
			})
		}
	}

	i.mapping.Value(from).(*UndeterminedVal).Implicates.Store(to, assertion)
	i.mapping.Value(to).(*UndeterminedVal).Implicants.Store(from, assertion)
}

// Len returns the number of annotation sites currently stored in the map.
func (i *InferredMap) Len() int {
	return len(i.mapping.Pairs)
}

// OrderedRange calls f sequentially for each annotation site and inferred value present in the map
// in insertion order. If f returns false, range stops the iteration.
func (i *InferredMap) OrderedRange(f func(primitiveSite, InferredVal) bool) {
	for _, p := range i.mapping.Pairs {
		if !f(p.Key, p.Value) {
			return
		}
	}
}

// Export encodes the information this package contributes to multi-package inference as a
// package fact. For sites of this package it only encodes the exported (in the go sense; i.e.
// capitalized) ones and the sites needed to keep the implication graph between them convex (see
// chooseSitesToExport), and for sites already known upstream it only encodes the information this
// package adds on top of what upstream already said. This incremental encoding plays a _vital_
// role in minimizing build output.
//
// On top of that increment, Export _forwards_ the upstream information that packages importing us
// would otherwise never see: modular analyzer drivers (bazel/nogo, go vet / unitchecker) hand a
// package only the facts of its _direct_ imports, and x/tools does not re-export imported package
// facts (see golang/tools@d75c38746e), so a fact established two hops away would simply be lost.
// Forwarding is restricted to the upstream objects that remain reachable from this package's
// exported API (see primitivizer.upstreamAPISurface), which are precisely the upstream objects
// that a package importing us but not our dependencies could mention; forwarding everything
// instead would grow each package's fact with the entire transitive closure of its dependencies.
func (i *InferredMap) Export(pass *analysishelper.EnhancedPass) {
	if len(i.mapping.Pairs) == 0 {
		return
	}

	// First create a new map containing only the sites and their inferred values that we would
	// like to export.
	exported := orderedmap.New[primitiveSite, InferredVal]()
	sitesToExport := i.chooseSitesToExport()
	sitesToForward := i.chooseSitesToForward()
	for _, p := range i.mapping.Pairs {
		site, val := p.Key, p.Value
		if !sitesToExport[site] && !sitesToForward[site] {
			continue
		}

		if upstreamVal, upstreamPresent := i.upstreamMapping[site]; upstreamPresent {
			// Forwarded sites are exported in full, since downstream packages may not have access
			// to the upstream fact this value refines.
			if sitesToForward[site] {
				exported.Store(site, val)
				continue
			}
			diff, diffNonempty := inferredValDiff(val, upstreamVal)
			if diffNonempty && diff != nil {
				exported.Store(site, diff)
			}
		} else {
			exported.Store(site, val)
		}
	}

	if len(exported.Pairs) > 0 {
		// We do not need to encode the primitivizer since it is just a helper for the analysis of
		// the current package.
		m := newInferredMap(nil /* primitive */)
		m.mapping = exported

		pass.ExportPackageFact(m)
	}
}

// GobEncode encodes the inferred map via gob encoding.
func (i *InferredMap) GobEncode() (b []byte, err error) {
	var buf bytes.Buffer
	writer := s2.NewWriter(&buf)
	defer func() {
		if cerr := writer.Close(); cerr != nil {
			err = errors.Join(err, cerr)
		}
	}()

	if err := gob.NewEncoder(writer).Encode(i.mapping); err != nil {
		return nil, err
	}

	// Close the s2 writer before getting the bytes such that we have complete information.
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobDecode decodes the InferredMap from buffer.
func (i *InferredMap) GobDecode(input []byte) error {
	i.mapping = orderedmap.New[primitiveSite, InferredVal]()
	i.upstreamMapping = make(map[primitiveSite]InferredVal)

	buf := bytes.NewBuffer(input)
	return gob.NewDecoder(s2.NewReader(buf)).Decode(&i.mapping)
}

// chooseSitesToForward returns the set of sites imported from upstream packages whose (possibly
// locally refined) values must be re-exported by this package, namely the sites of upstream
// objects that are still reachable from this package's exported API. Packages importing us can
// only mention those upstream objects, and under modular drivers this package's fact is the only
// place they can learn about them (see Export).
func (i *InferredMap) chooseSitesToForward() map[primitiveSite]bool {
	// The primitivizer is nil for maps that were decoded from a fact; those are never exported.
	if len(i.upstreamMapping) == 0 || i.primitive == nil {
		return nil
	}

	// Collect the upstream packages we hold facts about, so that the API surface walk below only
	// pays for the object path encoding of objects that could possibly be matched.
	pkgPaths := make(map[string]bool)
	for site := range i.upstreamMapping {
		if site.ObjectPath != "" {
			pkgPaths[site.PkgPath] = true
		}
	}
	delete(pkgPaths, i.primitive.pass.Pkg.Path())
	if len(pkgPaths) == 0 {
		return nil
	}

	reachable := i.primitive.upstreamAPISurface(pkgPaths)
	if len(reachable) == 0 {
		return nil
	}

	toForward := make(map[primitiveSite]bool)
	for site := range i.upstreamMapping {
		if site.ObjectPath != "" && reachable[site.PkgPath+"."+string(site.ObjectPath)] {
			toForward[site] = true
		}
	}
	return toForward
}

// chooseSitesToExport returns the set of AnnotationSites mapped by this InferredMap that are both
// reachable from and that reach an Exported (in the go sense; i.e. capitalized) site. We define
// reachability  here to be reflexive, and we choose this definition so that the returned set is
// convex -guaranteeing that we never forget a semantically meaningful implication - yet minimal -
// containing no site that could be forgotten without sacrificing soundness
func (i *InferredMap) chooseSitesToExport() map[primitiveSite]bool {
	toExport := make(map[primitiveSite]bool)
	reachableFromExported := make(map[primitiveSite]bool)
	reachesExported := make(map[primitiveSite]bool)

	var markReachableFromExported func(site primitiveSite)
	markReachableFromExported = func(site primitiveSite) {
		val, _ := i.mapping.Load(site)
		if v, ok := val.(*UndeterminedVal); ok && !site.Exported && !toExport[site] && !reachableFromExported[site] {
			if reachesExported[site] {
				toExport[site] = true
			} else {
				reachableFromExported[site] = true
			}

			for _, p := range v.Implicates.Pairs {
				markReachableFromExported(p.Key)
			}
		}
	}

	var markReachesExported func(site primitiveSite)
	markReachesExported = func(site primitiveSite) {
		val, _ := i.mapping.Load(site)
		if v, ok := val.(*UndeterminedVal); ok && !site.Exported && !toExport[site] && !reachesExported[site] {
			if reachableFromExported[site] {
				toExport[site] = true
			} else {
				reachesExported[site] = true
			}

			for _, p := range v.Implicants.Pairs {
				markReachesExported(p.Key)
			}
		}
	}

	for _, p := range i.mapping.Pairs {
		site := p.Key
		if !site.Exported {
			continue
		}
		// Mark the current site as to be exported.
		toExport[site] = true

		// For UndeterminedVal, we visit the implicants and implicates recursively and mark
		// them as to be exported as well.
		if v, ok := i.mapping.Value(site).(*UndeterminedVal); ok {
			for _, p := range v.Implicants.Pairs {
				markReachesExported(p.Key)
			}
			for _, p := range v.Implicates.Pairs {
				markReachableFromExported(p.Key)
			}
		}
	}
	return toExport
}
