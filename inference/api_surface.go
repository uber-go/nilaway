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
// could still reach through our exported API. Objects are keyed by "<pkg path>.<object path>",
// which is exactly how primitive sites identify the object they annotate (see
// primitiveSite.ObjectPath), so the returned set can be matched against sites imported from
// upstream packages.
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
		primitive: p,
		local:     p.pass.Pkg,
		pkgPaths:  pkgPaths,
		objects:   make(map[string]bool),
		seen:      make(map[seenEntry]bool),
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
		w.visitType(obj.Type(), true /* out */)
	}

	return w.objects
}

// apiSurfaceWalker traverses the types and objects reachable from a package's exported API,
// recording the upstream objects it encounters. Traversal is guarded by a seen set, making it
// terminate on recursive types.
type apiSurfaceWalker struct {
	primitive *primitivizer
	// local is the package being analyzed; its own objects are never recorded.
	local *types.Package
	// pkgPaths is the set of upstream package paths worth recording objects for.
	pkgPaths map[string]bool
	// objects is the result set, keyed by "<pkg path>.<object path>".
	objects map[string]bool
	seen    map[seenEntry]bool
}

// seenEntry marks a type or an object as visited at a given polarity (see visitType).
type seenEntry struct {
	key any
	out bool
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
		w.objects[pkg.Path()+"."+string(path)] = true
	}
}

// visitType continues the traversal through the given type, recording the objects (methods,
// fields, type names) that an importer could reach from a value of that type. out reports whether
// values of this type flow out of our API (and can hence be inspected by the importer) rather
// than into it.
func (w *apiSurfaceWalker) visitType(t types.Type, out bool) {
	t = types.Unalias(t)
	if t == nil {
		return
	}
	entry := seenEntry{key: t, out: out}
	if w.seen[entry] {
		return
	}
	w.seen[entry] = true

	switch t := t.(type) {
	case *types.Pointer:
		w.visitType(t.Elem(), out)
	case *types.Slice:
		w.visitType(t.Elem(), out)
	case *types.Array:
		w.visitType(t.Elem(), out)
	case *types.Chan:
		w.visitType(t.Elem(), out)
	case *types.Map:
		w.visitType(t.Key(), out)
		w.visitType(t.Elem(), out)
	case *types.Struct:
		for i := range t.NumFields() {
			field := t.Field(i)
			// Unexported fields cannot be named by an importer. Embedded ones are still traversed,
			// since the methods and fields they promote _are_ reachable through them.
			if !field.Exported() {
				if field.Embedded() {
					w.visitType(field.Type(), out)
				}
				continue
			}
			if out {
				w.recordObject(field)
			}
			w.visitType(field.Type(), out)
		}
	case *types.Signature:
		w.visitSignature(t, out)
	case *types.Interface:
		// Interfaces are reachable in both polarities: values handed out can be inspected, and
		// interfaces asked for can be implemented by the importer, whose implementations are
		// related to the interface methods by NilAway's affiliation analysis.
		for i := range t.NumMethods() {
			method := t.Method(i)
			// An unexported method can neither be called nor implemented from another package,
			// which incidentally makes the whole interface unimplementable outside its own.
			if !method.Exported() {
				continue
			}
			w.recordObject(method)
			w.visitSignature(method.Signature(), out)
		}
	case *types.Named:
		if out {
			w.recordObject(t.Obj())
			for i := range t.NumMethods() {
				method := t.Method(i)
				if !method.Exported() {
					continue
				}
				w.recordObject(method)
				w.visitSignature(method.Signature(), out)
			}
		}
		for i := range t.TypeArgs().Len() {
			w.visitType(t.TypeArgs().At(i), out)
		}
		if origin := t.Origin(); origin != nil && origin != t {
			w.visitType(origin, out)
		}
		// Promoted methods and accessible fields are reached through the underlying type.
		w.visitType(t.Underlying(), out)
	case *types.TypeParam:
		w.visitType(t.Constraint(), out)
	}
}

// visitSignature traverses a function signature. Results keep the polarity of the signature,
// while parameters flip it: for a function the importer calls, results flow out and arguments
// flow in, and for one it implements or supplies, the reverse holds.
func (w *apiSurfaceWalker) visitSignature(sig *types.Signature, out bool) {
	for i := range sig.Results().Len() {
		w.visitType(sig.Results().At(i).Type(), out)
	}
	for i := range sig.Params().Len() {
		w.visitType(sig.Params().At(i).Type(), !out)
	}
	for i := range sig.TypeParams().Len() {
		w.visitType(sig.TypeParams().At(i), out)
	}
}
