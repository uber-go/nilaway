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

package assertiontree

import (
	"go/ast"
	"go/types"
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/asthelper"
	"go.uber.org/nilaway/util/typeshelper"
	"golang.org/x/tools/go/types/typeutil"
)

// ParamFieldEffects is the package-level boundary summary computed once per package by
// ComputeParamFieldEffects. Every effect set is keyed by *types.Func, then by indexedFieldPath.
// The read sets bound the field binding at boundaries so it enumerates only the
// field paths a boundary actually dereferences, never the full type graph.
type ParamFieldEffects struct {
	// ParamReads records (param idx, field path) pairs a function dereferences of that parameter —
	// the demand a callee places on its caller's argument. Transitively closed over forwarding edges
	// (a pure forwarder inherits its forwardees' reads), so a caller binds exactly the field paths
	// the callee (and everything it forwards to) may dereference.
	ParamReads fieldEffects
	// ReturnReads records (result idx, field path) pairs that callers dereference of a function's
	// result — the demand callers place on a returned value, so a `return <var>` binds only those
	// paths. Collected at call sites; not transitively closed (under-report only).
	ReturnReads fieldEffects
}

// ComputeParamFieldEffects walks every function and method in the package once and records the
// boundary summary as ParamFieldEffects: the param fields it dereferences and the result fields
// its callers dereference. It is a read-only, package-level pre-pass (pure
// syntax/type inspection, no backpropagation).
//
// Reads are gathered from selector bases — to evaluate `base.Sel`, base must be non-nil, so the
// field path of base is a read of whatever boundary value it roots at (a parameter → ParamReads,
// or a struct-returning-call result local → ReturnReads). ast.Inspect visits nested selectors, so
// every prefix of a deep access is recorded. Every static call also records an arg→param forwarding
// edge (which caller parameter, possibly at a nested field prefix, is passed as which callee
// parameter). closeParamFieldSets then runs a fixpoint over those edges so forwarders inherit their
// forwardees' param reads.
//
// Cross-package and unresolvable (interface/func-value) callees contribute no edge and are treated
// as mutating/dereferencing nothing (under-report only).
func ComputeParamFieldEffects(pass *analysishelper.EnhancedPass) *ParamFieldEffects {
	reads := make(fieldEffects)
	returnReads := make(fieldEffects)
	edges := make(map[*types.Func][]paramFieldForwardEdge)
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			funcObj, ok := pass.TypesInfo.ObjectOf(fd.Name).(*types.Func)
			if !ok {
				continue
			}
			sig, ok := funcObj.Type().(*types.Signature)
			if !ok {
				continue
			}
			paramIdx := make(map[*types.Var]int)
			if recv := sig.Recv(); recv != nil {
				paramIdx[recv] = annotation.ReceiverParamIndex
			}
			for i := range sig.Params().Len() {
				paramIdx[sig.Params().At(i)] = i
			}
			// Locals bound directly to a struct-returning call, so a later dereference of the local's
			// fields can be attributed to that callee's result (the return-read demand).
			resultVars := collectStructResultVars(pass, fd.Body)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch n := n.(type) {
				case *ast.CallExpr:
					collectParamForwardEdges(pass, n, paramIdx, funcObj, edges)
				case *ast.SelectorExpr:
					collectFieldReadDemand(pass, n, paramIdx, resultVars, funcObj, reads, returnReads)
				}
				return true
			})
		}
	}
	closeParamFieldSets(reads, edges)
	return &ParamFieldEffects{ParamReads: reads, ReturnReads: returnReads}
}

// indexedFieldPath identifies a boundary value by parameter/result index and field path.
// For example, in an access to `a.b.c` where `a` is the first parameter, {idx: 0,
// path: "b"} represents the read demand on that parameter's `b` field.
type indexedFieldPath struct {
	idx  int
	path string
}

// fieldEffects maps each function to the set of boundary field paths it reads.
type fieldEffects map[*types.Func]map[indexedFieldPath]bool

// add records key for funcObj, allocating the inner set on first use. It reports whether the key
// was newly added.
func (e fieldEffects) add(funcObj *types.Func, key indexedFieldPath) bool {
	if e[funcObj] == nil {
		e[funcObj] = make(map[indexedFieldPath]bool)
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
		callee := typeutil.StaticCallee(pass.TypesInfo, call)
		if callee == nil {
			return
		}
		sig, ok := callee.Type().(*types.Signature)
		if !ok || sig.Results().Len() != len(lhs) {
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
			if typeshelper.AsDeeplyStruct(sig.Results().At(i).Type()) == nil {
				continue
			}
			if _, seen := out[v]; !seen {
				out[v] = structResultSource{callee: callee, idx: i}
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
		reads.add(funcObj, indexedFieldPath{idx: idx, path: prefix})
		return
	}
	if src, ok := resultVars[v]; ok {
		returnReads.add(src.callee, indexedFieldPath{idx: src.idx, path: prefix})
	}
}

// collectParamForwardEdges records, for the forwarding phase of ComputeParamFieldEffects, an arg→param
// edge for each argument (and the receiver) of call that resolves — through a field chain — to a
// parameter/receiver of funcObj (the function containing the call). Unresolvable or cross-package
// callees contribute no edge.
func collectParamForwardEdges(pass *analysishelper.EnhancedPass, call *ast.CallExpr, paramIdx map[*types.Var]int, funcObj *types.Func, edges map[*types.Func][]paramFieldForwardEdge) {
	callee := typeutil.StaticCallee(pass.TypesInfo, call)
	if callee == nil {
		return
	}
	sig, ok := callee.Type().(*types.Signature)
	if !ok {
		return
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
			callee:         callee,
			calleeParamIdx: calleeIdx,
		})
	}
	if recv := sig.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			record(annotation.ReceiverParamIndex, sel.X)
		}
	}
	for argIdx, arg := range call.Args {
		if argIdx >= sig.Params().Len() {
			break
		}
		record(argIdx, arg)
	}
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
				if ck.idx != e.calleeParamIdx {
					continue
				}
				path := joinFieldPath(e.callerPrefix, ck.path)
				// Skip recursive field paths; otherwise forwarding can keep growing paths like
				// inner.f, inner.inner.f, ...
				if !fieldPathIsAcyclic(f, e.callerParamIdx, path) {
					continue
				}
				if fields.add(f, indexedFieldPath{idx: e.callerParamIdx, path: path}) {
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

// joinFieldPath concatenates a (possibly empty) field-path prefix with a sub-path: join("", p) = p,
// join("inner", "f") = "inner.f".
func joinFieldPath(prefix, sub string) string {
	if prefix == "" {
		return sub
	}
	return prefix + "." + sub
}

// fieldPathIsAcyclic reports whether path can be followed without re-entering a struct type already
// seen on the chain. Recursive paths are skipped so the forwarding fixpoint stays finite.
func fieldPathIsAcyclic(fn *types.Func, paramIdx int, path string) bool {
	sig := fn.Signature()
	var paramType types.Type
	if paramIdx == annotation.ReceiverParamIndex {
		if sig.Recv() == nil {
			return false
		}
		paramType = sig.Recv().Type()
	} else {
		if paramIdx < 0 || paramIdx >= sig.Params().Len() {
			return false
		}
		paramType = sig.Params().At(paramIdx).Type()
	}
	st := typeshelper.AsDeeplyStruct(paramType)
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
