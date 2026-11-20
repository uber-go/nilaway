//  Copyright (c) 2026 Uber Technologies, Inc.
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

import "go/types"

// upstreamAPISurface returns the set of upstream objects (i.e., objects owned by packages other
// than the one being analyzed) that a package importing us -- but not our own dependencies --
// could still reach through our exported API. Objects are keyed by upstreamObjKey, the same key
// derived from the sites imported from upstream packages (see primitiveSite.ObjectPath), so the
// returned set can be matched against those sites.
//
// This is the relevance criterion for _forwarding_ upstream inference facts (see
// InferredMap.Export). An importer can only make an assertion about an upstream object if it can
// name that object, which in turn requires a value of a type that our API hands out (or an
// interface it asks for). Upstream objects that our API does not expose are of no use to it:
// obtaining one requires importing the package that declares it, and importers get the facts of
// their own imports directly.
//
// The walk is polarity-aware. Values flowing _out_ of our API (results of exported functions,
// exported variables and fields, ...) can be inspected by the importer, so their methods and
// fields are reachable. Values flowing _in_ (parameters) cannot: the importer must have obtained
// them from the package that declares them, or from another package's API which -- by this very
// same rule -- forwards facts about them already. Interfaces are the exception in input position:
// an importer can implement one, which relates its own annotation sites to those of the interface
// methods.
//
// pkgPaths restricts recording to the upstream packages we actually hold facts about; objects of
// any other package are skipped without paying for object path encoding. Reachability is still
// explored _through_ those packages' types, since a type of interest may only be reachable
// through one that is not.
func (p *primitivizer) upstreamAPISurface(pkgPaths map[string]bool) map[string]bool {
	w := &apiSurfaceWalker{
		primitive:   p,
		local:       p.pass.Pkg,
		pkgPaths:    pkgPaths,
		objectPaths: make(map[string]bool),
		seenOut:     make(map[types.Type]bool),
		seenIn:      make(map[types.Type]bool),
	}

	// The exported package-level objects are the entry points of the exported API: everything an
	// importer can name is obtained by destructuring one of them.
	scope := p.pass.Pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj == nil || !obj.Exported() {
			continue
		}
		// The objects themselves belong to this package, so there is nothing to record for them:
		// facts about them are exported by this package in the first place.
		w.visitType(obj.Type(), true /* isOut */)
	}

	return w.objectPaths
}

// apiSurfaceWalker traverses the types and objects reachable from a package's exported API,
// recording the upstream objects it encounters. Traversal is guarded by seen sets, making it
// terminate on recursive types.
type apiSurfaceWalker struct {
	primitive *primitivizer
	// local is the package being analyzed; its own objects are never recorded.
	local *types.Package
	// pkgPaths is the set of upstream package paths worth recording objects for.
	pkgPaths map[string]bool
	// objectPaths is the result set, keyed by upstreamObjKey.
	objectPaths map[string]bool
	// seenOut and seenIn are the types already visited in output and input polarity, respectively
	// (see visitType).
	seenOut map[types.Type]bool
	seenIn  map[types.Type]bool
}

// recordObject records obj if it is an upstream object of interest. All annotation sites of the
// object are then forwarded, which is what we want: an importer that can name a method can pass
// arguments to it and consume its results alike.
func (w *apiSurfaceWalker) recordObject(obj types.Object) {
	pkg := obj.Pkg()
	if pkg == nil || pkg == w.local || !w.pkgPaths[pkg.Path()] {
		return
	}
	// Object paths only exist for objects reachable from their own package's exported API (which
	// is the case here, by construction). Sites of objects without one cannot be matched across
	// packages in the first place.
	if path := w.primitive.objectPath(obj); path != "" {
		w.objectPaths[upstreamObjKey(pkg.Path(), path)] = true
	}
}

// visitType continues the traversal through the given type, recording the objects (methods,
// fields, type names) that an importer could reach from a value of that type. isOut reports
// whether values of this type flow out of our API (and can hence be inspected by the importer)
// rather than into it.
func (w *apiSurfaceWalker) visitType(t types.Type, isOut bool) {
	unaliased := types.Unalias(t)
	if unaliased == nil {
		return
	}
	seen := w.seenIn
	if isOut {
		seen = w.seenOut
	}
	if seen[unaliased] {
		return
	}
	seen[unaliased] = true

	switch typ := unaliased.(type) {
	case *types.Pointer:
		w.visitType(typ.Elem(), isOut)
	case *types.Slice:
		w.visitType(typ.Elem(), isOut)
	case *types.Array:
		w.visitType(typ.Elem(), isOut)
	case *types.Chan:
		w.visitType(typ.Elem(), isOut)
	case *types.Map:
		w.visitType(typ.Key(), isOut)
		w.visitType(typ.Elem(), isOut)
	case *types.Struct:
		for i := range typ.NumFields() {
			field := typ.Field(i)
			// Unexported fields cannot be named by an importer. Embedded ones are still traversed,
			// since the methods and fields they promote _are_ reachable through them.
			if !field.Exported() {
				if field.Embedded() {
					w.visitType(field.Type(), isOut)
				}
				continue
			}
			if isOut {
				w.recordObject(field)
			}
			w.visitType(field.Type(), isOut)
		}
	case *types.Signature:
		w.visitSignature(typ, isOut)
	case *types.Interface:
		// Interfaces are reachable in both polarities: values handed out can be inspected, and
		// interfaces asked for can be implemented by the importer, whose implementations are
		// related to the interface methods by NilAway's affiliation analysis. Note that an
		// unexported method makes the whole interface unimplementable outside its own package.
		w.visitMethods(typ.NumMethods(), typ.Method, isOut)
	case *types.Named:
		if isOut {
			w.recordObject(typ.Obj())
			w.visitMethods(typ.NumMethods(), typ.Method, isOut)
		}
		for i := range typ.TypeArgs().Len() {
			w.visitType(typ.TypeArgs().At(i), isOut)
		}
		if origin := typ.Origin(); origin != nil && origin != typ {
			w.visitType(origin, isOut)
		}
		// Promoted methods and accessible fields are reached through the underlying type.
		w.visitType(typ.Underlying(), isOut)
	case *types.TypeParam:
		w.visitType(typ.Constraint(), isOut)
	}
}

// visitMethods records and traverses the exported methods of an interface or named type, given its
// method count and accessor. Unexported methods can neither be called nor implemented from another
// package, so they are skipped.
func (w *apiSurfaceWalker) visitMethods(numMethods int, methodAt func(int) *types.Func, isOut bool) {
	for i := range numMethods {
		method := methodAt(i)
		if !method.Exported() {
			continue
		}
		w.recordObject(method)
		w.visitSignature(method.Signature(), isOut)
	}
}

// visitSignature traverses a function signature. Results keep the polarity of the signature,
// while parameters flip it: for a function the importer calls, results flow out and arguments
// flow in, and for one it implements or supplies, the reverse holds.
func (w *apiSurfaceWalker) visitSignature(sig *types.Signature, isOut bool) {
	for i := range sig.Results().Len() {
		w.visitType(sig.Results().At(i).Type(), isOut)
	}
	for i := range sig.Params().Len() {
		w.visitType(sig.Params().At(i).Type(), !isOut)
	}
	for i := range sig.TypeParams().Len() {
		w.visitType(sig.TypeParams().At(i), isOut)
	}
}
