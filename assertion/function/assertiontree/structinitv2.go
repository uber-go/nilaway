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

package assertiontree

// This file implements the allocation-site-sensitive struct initialization analysis (v2),
// gated behind the -struct-init-v2 flag.
//
// Core model: a struct value carries a "shape" — for each nilable field, whether that field is
// nil. A struct allocation (`&A{...}`, `A{...}`, `new(A)`, or the zero value `var a A`) is the
// source of truth for its shape. Field producers are attached to the flow-sensitive assertion tree,
// and later field dereferences use the existing field-access consumers.

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/guard"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/asthelper"
	"go.uber.org/nilaway/util/typeshelper"
)

// V2ParamFieldEffects is the package-level struct-init-v2 boundary summary computed once per
// package by ComputeParamFieldEffects. Every map is keyed by *types.Func, then by indexedFieldPathKey
// ("idx:path"). Writes drive the param-out (side-effect) machinery; the two read sets bound the
// field binding at boundaries so it enumerates only the field paths a boundary actually
// dereferences, never the full type graph.
type V2ParamFieldEffects struct {
	// Writes records (param idx, field path) pairs a function assigns to (e.g. `x.f = ...`, x a
	// parameter/receiver) — its param-out side effects. Transitively closed over forwarding edges.
	Writes map[*types.Func]map[string]bool
	// ParamReads records (param idx, field path) pairs a function dereferences of that parameter —
	// the demand a callee places on its caller's argument. Transitively closed over forwarding edges
	// (a pure forwarder inherits its forwardees' reads), so a caller binds exactly the field paths
	// the callee (and everything it forwards to) may dereference.
	ParamReads map[*types.Func]map[string]bool
	// ReturnReads records (result idx, field path) pairs that callers dereference of a function's
	// result — the demand callers place on a returned value, so a `return <var>` binds only those
	// paths. Collected at call sites; not transitively closed (under-report only).
	ReturnReads map[*types.Func]map[string]bool
}

// ComputeParamFieldEffects walks every function and method in the package once and records its
// struct-init-v2 boundary summary (see V2ParamFieldEffects): the param fields it writes, the param
// fields it dereferences, and the result fields its callers dereference. It is a read-only,
// package-level pre-pass (pure syntax/type inspection, no backpropagation).
//
// All three are gathered in a single AST walk. Writes use the local phase (`x.f = ...`, mirroring
// captureParamFieldWrite) plus the forwarding phase: every static call records an arg→param edge
// (which caller parameter, possibly at a nested field prefix, is passed as which callee parameter).
// Reads are gathered from selector bases — to evaluate `base.Sel`, base must be non-nil, so the
// field path of base is a read of whatever boundary value it roots at (a parameter → ParamReads,
// or a struct-returning-call result local → ReturnReads). ast.Inspect visits nested selectors, so
// every prefix of a deep access is recorded; writes contribute their container prefix the same way
// (the LHS `x.a.b` reads `a`). closeParamFieldSets then runs the same fixpoint over the shared
// edges for both Writes and ParamReads, so forwarders inherit their forwardees' effects.
//
// Cross-package and unresolvable (interface/func-value) callees contribute no edge and are treated
// as mutating/dereferencing nothing (under-report only).
func ComputeParamFieldEffects(pass *analysishelper.EnhancedPass) *V2ParamFieldEffects {
	writes := make(map[*types.Func]map[string]bool)
	reads := make(map[*types.Func]map[string]bool)
	returnReads := make(map[*types.Func]map[string]bool)
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
				case *ast.AssignStmt:
					collectDirectParamFieldWrites(pass, n, paramIdx, funcObj, writes)
				case *ast.CallExpr:
					collectParamForwardEdges(pass, n, paramIdx, funcObj, edges)
				case *ast.SelectorExpr:
					collectFieldReadDemand(pass, n, paramIdx, resultVars, funcObj, reads, returnReads)
				}
				return true
			})
		}
	}
	closeParamFieldSets(writes, edges)
	closeParamFieldSets(reads, edges)
	return &V2ParamFieldEffects{Writes: writes, ParamReads: reads, ReturnReads: returnReads}
}

// addFieldEffectKey records key in m[funcObj], allocating the inner set on first use. It reports whether
// the key was newly added.
func addFieldEffectKey(m map[*types.Func]map[string]bool, funcObj *types.Func, key string) bool {
	if m[funcObj] == nil {
		m[funcObj] = make(map[string]bool)
	}
	if m[funcObj][key] {
		return false
	}
	m[funcObj][key] = true
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
		callee := staticCalledFunc(pass, call)
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
func collectFieldReadDemand(pass *analysishelper.EnhancedPass, sel *ast.SelectorExpr, paramIdx map[*types.Var]int, resultVars map[*types.Var]structResultSource, funcObj *types.Func, reads, returnReads map[*types.Func]map[string]bool) {
	base, prefix := splitFieldChain(sel.X)
	if base == nil || prefix == "" {
		return
	}
	v, ok := pass.TypesInfo.ObjectOf(base).(*types.Var)
	if !ok {
		return
	}
	if idx, ok := paramIdx[v]; ok {
		addFieldEffectKey(reads, funcObj, indexedFieldPathKey(idx, prefix))
		return
	}
	if src, ok := resultVars[v]; ok {
		addFieldEffectKey(returnReads, src.callee, indexedFieldPathKey(src.idx, prefix))
	}
}

// collectDirectParamFieldWrites records, for the local phase of ComputeParamFieldEffects, each single-level
// field write `x.f = ...` whose base x is a parameter/receiver of funcObj (per paramIdx) and whose
// field f is nilable. Nested direct writes such as `x.f.g = ...` are not yet supported.
func collectDirectParamFieldWrites(pass *analysishelper.EnhancedPass, assign *ast.AssignStmt, paramIdx map[*types.Var]int, funcObj *types.Func, result map[*types.Func]map[string]bool) {
	for _, lhs := range assign.Lhs {
		sel, ok := lhs.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		baseIdent, ok := ast.Unparen(sel.X).(*ast.Ident)
		if !ok {
			continue
		}
		v, ok := pass.TypesInfo.ObjectOf(baseIdent).(*types.Var)
		if !ok {
			continue
		}
		idx, ok := paramIdx[v]
		if !ok {
			continue
		}
		field, ok := pass.TypesInfo.ObjectOf(sel.Sel).(*types.Var)
		if !ok || typeshelper.TypeBarsNilness(field.Type()) {
			continue
		}
		addFieldEffectKey(result, funcObj, indexedFieldPathKey(idx, field.Name()))
	}
}

// collectParamForwardEdges records, for the forwarding phase of ComputeParamFieldEffects, an arg→param
// edge for each argument (and the receiver) of call that resolves — through a field chain — to a
// parameter/receiver of funcObj (the function containing the call). Unresolvable or cross-package
// callees contribute no edge.
func collectParamForwardEdges(pass *analysishelper.EnhancedPass, call *ast.CallExpr, paramIdx map[*types.Var]int, funcObj *types.Func, edges map[*types.Func][]paramFieldForwardEdge) {
	callee := staticCalledFunc(pass, call)
	if callee == nil {
		return
	}
	sig, ok := callee.Type().(*types.Signature)
	if !ok {
		return
	}
	record := func(calleeIdx int, arg ast.Expr) {
		base, prefix := splitFieldChain(arg)
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
func closeParamFieldSets(fields map[*types.Func]map[string]bool, edges map[*types.Func][]paramFieldForwardEdge) {
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
				j, p, ok := parseIndexedFieldPathKey(ck)
				if !ok || j != e.calleeParamIdx {
					continue
				}
				path := joinFieldPath(e.callerPrefix, p)
				// Skip recursive field paths; otherwise forwarding can keep growing paths like
				// inner.f, inner.inner.f, ...
				if !pathSkipsRecursiveFields(f, e.callerParamIdx, path) {
					continue
				}
				if addFieldEffectKey(fields, f, indexedFieldPathKey(e.callerParamIdx, path)) {
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

// staticCalledFunc returns the *types.Func statically called by call, or nil if it cannot be
// resolved to a static function/method (interface, func value, builtin, dynamic). It intentionally
// uses raw go/types information in both the pre-pass and backprop paths because struct-init-v2
// summaries are keyed by source-level *types.Func objects. Anonymous/fake functions are not part of
// this boundary-summary model yet.
func staticCalledFunc(pass *analysishelper.EnhancedPass, call *ast.CallExpr) *types.Func {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		if f, ok := pass.TypesInfo.ObjectOf(fun).(*types.Func); ok {
			return f
		}
	case *ast.SelectorExpr:
		if f, ok := pass.TypesInfo.ObjectOf(fun.Sel).(*types.Func); ok {
			return f
		}
	}
	return nil
}

// splitFieldChain decomposes a field-chain expression into its base identifier and the dotted
// field prefix from that base: `x` -> (x, ""), `x.a` -> (x, "a"), `x.a.b` -> (x, "a.b"). It
// returns (nil, "") for anything whose innermost base is not a bare identifier (e.g. `(*x).a`,
// `f().a`), matching the single-level write detection's bare-base restriction.
func splitFieldChain(expr ast.Expr) (*ast.Ident, string) {
	expr = ast.Unparen(expr)
	var parts []string
	for {
		switch e := expr.(type) {
		case *ast.Ident:
			if len(parts) == 0 {
				return e, ""
			}
			for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
				parts[i], parts[j] = parts[j], parts[i]
			}
			return e, strings.Join(parts, ".")
		case *ast.SelectorExpr:
			parts = append(parts, e.Sel.Name)
			expr = ast.Unparen(e.X)
		default:
			return nil, ""
		}
	}
}

// parseIndexedFieldPathKey is the inverse of indexedFieldPathKey: it splits "idx:path"
// back into its parts.
func parseIndexedFieldPathKey(key string) (int, string, bool) {
	i := strings.IndexByte(key, ':')
	if i < 0 {
		return 0, "", false
	}
	idx, err := strconv.Atoi(key[:i])
	if err != nil {
		return 0, "", false
	}
	return idx, key[i+1:], true
}

// joinFieldPath concatenates a (possibly empty) field-path prefix with a sub-path: join("", p) = p,
// join("inner", "f") = "inner.f".
func joinFieldPath(prefix, sub string) string {
	if prefix == "" {
		return sub
	}
	return prefix + "." + sub
}

// pathSkipsRecursiveFields reports whether path can be followed without re-entering a struct
// type already seen on the chain. Recursive paths are skipped so the forwarding fixpoint stays
// finite.
func pathSkipsRecursiveFields(fn *types.Func, paramIdx int, path string) bool {
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

// structAllocation inspects expr and, if it allocates a struct value, returns the (deeply
// resolved) struct type together with the composite-literal element expressions (nil when the
// allocation has no explicit field initializers, e.g. `new(A)`). The boolean result reports
// whether expr is a struct allocation.
//
// Recognized forms: `A{...}`, `&A{...}`, and `new(A)`.
func (r *RootAssertionNode) structAllocation(expr ast.Expr) (*types.Struct, []ast.Expr, bool) {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return r.structAllocation(e.X)
		}
	case *ast.ParenExpr:
		return r.structAllocation(e.X)
	case *ast.CompositeLit:
		if structType := typeshelper.AsDeeplyStruct(r.Pass().TypesInfo.TypeOf(e)); structType != nil {
			return structType, e.Elts, true
		}
	case *ast.CallExpr:
		if ident, ok := e.Fun.(*ast.Ident); ok && r.ObjectOf(ident) == typeshelper.BuiltinNew {
			// new(A) yields a *A whose fields are all zero-valued (nil for nilable fields).
			if structType := typeshelper.AsDeeplyStruct(r.Pass().TypesInfo.TypeOf(e)); structType != nil {
				return structType, nil, true
			}
		}
	}
	return nil, nil, false
}

// emitAllocationShape attaches, for every nilable field of a struct value allocated on the
// RHS of an assignment and bound to lhsVal, a producer describing that field's nilability at
// this allocation site:
//   - a field with no initializer  -> StructFieldNil (definitely nil)
//   - a field initialized to expr e -> the (shallow) producer of e (e.g. nonnil for `&A{}` or
//     `new(A)`, nil for an explicit `nil`)
//
// If the RHS is instead a call to a struct-returning function, the value's fields are bound,
// symbolically, to that function's return context sites (see emitContextFieldProducers), so
// the nilability flows interprocedurally through inference.
//
// It must be called before the generic assignment handling produces lhsVal itself, because the
// latter detaches the lhsVal subtree (including the field nodes we target) once produced.
func (r *RootAssertionNode) emitAllocationShape(lhsVal, rhsVal ast.Expr) {
	if structType, fieldInits, ok := r.structAllocation(rhsVal); ok {
		r.emitFieldProducers(structType, fieldInits, lhsVal)
		return
	}

	// `lhs := f()` where f returns a single struct value: bind lhs's fields to f's return
	// context sites. (Multi-return calls are handled by the many-to-one assignment path.)
	if call, ok := ast.Unparen(rhsVal).(*ast.CallExpr); ok {
		if funcObj := staticCalledFunc(r.Pass(), call); funcObj != nil {
			sig := funcObj.Type().(*types.Signature)
			if sig.Results().Len() == 1 {
				if structType := typeshelper.AsDeeplyStruct(sig.Results().At(0).Type()); structType != nil {
					r.emitContextFieldProducers(structType, lhsVal, funcObj, annotation.StructFieldReturnContext, 0)
				}
			}
		}
	}
}

// shallowExprNilabilityProducer returns the producer encoding the nilability of the value of expr: an
// always-nil producer for an explicit `nil`, the (shallow) producer of a trackable/nilable
// expression, or Never for a value that cannot be nil (e.g. `&A{}`, `new(A)`).
func (r *RootAssertionNode) shallowExprNilabilityProducer(expr ast.Expr) annotation.ProducingAnnotationTrigger {
	if ident, ok := ast.Unparen(expr).(*ast.Ident); ok && r.isNil(ident) {
		return &annotation.ProduceTriggerTautology{}
	}
	if _, _, ok := r.structAllocation(expr); ok {
		return &annotation.ProduceTriggerNever{}
	}
	// Fall back to the generic producer parser for variables, fields, calls, and other
	// expressions whose nilability depends on existing annotations.
	if _, producers := r.ParseExprAsProducer(expr, true); len(producers) != 0 {
		return producers[0].GetShallow().Annotation
	}
	return &annotation.ProduceTriggerNever{}
}

// fieldInitNilabilityProducer returns the producer encoding the nilability of field i of a struct
// allocation with the given (possibly nil) field initializers.
func (r *RootAssertionNode) fieldInitNilabilityProducer(structType *types.Struct, fieldInits []ast.Expr, i int) annotation.ProducingAnnotationTrigger {
	field := structType.Field(i)
	fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), structType.NumFields(), i)
	if fieldVal == nil {
		// Omitted nilable field: nil at this allocation site.
		return &annotation.StructFieldNil{
			ProduceTriggerTautology: &annotation.ProduceTriggerTautology{},
			FieldName:               field.Name(),
		}
	}
	// Initialized field: its nilability is that of the assigned expression.
	return r.shallowExprNilabilityProducer(fieldVal)
}

// emitFieldProducers performs the per-field producer attachment described on
// emitAllocationShape for a concrete struct allocation. fieldInits may be nil.
//
// It is deep: a field initialized to a nested struct allocation (`&B{...}`, `new(B)`, `B{...}`),
// or an omitted value-struct field, has its own fields tracked recursively, so a chain like
// `a.b.c.x` resolves to the nilability established at allocation at every level — not just the
// first. Deeper paths are produced before shallower ones because AddProduction detaches the
// matched subtree; producing `a.b` first would remove the `a.b.c` node before we could attach to
// it.
func (r *RootAssertionNode) emitFieldProducers(structType *types.Struct, fieldInits []ast.Expr, base ast.Expr) {
	numFields := structType.NumFields()
	for i := range numFields {
		field := structType.Field(i)
		fieldSel := r.getSelectorExpr(field, base)
		fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), numFields, i)
		nilable := !typeshelper.TypeBarsNilness(field.Type())

		// Recurse to track this field's own (deeper) fields first.
		switch {
		case fieldVal != nil:
			// Field initialized to a nested allocation: track that allocation's shape.
			if innerType, innerInits, ok := r.structAllocation(fieldVal); ok {
				r.emitFieldProducers(innerType, innerInits, fieldSel)
			}
		case !nilable:
			// Omitted value-struct field: it is the zero struct, so its nilable sub-fields are
			// nil. (A pointer/interface field is nilable, handled by the producer below; we do not
			// recurse into it because the field itself is nil at this site.)
			if innerType := typeshelper.AsDeeplyStruct(field.Type()); innerType != nil {
				r.emitFieldProducers(innerType, nil, fieldSel)
			}
		}

		if !nilable {
			// The field value itself cannot be nil; only its sub-fields (handled above) matter.
			continue
		}
		r.AddProduction(&annotation.ProduceTrigger{
			Annotation: r.fieldInitNilabilityProducer(structType, fieldInits, i),
			Expr:       fieldSel,
		})
	}
}

// accessedFieldPath is one accessed field path under a boundary value, paired with the synthesized
// selector expression that reaches it.
type accessedFieldPath struct {
	sel  ast.Expr
	path string
}

// collectAccessedFieldPaths walks the live assertion subtree under node and collects accessed nilable
// field paths, deepest path first. prefix is the dotted path from the boundary value to node.
//
// seen holds the struct types already on the path. Recursion stops before following a field whose
// struct type already appeared on the chain, which keeps recursive struct types finite.
func (r *RootAssertionNode) collectAccessedFieldPaths(node AssertionNode, base ast.Expr, prefix string, seen map[*types.Struct]bool, out *[]accessedFieldPath) {
	for _, child := range node.Children() {
		fldNode, ok := child.(*fldAssertionNode)
		if !ok {
			continue
		}
		field := fldNode.decl
		sel := r.getSelectorExpr(field, base)
		path := field.Name()
		if prefix != "" {
			path = prefix + "." + field.Name()
		}
		// Recurse first because producing a shallow path detaches the subtree below it.
		if inner := typeshelper.AsDeeplyStruct(field.Type()); inner == nil || !seen[inner] {
			if inner != nil {
				seen[inner] = true
			}
			r.collectAccessedFieldPaths(child, sel, path, seen, out)
			if inner != nil {
				delete(seen, inner)
			}
		}
		if !typeshelper.TypeBarsNilness(field.Type()) {
			*out = append(*out, accessedFieldPath{sel: sel, path: path})
		}
	}
}

// emitContextFieldProducers attaches, for each accessed nilable field path under base, a producer
// making `base.<path>` nil iff the corresponding return/param context site of funcObj is inferred
// nilable. This is the caller-side read of a boundary summary. It is deep: any accessed nested path
// (e.g. `base.aptr.aptr`) is read from its own field-path site, so deep nilability flows across the
// boundary. structType is the boundary value's struct type (unused beyond documenting intent — the
// path set is taken from the live access subtree).
func (r *RootAssertionNode) emitContextFieldProducers(_ *types.Struct, base ast.Expr, funcObj *types.Func, kind annotation.StructFieldContextKind, index int) {
	path, _ := r.ParseExprAsProducer(base, false)
	node, _ := r.lookupPath(path)
	if node == nil {
		return
	}
	var paths []accessedFieldPath
	r.collectAccessedFieldPaths(node, base, "", make(map[*types.Struct]bool), &paths)
	for _, p := range paths {
		site := &annotation.StructFieldContextSite{
			FuncObj: funcObj, Kind: kind, Index: index, Path: p.path,
		}
		r.AddProduction(&annotation.ProduceTrigger{
			Annotation: &annotation.StructFieldFromContext{
				TriggerIfNilable: &annotation.TriggerIfNilable{Ann: site},
			},
			Expr: p.sel,
		})
	}
}

// emitReturnFieldContext binds, at a return statement, the fields of each struct-typed return
// value to that result's return context site, so the returned value's per-field nilability
// becomes the function's return summary (resolved by inference). Callee side.
//
// Error-return correlation: for a function that returns an error as its last result, a value
// returned alongside a definitely-non-nil error is never observed by a caller that checks
// `if err != nil { return }`. So at such a return we skip binding the value's field nilability —
// otherwise an error-path nil field (e.g. `return &A{}, err`) would wrongly poison the success-path
// summary. Returns with a nil or unknown error still bind. This mirrors how NilAway's generic
// error-return machinery conditions a result's nilability on the error result.
func (r *RootAssertionNode) emitReturnFieldContext(node *ast.ReturnStmt) {
	sig := r.FuncObj().Type().(*types.Signature)
	// Only handle the straightforward case where each result has its own return expression.
	// Naked returns and single-call spreads are currently unhandled
	if len(node.Results) != sig.Results().Len() {
		return
	}
	if typeshelper.FuncIsErrReturning(sig) {
		errExpr := node.Results[sig.Results().Len()-1]
		if _, definitelyNonNil := r.shallowExprNilabilityProducer(errExpr).(*annotation.ProduceTriggerNever); definitelyNonNil {
			return
		}
	}
	for retIdx, retExpr := range node.Results {
		structType := typeshelper.AsDeeplyStruct(sig.Results().At(retIdx).Type())
		if structType == nil {
			continue
		}
		r.bindValueFieldsToContext(r.FuncObj(), retExpr, structType, annotation.StructFieldReturnContext, retIdx)
	}
}

// bindValueFieldsToContext connects the fields of the value produced by valExpr to the
// context site of targetFunc at (kind, index). For returns, targetFunc is the current function;
// for arguments, it is the callee. For a trackable valExpr (a variable or field chain) it adds
// consumers on `valExpr.field` that the value's own shape producers will match; for an inline
// struct allocation it creates the full triggers directly; for a struct-returning call it links
// site-to-site (transitive).
func (r *RootAssertionNode) bindValueFieldsToContext(targetFunc *types.Func, valExpr ast.Expr, structType *types.Struct, kind annotation.StructFieldContextKind, index int) {
	funcObj := targetFunc

	// Transitive case: the value being bound is itself the result of a call (e.g. `return g()`).
	// A call result is not a trackable lvalue, so we handle it here and return without falling
	// through to the trackable/allocation paths. For a single struct result we link the
	// destination site to the callee's return site (site-to-site implication); other shapes
	// (multi-result, non-struct) are left for a later iteration.
	if call, ok := ast.Unparen(valExpr).(*ast.CallExpr); ok {
		if callee := staticCalledFunc(r.Pass(), call); callee != nil {
			calleeSig := callee.Type().(*types.Signature)
			if calleeSig.Results().Len() == 1 && typeshelper.AsDeeplyStruct(calleeSig.Results().At(0).Type()) != nil {
				numFields := structType.NumFields()
				for i := range numFields {
					field := structType.Field(i)
					if typeshelper.TypeBarsNilness(field.Type()) {
						continue
					}
					srcSite := &annotation.StructFieldContextSite{
						FuncObj: callee, Kind: annotation.StructFieldReturnContext, Index: 0, Path: field.Name(),
					}
					dstSite := &annotation.StructFieldContextSite{
						FuncObj: funcObj, Kind: kind, Index: index, Path: field.Name(),
					}
					r.AddNewTriggers(annotation.FullTrigger{
						Producer: &annotation.ProduceTrigger{
							Annotation: &annotation.StructFieldFromContext{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: srcSite}},
							Expr:       valExpr,
						},
						Consumer: &annotation.ConsumeTrigger{
							Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: dstSite}},
							Expr:       valExpr,
							Guards:     guard.NoGuards(),
						},
					})
				}
			}
		}
		return
	}

	trackablePath, _ := r.ParseExprAsProducer(valExpr, false)
	allocType, fieldInits, isAlloc := r.structAllocation(valExpr)

	// Inline allocation: bind its shape to the boundary site deeply (every nested path), so deep
	// nilability established at the allocation reaches the boundary.
	if isAlloc {
		r.bindAllocationFieldsToContext(allocType, fieldInits, valExpr, "", funcObj, kind, index)
		return
	}

	// Trackable value: bind only demanded field paths. Demand comes from the callee's param read set
	// or the callers' return read set, precomputed by ComputeParamFieldEffects. Each demanded path
	// gets a StructFieldToContext consumer on `value.<path>`.
	if trackablePath == nil {
		return
	}
	var demanded map[string]bool
	switch kind {
	case annotation.StructFieldParamContext:
		demanded = r.functionContext.v2ParamFieldEffects.ParamReads[funcObj]
	case annotation.StructFieldReturnContext:
		demanded = r.functionContext.v2ParamFieldEffects.ReturnReads[funcObj]
	}
	for ck := range demanded {
		j, path, ok := parseIndexedFieldPathKey(ck)
		if !ok || j != index || path == "" {
			continue
		}
		sel, ok := r.buildFieldPathSelector(valExpr, structType, path)
		if !ok {
			continue
		}
		site := &annotation.StructFieldContextSite{
			FuncObj: funcObj, Kind: kind, Index: index, Path: path,
		}
		r.AddConsumption(&annotation.ConsumeTrigger{
			Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site}},
			Expr:       sel,
			Guards:     guard.NoGuards(),
		})
	}
}

// bindAllocationFieldsToContext binds, deeply, the per-field nilability of an inline struct allocation to
// the boundary context site of funcObj at (kind, index). It mirrors emitFieldProducers' recursion
// over the allocation literal (which is finite), emitting one full trigger per nilable path:
// producer = the path's nilability at this allocation, consumer = the field-path context site.
// prefix is the dotted path accumulated so far.
func (r *RootAssertionNode) bindAllocationFieldsToContext(structType *types.Struct, fieldInits []ast.Expr, valExpr ast.Expr, prefix string, funcObj *types.Func, kind annotation.StructFieldContextKind, index int) {
	numFields := structType.NumFields()
	for i := range numFields {
		field := structType.Field(i)
		fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), numFields, i)
		nilable := !typeshelper.TypeBarsNilness(field.Type())
		path := field.Name()
		if prefix != "" {
			path = prefix + "." + field.Name()
		}

		switch {
		case fieldVal != nil:
			if innerType, innerInits, ok := r.structAllocation(fieldVal); ok {
				r.bindAllocationFieldsToContext(innerType, innerInits, valExpr, path, funcObj, kind, index)
			}
		case !nilable:
			if innerType := typeshelper.AsDeeplyStruct(field.Type()); innerType != nil {
				r.bindAllocationFieldsToContext(innerType, nil, valExpr, path, funcObj, kind, index)
			}
		}

		if !nilable {
			continue
		}
		site := &annotation.StructFieldContextSite{
			FuncObj: funcObj, Kind: kind, Index: index, Path: path,
		}
		r.AddNewTriggers(annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{Annotation: r.fieldInitNilabilityProducer(structType, fieldInits, i), Expr: valExpr},
			Consumer: &annotation.ConsumeTrigger{
				Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site}},
				Expr:       valExpr,
				Guards:     guard.NoGuards(),
			},
		})
	}
}

// emitZeroValueShape attaches StructFieldNil producers for the nilable fields of a struct
// value that is the zero value (e.g. `var a A`), for which every field is nil. It targets the
// field nodes currently attached under varNode (those are the fields actually accessed and thus
// pending resolution). It is called from the zero-value variable's default-trigger handling.
func (r *RootAssertionNode) emitZeroValueShape(varNode AssertionNode, base ast.Expr) {
	// Copy children first: AddProduction detaches matched nodes, which would mutate the slice
	// we are iterating.
	children := make([]AssertionNode, len(varNode.Children()))
	copy(children, varNode.Children())

	for _, child := range children {
		fldNode, ok := child.(*fldAssertionNode)
		if !ok {
			continue
		}
		if typeshelper.TypeBarsNilness(fldNode.decl.Type()) {
			continue
		}
		selExpr := r.getSelectorExpr(fldNode.decl, base)
		r.AddProduction(&annotation.ProduceTrigger{
			Annotation: &annotation.StructFieldNil{
				ProduceTriggerTautology: &annotation.ProduceTriggerTautology{},
				FieldName:               fldNode.decl.Name(),
			},
			Expr: selExpr,
		})
	}
}

// paramIndex returns the parameter index of v within the current function's signature, or the
// receiver index for the receiver, and whether v is a parameter/receiver at all.
func (r *RootAssertionNode) paramIndex(v *types.Var) (int, bool) {
	sig := r.FuncObj().Signature()
	if recv := sig.Recv(); recv != nil && recv == v {
		return annotation.ReceiverParamIndex, true
	}
	for i := range sig.Params().Len() {
		if sig.Params().At(i) == v {
			return i, true
		}
	}
	return 0, false
}

// emitParamFieldProducers attaches, at function entry, producers making each nilable field of
// a struct-typed parameter/receiver nil iff the corresponding parameter context site is inferred
// nilable. Callee side of the parameter boundary. builtExpr is the parameter identifier.
func (r *RootAssertionNode) emitParamFieldProducers(builtExpr ast.Expr) {
	ident, ok := builtExpr.(*ast.Ident)
	if !ok {
		return
	}
	v, ok := r.ObjectOf(ident).(*types.Var)
	if !ok {
		return
	}
	idx, ok := r.paramIndex(v)
	if !ok {
		return
	}
	structType := typeshelper.AsDeeplyStruct(v.Type())
	if structType == nil {
		return
	}
	r.emitContextFieldProducers(structType, builtExpr, r.FuncObj(), annotation.StructFieldParamContext, idx)
}

// ---------------------------------------------------------------------------
// Parameter side effects (caller-visible mutation of argument fields)
// ---------------------------------------------------------------------------
//
// When a callee writes to a field of one of its parameters (or receiver), the caller sees that
// field change after the call. v2 models this with a separate "param-out" context site per
// (function, param index, field path): the field's nilability as the function leaves it.
//
//   - Callee: each write `x.f = e` (x a param) binds e's nilability to the param-out site
//     (captureParamFieldWrite). A param field the function never writes is not in the write set,
//     so the caller leaves its existing argument field producer untouched.
//   - Caller: immediately after the call, each argument's fields are re-produced from the
//     callee's param-out sites (emitCallParamOutProductions). Because production detaches the
//     matched subtree, uses after the call read the param-out shape while earlier uses and any
//     post-call reassignment keep their own shapes. When the same variable is passed to several
//     parameters, the first argument's production is kept.
//
// This is a best-effort, straight-line model: the param-out value is taken from the last write
// in source order (the first write seen during backward traversal). Writes guarded by branches
// are not merged; those cases may under-report.

// indexedFieldPathKey is the map key identifying an indexed field-path summary entry.
func indexedFieldPathKey(idx int, path string) string {
	return fmt.Sprintf("%d:%s", idx, path)
}

// captureParamFieldWrite records, during backpropagation, an assignment whose left-hand side is
// a field of one of the current function's parameters/receiver, e.g. `x.f = e`. It binds e's
// nilability to the param-out context site for that field. Because backward traversal visits the
// last source-order write first, only the first capture per field is kept (it is the value the
// field holds when the function returns). lhs must be the assignment's LHS, rhs its RHS.
func (r *RootAssertionNode) captureParamFieldWrite(lhs, rhs ast.Expr) {
	sel, ok := lhs.(*ast.SelectorExpr)
	if !ok {
		return
	}
	baseIdent, ok := ast.Unparen(sel.X).(*ast.Ident)
	if !ok {
		return
	}
	v, ok := r.ObjectOf(baseIdent).(*types.Var)
	if !ok {
		return
	}
	idx, ok := r.paramIndex(v)
	if !ok {
		return
	}
	field, ok := r.ObjectOf(sel.Sel).(*types.Var)
	if !ok || typeshelper.TypeBarsNilness(field.Type()) {
		return
	}
	key := indexedFieldPathKey(idx, field.Name())
	if r.functionContext.v2ParamWrites[key] {
		// A later (in source order) write to this field was already captured; it determines the
		// exit value. Mark-only here so pass-through stays suppressed.
		return
	}
	r.functionContext.v2ParamWrites[key] = true

	site := &annotation.StructFieldContextSite{
		FuncObj: r.FuncObj(), Kind: annotation.StructFieldParamOutContext, Index: idx, Path: field.Name(),
	}
	r.AddNewTriggers(annotation.FullTrigger{
		Producer: &annotation.ProduceTrigger{Annotation: r.shallowExprNilabilityProducer(rhs), Expr: rhs},
		Consumer: &annotation.ConsumeTrigger{
			Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site}},
			Expr:       lhs,
			Guards:     guard.NoGuards(),
		},
	})
}

// emitCallParamOutProductions re-produces, at a call site, the fields a call may mutate for each
// struct-typed argument (and the receiver, for a method call) from the callee's param-out context
// sites. This is the caller-side read of the callee's side effects. Only fields the callee actually
// writes (per the package-level, transitively-closed write-set) are re-produced; fields the callee
// leaves untouched are not overridden, so the caller's own pre-call value flows naturally to uses
// after the call. It must run before emitCallArgBindings so that uses after the call attach to the
// param-out producers (and are detached) before the param-in consumers for the inbound value are
// added.
//
// The write-set is keyed by dotted field path (e.g. "inner.f" for a forwarded nested write), so a
// re-produced field may be nested: buildFieldPathSelector reaches it, and deeper paths are
// produced before shallower ones (AddProduction detaches the matched subtree).
func (r *RootAssertionNode) emitCallParamOutProductions(call *ast.CallExpr, funcIdent *ast.Ident) {
	funcObj, ok := r.ObjectOf(funcIdent).(*types.Func)
	if !ok {
		return
	}
	writes := r.functionContext.v2ParamFieldEffects.Writes[funcObj]
	if len(writes) == 0 {
		// Callee writes no parameter fields we can see (including any cross-package callee): the
		// call is transparent to argument fields; nothing to override.
		return
	}
	sig := funcObj.Signature()

	produce := func(idx int, argExpr ast.Expr, paramType types.Type) {
		structType := typeshelper.AsDeeplyStruct(paramType)
		if structType == nil {
			return
		}
		// Only a trackable argument (a variable or field chain) can carry post-call uses we need
		// to re-produce; an inline allocation has no later reads through a name.
		if trackable, _ := r.ParseExprAsProducer(argExpr, false); trackable == nil {
			return
		}
		// Collect this argument's written paths, deepest first: AddProduction detaches the matched
		// subtree, so producing a shallow path before a deeper one under it would drop the deeper
		// node prematurely.
		var paths []string
		for key := range writes {
			kIdx, path, ok := parseIndexedFieldPathKey(key)
			if !ok || kIdx != idx {
				continue
			}
			paths = append(paths, path)
		}
		sort.Slice(paths, func(i, j int) bool {
			return strings.Count(paths[i], ".") > strings.Count(paths[j], ".")
		})
		for _, path := range paths {
			sel, ok := r.buildFieldPathSelector(argExpr, structType, path)
			if !ok {
				continue
			}
			site := &annotation.StructFieldContextSite{
				FuncObj: funcObj, Kind: annotation.StructFieldParamOutContext, Index: idx, Path: path,
			}
			r.AddProduction(&annotation.ProduceTrigger{
				Annotation: &annotation.StructFieldFromContext{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: site}},
				Expr:       sel,
			})
		}
	}

	if recv := sig.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			produce(annotation.ReceiverParamIndex, sel.X, recv.Type())
		}
	}
	for argIdx, arg := range call.Args {
		if argIdx >= sig.Params().Len() {
			break
		}
		produce(argIdx, arg, sig.Params().At(argIdx).Type())
	}
}

// buildFieldPathSelector builds the (cached) selector expression reaching base.<path>, where path
// is a dotted field path under base's struct type structType (e.g. "inner.f"). It resolves each
// field along the path against the running struct type; it returns (nil, false) if any path segment
// is not a field of the current struct type (e.g. the path runs past a non-struct field).
func (r *RootAssertionNode) buildFieldPathSelector(base ast.Expr, structType *types.Struct, path string) (ast.Expr, bool) {
	cur := base
	curStruct := structType
	for _, name := range strings.Split(path, ".") {
		if curStruct == nil {
			return nil, false
		}
		var field *types.Var
		for i := range curStruct.NumFields() {
			if curStruct.Field(i).Name() == name {
				field = curStruct.Field(i)
				break
			}
		}
		if field == nil {
			return nil, false
		}
		cur = r.getSelectorExpr(field, cur)
		curStruct = typeshelper.AsDeeplyStruct(field.Type())
	}
	return cur, true
}

// emitForwardedParamOut links callee param-out sites to the current function's param-out sites
// when the current function forwards one of its own parameters/receiver to the callee.
func (r *RootAssertionNode) emitForwardedParamOut(call *ast.CallExpr, funcIdent *ast.Ident) {
	callee, ok := r.ObjectOf(funcIdent).(*types.Func)
	if !ok {
		return
	}
	calleeWrites := r.functionContext.v2ParamFieldEffects.Writes[callee]
	if len(calleeWrites) == 0 {
		return
	}
	caller := r.FuncObj()
	sig := callee.Signature()

	link := func(calleeIdx int, arg ast.Expr) {
		base, prefix := splitFieldChain(arg)
		if base == nil {
			return
		}
		v, ok := r.ObjectOf(base).(*types.Var)
		if !ok {
			return
		}
		callerIdx, ok := r.paramIndex(v)
		if !ok {
			return
		}
		for ck := range calleeWrites {
			j, p, ok := parseIndexedFieldPathKey(ck)
			if !ok || j != calleeIdx {
				continue
			}
			srcSite := &annotation.StructFieldContextSite{
				FuncObj: callee, Kind: annotation.StructFieldParamOutContext, Index: calleeIdx, Path: p,
			}
			dstSite := &annotation.StructFieldContextSite{
				FuncObj: caller, Kind: annotation.StructFieldParamOutContext, Index: callerIdx, Path: joinFieldPath(prefix, p),
			}
			r.AddNewTriggers(annotation.FullTrigger{
				Producer: &annotation.ProduceTrigger{
					Annotation: &annotation.StructFieldFromContext{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: srcSite}},
					Expr:       arg,
				},
				Consumer: &annotation.ConsumeTrigger{
					Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: dstSite}},
					Expr:       arg,
					Guards:     guard.NoGuards(),
				},
			})
		}
	}

	if recv := sig.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			link(annotation.ReceiverParamIndex, sel.X)
		}
	}
	for argIdx, arg := range call.Args {
		if argIdx >= sig.Params().Len() {
			break
		}
		link(argIdx, arg)
	}
}

// emitCallArgBindings binds, at a function or method call, the fields of each struct-typed
// argument (and, for a method call, the receiver) to the callee's corresponding parameter context
// site. Caller side of the parameter boundary.
func (r *RootAssertionNode) emitCallArgBindings(call *ast.CallExpr, funcIdent *ast.Ident) {
	funcObj, ok := r.ObjectOf(funcIdent).(*types.Func)
	if !ok {
		return
	}
	sig := funcObj.Signature()

	// Method call: bind the receiver value's fields to the callee's receiver parameter context.
	if recv := sig.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if structType := typeshelper.AsDeeplyStruct(recv.Type()); structType != nil {
				r.bindValueFieldsToContext(funcObj, sel.X, structType, annotation.StructFieldParamContext, annotation.ReceiverParamIndex)
			}
		}
	}

	for argIdx, arg := range call.Args {
		// Skip variadic spillover (and any arg/param count mismatch); handled in a later iteration.
		if argIdx >= sig.Params().Len() {
			break
		}
		structType := typeshelper.AsDeeplyStruct(sig.Params().At(argIdx).Type())
		if structType == nil {
			continue
		}
		r.bindValueFieldsToContext(funcObj, arg, structType, annotation.StructFieldParamContext, argIdx)
	}
}
