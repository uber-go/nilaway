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

package structfieldeffects

import (
	"cmp"
	"go/types"
	"slices"
	"sort"
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/typeshelper"
)

// BoundaryFieldEffects is the package-level boundary summary. Every effect set is keyed by a
// function origin, then by IndexedFieldPath. Function declarations and StaticCallTarget both
// provide that identity, including for instantiated generic calls.
// The read sets bound the field binding at boundaries so it enumerates only the
// field paths a boundary actually dereferences, never the full type graph.
type BoundaryFieldEffects struct {
	// paramReads records (param idx, field path) pairs a function dereferences of that parameter —
	// the demand a callee places on its caller's argument. Transitively closed over forwarding edges
	// (a pure forwarder inherits its forwardees' reads), so a caller binds exactly the field paths
	// the callee (and everything it forwards to) may dereference.
	paramReads fieldEffects
	// returnReads records (result idx, field path) pairs that callers dereference of a function's
	// result — the demand callers place on a returned value, so a `return <var>` binds only those
	// paths. Collected at call sites; not transitively closed (under-report only).
	returnReads fieldEffects
	// paramWrites records (param idx, field path) pairs a function assigns through a parameter or
	// receiver. Transitively closed over forwarding edges.
	paramWrites fieldEffects
	// returnEffects records (result idx, field path) pairs that are provably nil at a concrete
	// construction return site. Closed over return forwarding edges.
	returnEffects fieldEffects
	// returnParamSources records, per function, the sources relating a result (or result field)
	// to the parameter that supplies it. See ReturnParamSource for the collection rules.
	returnParamSources returnParamSourceSet
}

// ParamReadPaths returns the field paths read from funcObj's parameter or receiver at idx.
func (e *BoundaryFieldEffects) ParamReadPaths(funcObj *types.Func, idx int) []string {
	if e == nil {
		return nil
	}
	return fieldPathsForIndex(e.paramReads, funcObj, idx)
}

// ReturnReadPaths returns the field paths read from funcObj's result at idx.
func (e *BoundaryFieldEffects) ReturnReadPaths(funcObj *types.Func, idx int) []string {
	if e == nil {
		return nil
	}
	return fieldPathsForIndex(e.returnReads, funcObj, idx)
}

// ReturnEffectPaths returns the field paths that are provably nil in funcObj's result at idx.
func (e *BoundaryFieldEffects) ReturnEffectPaths(funcObj *types.Func, idx int) []string {
	if e == nil {
		return nil
	}
	return fieldPathsForIndex(e.returnEffects, funcObj, idx)
}

// ParamWritePaths returns the field paths written through funcObj's parameter or receiver at idx.
func (e *BoundaryFieldEffects) ParamWritePaths(funcObj *types.Func, idx int) []string {
	if e == nil {
		return nil
	}
	return fieldPathsForIndex(e.paramWrites, funcObj, idx)
}

func fieldPathsForIndex(effects fieldEffects, funcObj *types.Func, idx int) []string {
	fields := effects[funcObj]
	paths := make([]string, 0, len(fields))
	for key := range fields {
		if key.Idx == idx && key.Path != "" {
			paths = append(paths, key.Path)
		}
	}

	// Sort paths so diagnostics at the same source location have a deterministic order.
	sort.Strings(paths)
	return paths
}

func (c *collectedFieldEffects) close() *BoundaryFieldEffects {
	closeParamFieldSets(c.summary.paramWrites, c.paramForwardingEdges)
	closeParamFieldSets(c.summary.paramReads, c.paramForwardingEdges)
	closeReturnEffects(c.summary.returnEffects, c.returnForwardingEdges)
	c.dropMixedResultParamSources()
	return c.summary
}

// seedImportedParamEffects merges an imported callee's parameter effects before closure runs.
func seedImportedParamEffects(effects fieldEffects, funcObj *types.Func, paths []IndexedFieldPath) {
	for _, path := range paths {
		if path.Path == "" {
			continue
		}
		effects.add(funcObj, path)
	}
}

// sortedPaths returns funcObj's field paths in a deterministic order. The paths come from a
// map-backed set and are serialized in an exported fact, so map iteration order must not
// affect the encoded fact.
func (e fieldEffects) sortedPaths(funcObj *types.Func) []IndexedFieldPath {
	if len(e[funcObj]) == 0 {
		return nil
	}
	paths := make([]IndexedFieldPath, 0, len(e[funcObj]))
	for key := range e[funcObj] {
		paths = append(paths, key)
	}
	slices.SortFunc(paths, IndexedFieldPath.compare)
	return paths
}

// IndexedFieldPath identifies a boundary value by parameter/result index and field path.
// For example, in an access to `a.b.c` where `a` is the first parameter, {Idx: 0,
// Path: "b"} represents the read demand on that parameter's `b` field.
type IndexedFieldPath struct {
	Idx  int
	Path string
}

// compare orders IndexedFieldPaths by index, then path. The boundary summaries are map-backed
// sets that reach an exported fact, so every consumer sorts with this ordering to keep map
// iteration order from leaking.
func (p IndexedFieldPath) compare(other IndexedFieldPath) int {
	return cmp.Or(
		cmp.Compare(p.Idx, other.Idx),
		cmp.Compare(p.Path, other.Path),
	)
}

// fieldEffects maps each function to a set of boundary field paths.
type fieldEffects map[*types.Func]map[IndexedFieldPath]bool

// add records key for funcObj, allocating the inner set on first use. It reports whether the key
// was newly added.
func (e fieldEffects) add(funcObj *types.Func, key IndexedFieldPath) bool {
	if e[funcObj] == nil {
		e[funcObj] = make(map[IndexedFieldPath]bool)
	}
	if e[funcObj][key] {
		return false
	}
	e[funcObj][key] = true
	return true
}

// paramFieldForwardEdge records that the caller passes its own parameter/receiver (callerParamIdx) — possibly
// at a nested field prefix callerPrefix (e.g. "inner" for g(x.inner)) — as the calleeParamIdx-th
// parameter (or receiver) of callee. It is the unit of the transitive field-effect closure: if
// callee has effect (calleeParamIdx, p), the caller inherits effect
// (callerParamIdx, callerPrefix+"."+p).
type paramFieldForwardEdge struct {
	callerParamIdx int
	callerPrefix   string
	callee         *types.Func
	calleeParamIdx int
}

// returnForwardEdge records that one result directly returns a callee result.
type returnForwardEdge struct {
	callerResultIdx int
	callee          *types.Func
	calleeResultIdx int
}

// closeParamFieldSets extends a field-effect set to a fixpoint over the forwarding edges:
// whenever a callee has effect (j, p) and a caller forwards its own param i (at prefix pre) as the
// callee's param j, the caller inherits effect (i, join(pre, p)). It is a standard worklist
// fixpoint; a function's key set only grows and is bounded by the reachable field paths, so cycles
// (recursion, mutual forwarding) converge. A predecessor index makes re-queueing on change cheap.
func closeParamFieldSets(fields fieldEffects, edges map[*types.Func][]paramFieldForwardEdge) {
	// preds[callee] lists every caller with an edge into callee, so a change to callee's field set
	// re-queues exactly the callers that could inherit from it.
	preds := make(map[*types.Func][]*types.Func)
	worklist := make([]*types.Func, 0, len(edges))
	inWork := make(map[*types.Func]bool, len(edges))
	for caller, es := range edges {
		worklist = append(worklist, caller)
		inWork[caller] = true
		for _, e := range es {
			preds[e.callee] = append(preds[e.callee], caller)
		}
	}

	for len(worklist) > 0 {
		f := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		inWork[f] = false

		changed := false
		for _, e := range edges[f] {
			for ck := range fields[e.callee] {
				if ck.Idx != e.calleeParamIdx {
					continue
				}
				path := joinFieldPath(e.callerPrefix, ck.Path)
				// Skip recursive field paths; otherwise forwarding can keep growing paths like
				// inner.f, inner.inner.f, ...
				if !paramFieldPathIsAcyclic(f, e.callerParamIdx, path) {
					continue
				}
				if fields.add(f, IndexedFieldPath{Idx: e.callerParamIdx, Path: path}) {
					changed = true
				}
			}
		}
		if changed {
			for _, pred := range preds[f] {
				if !inWork[pred] {
					worklist = append(worklist, pred)
					inWork[pred] = true
				}
			}
		}
	}
}

// closeReturnEffects copies concrete effects through direct return forwarding to a
// fixpoint. Paths are unchanged because field projections are not part of this boundary.
func closeReturnEffects(effects fieldEffects, edges map[*types.Func][]returnForwardEdge) {
	preds := make(map[*types.Func][]*types.Func)
	worklist := make([]*types.Func, 0, len(edges))
	inWork := make(map[*types.Func]bool, len(edges))
	for caller, callerEdges := range edges {
		worklist = append(worklist, caller)
		inWork[caller] = true
		for _, edge := range callerEdges {
			preds[edge.callee] = append(preds[edge.callee], caller)
		}
	}

	for len(worklist) > 0 {
		funcObj := worklist[len(worklist)-1]
		worklist = worklist[:len(worklist)-1]
		inWork[funcObj] = false

		changed := false
		for _, edge := range edges[funcObj] {
			for effect := range effects[edge.callee] {
				if effect.Idx != edge.calleeResultIdx ||
					!resultFieldPathIsAcyclic(funcObj, edge.callerResultIdx, effect.Path) {
					continue
				}
				if effects.add(funcObj, IndexedFieldPath{Idx: edge.callerResultIdx, Path: effect.Path}) {
					changed = true
				}
			}
		}
		if changed {
			for _, pred := range preds[funcObj] {
				if !inWork[pred] {
					worklist = append(worklist, pred)
					inWork[pred] = true
				}
			}
		}
	}
}

// joinFieldPath concatenates a (possibly empty) field-path prefix with a sub-path: join("", p) = p,
// join("inner", "f") = "inner.f".
func joinFieldPath(prefix, sub string) string {
	if prefix == "" {
		return sub
	}
	return prefix + "." + sub
}

func paramFieldPathIsAcyclic(fn *types.Func, paramIdx int, path string) bool {
	sig := fn.Signature()
	if paramIdx == annotation.ReceiverParamIndex {
		if sig.Recv() == nil {
			return false
		}
		return fieldPathIsAcyclic(sig.Recv().Type(), path)
	}
	if paramIdx < 0 || paramIdx >= sig.Params().Len() {
		return false
	}
	return fieldPathIsAcyclic(sig.Params().At(paramIdx).Type(), path)
}

func resultFieldPathIsAcyclic(fn *types.Func, resultIdx int, path string) bool {
	sig := fn.Signature()
	if resultIdx < 0 || resultIdx >= sig.Results().Len() {
		return false
	}
	return fieldPathIsAcyclic(sig.Results().At(resultIdx).Type(), path)
}

// fieldPathIsAcyclic reports whether path can be followed without re-entering a struct type already
// seen on the chain. Recursive paths are skipped so forwarding fixpoints remain finite.
func fieldPathIsAcyclic(boundaryType types.Type, path string) bool {
	// AsDeeplyStruct unwraps at most one pointer, but forwarded field paths can be rooted at
	// boundary values with multiple pointer layers.
	for {
		ptr, ok := boundaryType.Underlying().(*types.Pointer)
		if !ok {
			break
		}
		boundaryType = ptr.Elem()
	}
	st := typeshelper.AsDeeplyStruct(boundaryType)
	if st == nil {
		return false
	}
	// Seed seen with the boundary struct type so a field that points back to it (a self-recursive
	// field such as `A.inner *A`) is treated as recursive and skipped.
	seen := map[*types.Struct]bool{st: true}
	segs := strings.Split(path, ".")
	for i, name := range segs {
		var field *types.Var
		for k := range st.NumFields() {
			if st.Field(k).Name() == name {
				field = st.Field(k)
				break
			}
		}
		if field == nil {
			return false
		}
		if i == len(segs)-1 {
			return true
		}
		inner := typeshelper.AsDeeplyStruct(field.Type())
		if inner == nil || seen[inner] {
			return false
		}
		seen[inner] = true
		st = inner
	}
	return true
}
