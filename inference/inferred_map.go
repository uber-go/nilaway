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
	"go/types"
	"testing"

	"github.com/klauspost/compress/s2"
	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/orderedmap"
	"golang.org/x/tools/go/analysis"
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
// information present in `Mapping` but not `UpstreamMapping` is exported.
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

// Export only encodes new information not already present in the upstream maps, and it does not
// encode all (in the go sense; i.e. capitalized) annotation sites (See chooseSitesToExport).
// This ensures that only _incremental_ information is exported by this package and plays a _vital_
// role in minimizing build output.
func (i *InferredMap) Export(pass *analysis.Pass) {
	if len(i.mapping.Pairs) == 0 {
		return
	}

	// If we are testing, we encode and decode the inferred map to ensure that the gob encoding
	// works correctly (i.e., there are no un-registered types to Gob encoding).
	// This is a little hacky given that this should belong to the test logic instead of production
	// logic. However, our current analyzer architecture (i.e., the accumulation.Analyzer generates
	// the diagnostics and exports the facts, where the top-level nilaway.Analyzer only does the
	// reporting) prevents us from accessing the facts of accumulation.Analyzer in the test logic,
	// since facts are assumed to be "private" to an analysis. In the meantime, we do not want to
	// merge the accumulation.Analyzer and the top-level nilaway.Analyzer since the `analysistest`
	// framework will then require us to write "want" strings for facts as well.
	// We also encode/decode the entire map instead of the incremental map to have as much coverage
	// as possible.
	if testing.Testing() {
		var buf bytes.Buffer
		if err := gob.NewEncoder(&buf).Encode(i); err != nil {
			panic(err)
		}
		var m *InferredMap
		if err := gob.NewDecoder(&buf).Decode(&m); err != nil {
			panic(err)
		}
	}

	// First create a new map containing only the sites and their inferred values that we would
	// like to export.
	exported := orderedmap.New[primitiveSite, InferredVal]()
	sitesToExport := i.chooseSitesToExport()
	for _, p := range i.mapping.Pairs {
		site, val := p.Key, p.Value
		if !sitesToExport[site] {
			continue
		}

		if upstreamVal, upstreamPresent := i.upstreamMapping[site]; upstreamPresent {
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

// The following method implementations make InferredMap satisfy the annotation.Map
// interface, so that triggers can be checked against it.

// CheckFieldAnn checks this InferredMap for a concrete mapping of the field key provided
func (i *InferredMap) CheckFieldAnn(fld *types.Var) (annotation.Val, bool) {
	return i.checkAnnotationKey(&annotation.FieldAnnotationKey{FieldDecl: fld})
}

// CheckFuncParamAnn checks this InferredMap for a concrete mapping of the param key provided
func (i *InferredMap) CheckFuncParamAnn(fdecl *types.Func, num int) (annotation.Val, bool) {
	return i.checkAnnotationKey(annotation.ParamKeyFromArgNum(fdecl, num))
}

// CheckFuncRetAnn checks this InferredMap for a concrete mapping of the return key provided
func (i *InferredMap) CheckFuncRetAnn(fdecl *types.Func, num int) (annotation.Val, bool) {
	return i.checkAnnotationKey(annotation.RetKeyFromRetNum(fdecl, num))
}

// CheckFuncRecvAnn checks this InferredMap for a concrete mapping of the receiver key provided
func (i *InferredMap) CheckFuncRecvAnn(fdecl *types.Func) (annotation.Val, bool) {
	return i.checkAnnotationKey(&annotation.RecvAnnotationKey{FuncDecl: fdecl})
}

// CheckDeepTypeAnn checks this InferredMap for a concrete mapping of the type name key provideed
func (i *InferredMap) CheckDeepTypeAnn(name *types.TypeName) (annotation.Val, bool) {
	return i.checkAnnotationKey(&annotation.TypeNameAnnotationKey{TypeDecl: name})
}

// CheckGlobalVarAnn checks this InferredMap for a concrete mapping of the global variable key provided
func (i *InferredMap) CheckGlobalVarAnn(v *types.Var) (annotation.Val, bool) {
	return i.checkAnnotationKey(&annotation.GlobalVarAnnotationKey{VarDecl: v})
}

// CheckFuncCallSiteParamAnn checks this InferredMap for a concrete mapping of the call site param
// key provided.
func (i *InferredMap) CheckFuncCallSiteParamAnn(key *annotation.CallSiteParamAnnotationKey) (annotation.Val, bool) {
	return i.checkAnnotationKey(key)
}

// CheckFuncCallSiteRetAnn checks this InferredMap for a concrete mapping of the call site return
// key provided.
func (i *InferredMap) CheckFuncCallSiteRetAnn(key *annotation.CallSiteRetAnnotationKey) (annotation.Val, bool) {
	return i.checkAnnotationKey(key)
}

func (i *InferredMap) checkAnnotationKey(key annotation.Key) (annotation.Val, bool) {
	shallowKey := i.primitive.site(key, false)
	deepKey := i.primitive.site(key, true)

	shallowVal, shallowOk := i.mapping.Load(shallowKey)
	deepVal, deepOk := i.mapping.Load(deepKey)
	if !shallowOk || !deepOk {
		return annotation.EmptyVal, false
	}

	shallowBoolVal, shallowOk := shallowVal.(*DeterminedVal)
	deepBoolVal, deepOk := deepVal.(*DeterminedVal)
	if !shallowOk || !deepOk {
		return annotation.EmptyVal, false
	}

	return annotation.Val{
		IsNilable:        shallowBoolVal.Bool.Val(),
		IsDeepNilable:    deepBoolVal.Bool.Val(),
		IsNilableSet:     true,
		IsDeepNilableSet: true,
	}, true
}
