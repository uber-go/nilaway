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
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/asthelper"
	"go.uber.org/nilaway/util/typeshelper"
)

// BoundaryFieldEffects is the package-level boundary summary. Every effect set is keyed by a
// function origin, then by IndexedFieldPath. Function declarations and StaticCallTarget both
// provide that identity, including for instantiated generic calls.
// The read sets bound the field binding at boundaries so it enumerates only the
// field paths a boundary actually dereferences, never the full type graph.
type BoundaryFieldEffects struct {
	// ParamReads records (param idx, field path) pairs a function dereferences of that parameter —
	// the demand a callee places on its caller's argument. Transitively closed over forwarding edges
	// (a pure forwarder inherits its forwardees' reads), so a caller binds exactly the field paths
	// the callee (and everything it forwards to) may dereference.
	ParamReads fieldEffects
	// ReturnReads records (result idx, field path) pairs that callers dereference of a function's
	// result — the demand callers place on a returned value, so a `return <var>` binds only those
	// paths. Collected at call sites; not transitively closed (under-report only).
	ReturnReads fieldEffects
	// ParamWrites records (param idx, field path) pairs a function assigns through a parameter or
	// receiver. Transitively closed over forwarding edges.
	ParamWrites fieldEffects
	// ReturnEffects records (result idx, field path) pairs that are provably nil at a concrete
	// construction return site. Closed over same-package return forwarding edges.
	ReturnEffects fieldEffects
}

// ParamReadPaths returns the field paths read from funcObj's parameter or receiver at idx.
func (e *BoundaryFieldEffects) ParamReadPaths(funcObj *types.Func, idx int) []string {
	if e == nil {
		return nil
	}
	return fieldPathsForIndex(e.ParamReads, funcObj, idx)
}

// ReturnReadPaths returns the field paths read from funcObj's result at idx.
func (e *BoundaryFieldEffects) ReturnReadPaths(funcObj *types.Func, idx int) []string {
	if e == nil {
		return nil
	}
	return fieldPathsForIndex(e.ReturnReads, funcObj, idx)
}

// ParamWritePaths returns the field paths written through funcObj's parameter or receiver at idx.
func (e *BoundaryFieldEffects) ParamWritePaths(funcObj *types.Func, idx int) []string {
	if e == nil {
		return nil
	}
	return fieldPathsForIndex(e.ParamWrites, funcObj, idx)
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

// collectedFieldEffects owns the unclosed package summary and the forwarding state needed to close
// it after imported effects have been seeded.
type collectedFieldEffects struct {
	summary               *BoundaryFieldEffects
	paramForwardingEdges  map[*types.Func][]paramFieldForwardEdge
	returnForwardingEdges map[*types.Func][]returnForwardEdge
	callees               map[*types.Func]bool
}

func newCollectedFieldEffects() *collectedFieldEffects {
	return &collectedFieldEffects{
		summary: &BoundaryFieldEffects{
			ParamReads:    make(fieldEffects),
			ReturnReads:   make(fieldEffects),
			ParamWrites:   make(fieldEffects),
			ReturnEffects: make(fieldEffects),
		},
		paramForwardingEdges:  make(map[*types.Func][]paramFieldForwardEdge),
		returnForwardingEdges: make(map[*types.Func][]returnForwardEdge),
		callees:               make(map[*types.Func]bool),
	}
}

// computeBoundaryFieldEffects collects the unclosed boundary summary and forwarding state for every
// function and method in the package.
//
// Reads are gathered from selector bases — to evaluate `base.Sel`, base must be non-nil, so the
// field path of base is a read of whatever boundary value it roots at (a parameter → ParamReads,
// or a struct-returning-call result local → ReturnReads). ast.Inspect visits nested selectors, so
// every prefix of a deep access is recorded. Writes are gathered from assignment LHS field chains
// rooted at a parameter or receiver. Every static call also records an arg→param forwarding
// edge (which caller parameter, possibly at a nested field prefix, is passed as which callee
// parameter). Concrete struct returns contribute nil field paths, while direct returns of
// same-package call results contribute return-forwarding edges. Static callees are retained so the
// analyzer imports only facts needed by calls in this package before closing the collection.
//
// Unresolvable (interface/func-value) callees are treated as mutating/dereferencing nothing
// (under-report only).
func computeBoundaryFieldEffects(pass *analysishelper.EnhancedPass) *collectedFieldEffects {
	collected := newCollectedFieldEffects()
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			collected.collectFunction(pass, fd)
		}
	}
	return collected
}

func (c *collectedFieldEffects) collectFunction(pass *analysishelper.EnhancedPass, fd *ast.FuncDecl) {
	funcObj, ok := pass.TypesInfo.ObjectOf(fd.Name).(*types.Func)
	if !ok {
		return
	}
	sig, ok := funcObj.Type().(*types.Signature)
	if !ok {
		return
	}

	paramIdx := make(map[*types.Var]int)
	if recv := sig.Recv(); recv != nil {
		paramIdx[recv] = annotation.ReceiverParamIndex
	}
	for i := range sig.Params().Len() {
		paramIdx[sig.Params().At(i)] = i
	}

	// Locals bound directly to a struct-returning call, so a later dereference of the local's fields
	// can be attributed to that callee's result (the return-read demand).
	resultVars := collectStructResultVars(pass, fd.Body)
	c.collectReturnEffects(pass, fd, funcObj)

	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			collectParamFieldWrites(pass, n, paramIdx, funcObj, c.summary.ParamWrites)
		case *ast.CallExpr:
			if callee := collectParamForwardEdges(pass, n, paramIdx, funcObj, c.paramForwardingEdges); callee != nil {
				c.callees[callee] = true
			}
		case *ast.SelectorExpr:
			collectFieldReadDemand(
				pass, n, paramIdx, resultVars, funcObj,
				c.summary.ParamReads, c.summary.ReturnReads,
			)
		}
		return true
	})
}

// collectReturnEffects records this function's concrete nil result fields and same-package return
// forwarding edges. Only direct construction sites and direct same-package call returns are
// recognized; returned locals are handled by a later revision.
func (c *collectedFieldEffects) collectReturnEffects(pass *analysishelper.EnhancedPass, fd *ast.FuncDecl, funcObj *types.Func) {
	sig := funcObj.Signature()
	// Fast path: no struct-shaped result can ever produce a concrete return effect or forwarding
	// edge, so the body walk below would do nothing. Use the same predicate as
	// collectConcreteReturnEffects to avoid a narrower type check.
	hasStructResult := false
	for result := range sig.Results().Variables() {
		if typeshelper.AsDeeplyStruct(result.Type()) != nil {
			hasStructResult = true
			break
		}
	}
	if !hasStructResult {
		return
	}
	collectConcreteReturnEffects(
		pass, fd.Body, funcObj,
		c.summary.ReturnEffects, c.returnForwardingEdges,
	)
}

func (c *collectedFieldEffects) close() *BoundaryFieldEffects {
	closeParamFieldSets(c.summary.ParamWrites, c.paramForwardingEdges)
	closeParamFieldSets(c.summary.ParamReads, c.paramForwardingEdges)
	closeReturnEffects(c.summary.ReturnEffects, c.returnForwardingEdges)
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
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].Idx != paths[j].Idx {
			return paths[i].Idx < paths[j].Idx
		}
		return paths[i].Path < paths[j].Path
	})
	return paths
}

// IndexedFieldPath identifies a boundary value by parameter/result index and field path.
// For example, in an access to `a.b.c` where `a` is the first parameter, {Idx: 0,
// Path: "b"} represents the read demand on that parameter's `b` field.
type IndexedFieldPath struct {
	Idx  int
	Path string
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

// structResultSource identifies the struct-returning callee and result index a local variable was
// assigned from, so dereferences of that local's fields can be attributed to the callee's return.
type structResultSource struct {
	callee *types.Func
	idx    int
}

// collectStructResultVars maps each local variable bound directly from a static struct-returning call
// (`b := callee()`, `var b = callee()`, `b, err := callee()`) to the callee and result index it
// came from. Only struct (or pointer-to-struct) results are recorded. Used to attribute return-read
// demand to callees. A variable later reassigned is best-effort (first binding wins); a non-bare or
// cross-package callee is skipped (under-report).
func collectStructResultVars(pass *analysishelper.EnhancedPass, body *ast.BlockStmt) map[*types.Var]structResultSource {
	out := make(map[*types.Var]structResultSource)
	record := func(lhs, rhs []ast.Expr) {
		if len(rhs) != 1 {
			return
		}
		call, ok := ast.Unparen(rhs[0]).(*ast.CallExpr)
		if !ok {
			return
		}
		target, ok := typeshelper.ResolveStaticCallTarget(pass.TypesInfo, call)
		if !ok {
			return
		}
		if target.Signature.Results().Len() != len(lhs) {
			return
		}
		for i, l := range lhs {
			ident, ok := l.(*ast.Ident)
			if !ok {
				continue
			}
			v, ok := pass.TypesInfo.ObjectOf(ident).(*types.Var)
			if !ok {
				continue
			}
			if typeshelper.AsDeeplyStruct(target.Signature.Results().At(i).Type()) == nil {
				continue
			}
			if _, seen := out[v]; !seen {
				out[v] = structResultSource{callee: target.Origin, idx: i}
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			record(n.Lhs, n.Rhs)
		case *ast.ValueSpec:
			// `var b = callee()` / `var b1, b2 = callee()`: the names are the LHS.
			idents := make([]ast.Expr, len(n.Names))
			for i, nm := range n.Names {
				idents[i] = nm
			}
			record(idents, n.Values)
		}
		return true
	})
	return out
}

type structAllocationSource struct {
	structType *types.Struct
	fieldInits []ast.Expr
}

// staticStructAllocation recognizes struct values created directly by a composite literal or new.
func staticStructAllocation(pass *analysishelper.EnhancedPass, expr ast.Expr) (structAllocationSource, bool) {
	switch e := ast.Unparen(expr).(type) {
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return staticStructAllocation(pass, e.X)
		}
	case *ast.CompositeLit:
		if structType := typeshelper.AsDeeplyStruct(pass.TypesInfo.TypeOf(e)); structType != nil {
			return structAllocationSource{structType: structType, fieldInits: e.Elts}, true
		}
	case *ast.CallExpr:
		ident, ok := ast.Unparen(e.Fun).(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(ident) == typeshelper.BuiltinNew {
			if structType := typeshelper.AsDeeplyStruct(pass.TypesInfo.TypeOf(e)); structType != nil {
				return structAllocationSource{structType: structType}, true
			}
		}
	}
	return structAllocationSource{}, false
}

// collectConcreteReturnEffects records concrete nil result fields and same-package forwarding edges.
func collectConcreteReturnEffects(
	pass *analysishelper.EnhancedPass,
	body *ast.BlockStmt,
	funcObj *types.Func,
	effects fieldEffects,
	edges map[*types.Func][]returnForwardEdge,
) {
	sig := funcObj.Signature()
	addEdge := func(callerResultIdx int, target typeshelper.StaticCallTarget, calleeResultIdx int) {
		if target.Origin == nil || target.Origin.Pkg() != pass.Pkg {
			return
		}
		if calleeResultIdx < 0 || calleeResultIdx >= target.Signature.Results().Len() ||
			typeshelper.AsDeeplyStruct(target.Signature.Results().At(calleeResultIdx).Type()) == nil {
			return
		}
		edges[funcObj] = append(edges[funcObj], returnForwardEdge{
			callerResultIdx: callerResultIdx,
			callee:          target.Origin,
			calleeResultIdx: calleeResultIdx,
		})
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			// A bare spreading return such as `return f()` is one expression that yields every
			// result, so len(n.Results) is 1 while the signature may have many; we bail out
			// rather than try to split it, so such multi-result call returns are unsupported.
			if len(n.Results) != sig.Results().Len() {
				return false
			}
			for resultIdx, resultExpr := range n.Results {
				if typeshelper.AsDeeplyStruct(sig.Results().At(resultIdx).Type()) == nil {
					continue
				}
				if source, ok := staticStructAllocation(pass, resultExpr); ok {
					enumerateConcreteReturnEffects(pass, funcObj, resultIdx, source.structType, source.fieldInits, "", effects)
					continue
				}
				if call, ok := ast.Unparen(resultExpr).(*ast.CallExpr); ok {
					target, ok := typeshelper.ResolveStaticCallTarget(pass.TypesInfo, call)
					if ok && target.Signature.Results().Len() == 1 {
						addEdge(resultIdx, target, 0)
					}
					continue
				}
			}
			return false
		}
		return true
	})
}

// enumerateConcreteReturnEffects records omitted and explicitly nil fields from one allocation.
func enumerateConcreteReturnEffects(pass *analysishelper.EnhancedPass, funcObj *types.Func, resultIdx int, structType *types.Struct, fieldInits []ast.Expr, prefix string, effects fieldEffects) {
	for i := range structType.NumFields() {
		field := structType.Field(i)
		path := joinFieldPath(prefix, field.Name())
		fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), structType.NumFields(), i)
		nilable := !typeshelper.TypeBarsNilness(field.Type())
		switch {
		case fieldVal == nil && !nilable:
			if innerType := typeshelper.AsDeeplyStruct(field.Type()); innerType != nil {
				enumerateConcreteReturnEffects(pass, funcObj, resultIdx, innerType, nil, path, effects)
			}
		case fieldVal == nil || pass.IsNil(fieldVal):
			if nilable && resultFieldPathIsAcyclic(funcObj, resultIdx, path) {
				effects.add(funcObj, IndexedFieldPath{Idx: resultIdx, Path: path})
			}
		default:
			if source, ok := staticStructAllocation(pass, fieldVal); ok {
				enumerateConcreteReturnEffects(pass, funcObj, resultIdx, source.structType, source.fieldInits, path, effects)
			}
		}
	}
}

// collectFieldReadDemand records the field-path dereference demand implied by a single selector
// expression. To evaluate `base.Sel`, base must be non-nil, so the field path of base (relative to
// the boundary value it roots at) is a read of that value. If base roots at a parameter/receiver of
// funcObj the demand goes to reads (param-in); if it roots at a local bound from a struct-returning
// call (resultVars) it goes to returnReads, attributed to that callee's result. A selector whose
// base is the boundary value itself (prefix "") only requires the value's own top-level nilability,
// handled by the ordinary annotation machinery, so it is skipped here.
func collectFieldReadDemand(pass *analysishelper.EnhancedPass, sel *ast.SelectorExpr, paramIdx map[*types.Var]int, resultVars map[*types.Var]structResultSource, funcObj *types.Func, reads, returnReads fieldEffects) {
	base, prefix := asthelper.SplitFieldChain(sel.X)
	if base == nil || prefix == "" {
		return
	}
	v, ok := pass.TypesInfo.ObjectOf(base).(*types.Var)
	if !ok {
		return
	}
	if idx, ok := paramIdx[v]; ok {
		reads.add(funcObj, IndexedFieldPath{Idx: idx, Path: prefix})
		return
	}
	if src, ok := resultVars[v]; ok {
		returnReads.add(src.callee, IndexedFieldPath{Idx: src.idx, Path: prefix})
	}
}

// collectParamFieldWrites records nilable fields assigned through a pointer parameter or receiver.
// The full path is retained for deep and explicit-deref writes. Recursive paths are skipped so the
// forwarding closure remains finite.
func collectParamFieldWrites(pass *analysishelper.EnhancedPass, assign *ast.AssignStmt, paramIdx map[*types.Var]int, funcObj *types.Func, writes fieldEffects) {
	for _, lhs := range assign.Lhs {
		sel, ok := ast.Unparen(lhs).(*ast.SelectorExpr)
		if !ok {
			continue
		}
		field, ok := pass.TypesInfo.ObjectOf(sel.Sel).(*types.Var)
		if !ok || typeshelper.TypeBarsNilness(field.Type()) {
			continue
		}
		base, path := asthelper.SplitFieldChain(lhs)
		if base == nil || path == "" {
			continue
		}
		param, ok := pass.TypesInfo.ObjectOf(base).(*types.Var)
		if !ok {
			continue
		}
		idx, ok := paramIdx[param]
		if !ok {
			continue
		}
		if _, ok := param.Type().Underlying().(*types.Pointer); !ok {
			continue
		}
		if !paramFieldPathIsAcyclic(funcObj, idx, path) {
			continue
		}
		writes.add(funcObj, IndexedFieldPath{Idx: idx, Path: path})
	}
}

// collectParamForwardEdges records an arg→param
// edge for each argument (and the receiver) of call that resolves — through a field chain — to a
// parameter/receiver of funcObj (the function containing the call). Unresolvable (interface/func-value)
// callees contribute no edge or callee.
func collectParamForwardEdges(pass *analysishelper.EnhancedPass, call *ast.CallExpr, paramIdx map[*types.Var]int, funcObj *types.Func, edges map[*types.Func][]paramFieldForwardEdge) *types.Func {
	target, ok := typeshelper.ResolveStaticCallTarget(pass.TypesInfo, call)
	if !ok {
		return nil
	}
	record := func(calleeIdx int, arg ast.Expr) {
		base, prefix := asthelper.SplitFieldChain(arg)
		if base == nil {
			return
		}
		v, ok := pass.TypesInfo.ObjectOf(base).(*types.Var)
		if !ok {
			return
		}
		callerIdx, ok := paramIdx[v]
		if !ok {
			return
		}
		edges[funcObj] = append(edges[funcObj], paramFieldForwardEdge{
			callerParamIdx: callerIdx,
			callerPrefix:   prefix,
			callee:         target.Origin,
			calleeParamIdx: calleeIdx,
		})
	}
	if recv := target.Signature.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			record(annotation.ReceiverParamIndex, sel.X)
		}
	}
	for argIdx, arg := range call.Args {
		if argIdx >= target.Signature.Params().Len() {
			break
		}
		record(argIdx, arg)
	}
	return target.Origin
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

// returnForwardEdge records that one result directly returns a same-package callee result.
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

// closeReturnEffects copies concrete effects through direct same-package return forwarding to a
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
