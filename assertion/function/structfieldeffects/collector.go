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

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/asthelper"
	"go.uber.org/nilaway/util/typeshelper"
)

// collectedFieldEffects owns the unclosed package summary and the forwarding state needed to close
// it after imported effects have been seeded.
type collectedFieldEffects struct {
	summary               *BoundaryFieldEffects
	paramForwardingEdges  map[*types.Func][]paramFieldForwardEdge
	returnForwardingEdges map[*types.Func][]returnForwardEdge
	// resultsWithConstructSite marks, per function, the result indices with at least one in-package
	// construction return site. Used to drop ambiguous mixed construct/forward param sources in close().
	resultsWithConstructSite map[*types.Func]map[int]bool
	// calledFunctions is the set of functions statically called from this package, whose facts
	// are imported before the collection is closed.
	calledFunctions map[*types.Func]bool
}

func newCollectedFieldEffects() *collectedFieldEffects {
	return &collectedFieldEffects{
		summary: &BoundaryFieldEffects{
			paramReads:         make(fieldEffects),
			returnReads:        make(fieldEffects),
			paramWrites:        make(fieldEffects),
			returnEffects:      make(fieldEffects),
			returnParamSources: make(returnParamSourceSet),
		},
		paramForwardingEdges:     make(map[*types.Func][]paramFieldForwardEdge),
		returnForwardingEdges:    make(map[*types.Func][]returnForwardEdge),
		resultsWithConstructSite: make(map[*types.Func]map[int]bool),
		calledFunctions:          make(map[*types.Func]bool),
	}
}

// addParamRead records that fn reads path from its parameter or receiver at fn's boundary.
func (c *collectedFieldEffects) addParamRead(fn *types.Func, p IndexedFieldPath) {
	c.summary.paramReads.add(fn, p)
}

// addReturnRead records read demand on callee's result — callee is the function whose returned
// value is dereferenced, not the function being collected.
func (c *collectedFieldEffects) addReturnRead(callee *types.Func, p IndexedFieldPath) {
	c.summary.returnReads.add(callee, p)
}

// addParamWrite records that fn writes path through its parameter or receiver.
func (c *collectedFieldEffects) addParamWrite(fn *types.Func, p IndexedFieldPath) {
	c.summary.paramWrites.add(fn, p)
}

// addReturnEffect records that fn's result field at p is provably nil at a construction return
// site.
func (c *collectedFieldEffects) addReturnEffect(fn *types.Func, p IndexedFieldPath) {
	c.summary.returnEffects.add(fn, p)
}

// addReturnParamSource records a source relating fn's result (or result field) to the parameter
// that supplies it.
func (c *collectedFieldEffects) addReturnParamSource(fn *types.Func, source ReturnParamSource) {
	c.summary.returnParamSources.add(fn, source)
}

// addParamForwardEdge records that fn passes one of its parameters (at a field prefix) as a
// callee argument.
func (c *collectedFieldEffects) addParamForwardEdge(fn *types.Func, e paramFieldForwardEdge) {
	c.paramForwardingEdges[fn] = append(c.paramForwardingEdges[fn], e)
}

// addReturnForwardEdge records that fn directly returns a callee's result.
func (c *collectedFieldEffects) addReturnForwardEdge(fn *types.Func, e returnForwardEdge) {
	c.returnForwardingEdges[fn] = append(c.returnForwardingEdges[fn], e)
}

// markCalled records that fn is statically called somewhere in this package, so fn's facts are
// imported before the collection is closed.
func (c *collectedFieldEffects) markCalled(fn *types.Func) {
	c.calledFunctions[fn] = true
}

// markResultWithConstructSite records that fn's resultIdx has an in-package construction return site,
// so close() can drop the ambiguous whole-result sources of mixed construct/forward results.
func (c *collectedFieldEffects) markResultWithConstructSite(fn *types.Func, resultIdx int) {
	if c.resultsWithConstructSite[fn] == nil {
		c.resultsWithConstructSite[fn] = make(map[int]bool)
	}
	c.resultsWithConstructSite[fn][resultIdx] = true
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
	fc, ok := newFunctionCollector(pass, fd, c)
	if !ok {
		return
	}
	fc.collectReturnSites()
	fc.collectUseEffects()
}

// functionCollector gathers one function declaration's boundary field effects. It owns the
// per-function state the collection passes share — the boundary indices and the classified
// locals — so the passes read it from the receiver instead of threading it through parameters.
type functionCollector struct {
	pass *analysishelper.EnhancedPass
	fd   *ast.FuncDecl
	// funcObj and sig identify the function being summarized. Effects on its own boundary are
	// keyed by funcObj, while return-read demand and the calledFunctions set are keyed by the callees
	// its body resolves.
	funcObj *types.Func
	sig     *types.Signature
	// paramIdx maps the receiver (annotation.ReceiverParamIndex) and each parameter to its
	// boundary index.
	paramIdx map[*types.Var]int
	// resultVars maps locals bound directly to a struct-returning call, so a later dereference
	// of the local's fields can be attributed to that callee's result (the return-read demand).
	resultVars map[*types.Var]structResultSource
	// allocationVars maps locals whose defining value is a concrete struct allocation, and
	allocationVars map[*types.Var]structAllocationSource
	// stableVars is the subset of allocation/result locals used only as bare return operands.
	stableVars map[*types.Var]bool
	// unstableParams holds the parameters whose value at a return may not be the caller's
	// argument (reassigned or address-taken), which never produce param sources.
	unstableParams map[*types.Var]bool
	// collected is the package-level collection every pass accumulates into.
	collected *collectedFieldEffects
}

// newFunctionCollector resolves fd's identity, boundary indices, and result-bound locals,
// reporting ok=false for declarations that resolve to no function object.
func newFunctionCollector(pass *analysishelper.EnhancedPass, fd *ast.FuncDecl, collected *collectedFieldEffects) (*functionCollector, bool) {
	funcObj, ok := pass.TypesInfo.ObjectOf(fd.Name).(*types.Func)
	if !ok {
		return nil, false
	}
	sig, ok := funcObj.Type().(*types.Signature)
	if !ok {
		return nil, false
	}

	paramIdx := make(map[*types.Var]int)
	if recv := sig.Recv(); recv != nil {
		paramIdx[recv] = annotation.ReceiverParamIndex
	}
	for i := range sig.Params().Len() {
		paramIdx[sig.Params().At(i)] = i
	}

	resultVars := collectStructResultVars(pass, fd.Body)

	return &functionCollector{
		pass:       pass,
		fd:         fd,
		funcObj:    funcObj,
		sig:        sig,
		paramIdx:   paramIdx,
		resultVars: resultVars,
		collected:  collected,
	}, true
}

// collectUseEffects records how the body uses boundary values, feeding each node kind of a
// single walk to its collection pass: assignments to param-field writes, calls to arg→param
// forwarding edges, and selectors to read demand.
func (fc *functionCollector) collectUseEffects() {
	// Selectors assigned to (`a.y = ...`) write their selected field rather than read it, so they
	// must not contribute value demand. ast.Inspect visits an AssignStmt before descending into its
	// LHS, so the set is always populated before the selector itself is visited.
	assignedSelectors := make(map[*ast.SelectorExpr]bool)
	ast.Inspect(fc.fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				if sel, ok := ast.Unparen(lhs).(*ast.SelectorExpr); ok {
					assignedSelectors[sel] = true
				}
			}
			fc.collectParamFieldWrites(n)
		case *ast.CallExpr:
			if callee := fc.collectParamForwardEdges(n); callee != nil {
				fc.collected.markCalled(callee)
			}
		case *ast.SelectorExpr:
			fc.collectFieldReadDemand(n, assignedSelectors[n])
		}
		return true
	})
}

// collectReturnSites gathers everything derived from this function's return statements: concrete
// nil result fields, return param sources, and return forwarding edges. One loop owns the site guards so
// the param-source and effect collections always see the same return sites — the mixed construct/forward
// drop in close() relies on that agreement.
func (fc *functionCollector) collectReturnSites() {
	// Fast path: no struct-shaped result can ever produce a concrete return effect, param source, or
	// forwarding edge, so the state preparation and walks below would do nothing.
	if !hasStructShapedResult(fc.sig) {
		return
	}
	fc.allocationVars = collectStructAllocationVars(fc.pass, fc.fd.Body)
	fc.stableVars = fc.collectStableStructVars()
	fc.unstableParams = fc.collectUnstableParamVars()

	for _, stmt := range returnStatements(fc.fd.Body) {
		// A bare spreading return such as `return f()` is one expression that yields every
		// result, so len(stmt.Results) is 1 while the signature may have many; we bail out
		// rather than try to split it, so such multi-result call returns are unsupported.
		if len(stmt.Results) != fc.sig.Results().Len() {
			continue
		}
		for resultIdx, resultExpr := range stmt.Results {
			if typeshelper.AsDeeplyStruct(fc.sig.Results().At(resultIdx).Type()) == nil {
				continue
			}
			fc.collectReturnSiteEffect(resultIdx, resultExpr)
			fc.collectWholeResultParamSource(resultIdx, resultExpr)
		}
	}
}

// returnStatements returns the return statements of body that return from the function being
// summarized; returns inside function literals belong to the literal, not the function.
func returnStatements(body *ast.BlockStmt) []*ast.ReturnStmt {
	var out []*ast.ReturnStmt
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		if s, ok := n.(*ast.ReturnStmt); ok {
			out = append(out, s)
			return false
		}
		return true
	})
	return out
}

// hasStructShapedResult reports whether any of sig's results is a struct or pointer-to-struct —
// the only results that can carry field-level return summaries.
func hasStructShapedResult(sig *types.Signature) bool {
	for result := range sig.Results().Variables() {
		if typeshelper.AsDeeplyStruct(result.Type()) != nil {
			return true
		}
	}
	return false
}

// structResultSource identifies the struct-returning callee and result index a local variable was
// assigned from, so dereferences of that local's fields can be attributed to the callee's return.
type structResultSource struct {
	callee typeshelper.StaticCallTarget
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
				out[v] = structResultSource{callee: target, idx: i}
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

// collectStructAllocationVars records locals whose defining value is a concrete struct allocation.
func collectStructAllocationVars(pass *analysishelper.EnhancedPass, body *ast.BlockStmt) map[*types.Var]structAllocationSource {
	var out map[*types.Var]structAllocationSource
	recordAllocation := func(ident *ast.Ident, source structAllocationSource) {
		v, ok := pass.TypesInfo.Defs[ident].(*types.Var)
		if !ok {
			return
		}
		if out == nil {
			out = make(map[*types.Var]structAllocationSource)
		}
		out[v] = source
	}
	recordIfAllocation := func(ident *ast.Ident, expr ast.Expr) {
		source, ok := staticStructAllocation(pass, expr)
		if ok {
			recordAllocation(ident, source)
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if len(n.Lhs) != len(n.Rhs) {
				return true
			}
			for i, lhs := range n.Lhs {
				if ident, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					recordIfAllocation(ident, n.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			if len(n.Values) == 0 {
				for _, ident := range n.Names {
					// `var x *A` declares a nil pointer, not a zero-valued struct: it has no
					// field shape to summarize
					if typeshelper.IsDeeplyType[*types.Pointer](pass.TypesInfo.TypeOf(ident)) {
						continue
					}
					if structType := typeshelper.AsDeeplyStruct(pass.TypesInfo.TypeOf(ident)); structType != nil {
						recordAllocation(ident, structAllocationSource{structType: structType})
					}
				}
				return true
			}
			if len(n.Names) != len(n.Values) {
				return true
			}
			for i, ident := range n.Names {
				recordIfAllocation(ident, n.Values[i])
			}
		case *ast.FuncLit:
			return false
		}
		return true
	})
	return out
}

// collectStableStructVars returns candidate locals whose only uses are as bare operands of explicit
// returns from the enclosing function. Starting from known-safe uses makes unfamiliar syntax
// conservative by default: any reassignment, mutation, alias, observation, escape, or closure
// capture is a non-return use and disqualifies the local.
func (fc *functionCollector) collectStableStructVars() map[*types.Var]bool {
	if len(fc.allocationVars) == 0 && len(fc.resultVars) == 0 {
		return nil
	}

	candidates := make(map[*types.Var]bool, len(fc.allocationVars)+len(fc.resultVars))
	for v := range fc.allocationVars {
		candidates[v] = true
	}
	for v := range fc.resultVars {
		candidates[v] = true
	}

	// Record exact identifier occurrences that are bare return operands. Returns inside closures do
	// not return from the function being summarized, so they are not allowed uses.
	allowedUses := make(map[*ast.Ident]bool)
	ast.Inspect(fc.fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			for _, result := range n.Results {
				if ident, ok := ast.Unparen(result).(*ast.Ident); ok {
					allowedUses[ident] = true
				}
			}
		}
		return true
	})

	// Only candidates that are actually returned start stable. The second walk intentionally enters
	// closures so a captured candidate is seen as a disallowed use.
	stable := make(map[*types.Var]bool)
	for ident := range allowedUses {
		if v, ok := fc.pass.TypesInfo.Uses[ident].(*types.Var); ok && candidates[v] {
			stable[v] = true
		}
	}

	// Reject a candidate if any of its uses is not one of the exact return occurrences recorded
	// above. Definitions are absent from TypesInfo.Uses, so they do not disqualify the candidate.
	ast.Inspect(fc.fd.Body, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		v, ok := fc.pass.TypesInfo.Uses[ident].(*types.Var)
		if ok && candidates[v] && !allowedUses[ident] {
			delete(stable, v)
		}
		return true
	})
	return stable
}

// collectReturnSiteEffect records what one struct-shaped return operand contributes to the
// effects summary: a construction (direct or through a stable local) enumerates its nil fields
// and marks the result index as constructed (see markResultWithConstructSite); a direct return of a
// static call result — or of a stable local bound to one — adds a return forwarding edge.
func (fc *functionCollector) collectReturnSiteEffect(resultIdx int, resultExpr ast.Expr) {
	if source, ok := staticStructAllocation(fc.pass, resultExpr); ok {
		fc.collected.markResultWithConstructSite(fc.funcObj, resultIdx)
		fc.enumerateConcreteReturnEffects(resultIdx, source.structType, source.fieldInits, annotation.FieldPath{})
		return
	}
	if call, ok := ast.Unparen(resultExpr).(*ast.CallExpr); ok {
		target, ok := typeshelper.ResolveStaticCallTarget(fc.pass.TypesInfo, call)
		if ok && target.Signature.Results().Len() == 1 {
			fc.addReturnEdge(resultIdx, target, 0)
		}
		return
	}
	ident, ok := ast.Unparen(resultExpr).(*ast.Ident)
	if !ok {
		return
	}
	v, ok := fc.pass.TypesInfo.ObjectOf(ident).(*types.Var)
	if !ok || !fc.stableVars[v] {
		return
	}
	if source, ok := fc.allocationVars[v]; ok {
		fc.collected.markResultWithConstructSite(fc.funcObj, resultIdx)
		fc.enumerateConcreteReturnEffects(resultIdx, source.structType, source.fieldInits, annotation.FieldPath{})
		return
	}
	if source, ok := fc.resultVars[v]; ok {
		fc.addReturnEdge(resultIdx, source.callee, source.idx)
	}
}

// addReturnEdge records a return forwarding edge when the callee resolves to an origin and its
// forwarded result is struct-shaped.
func (fc *functionCollector) addReturnEdge(callerResultIdx int, target typeshelper.StaticCallTarget, calleeResultIdx int) {
	if target.Origin == nil {
		return
	}
	if calleeResultIdx < 0 || calleeResultIdx >= target.Signature.Results().Len() ||
		typeshelper.AsDeeplyStruct(target.Signature.Results().At(calleeResultIdx).Type()) == nil {
		return
	}
	fc.collected.addReturnForwardEdge(fc.funcObj, returnForwardEdge{
		callerResultIdx: callerResultIdx,
		callee:          target.Origin,
		calleeResultIdx: calleeResultIdx,
	})
}

// enumerateConcreteReturnEffects records omitted and explicitly nil fields from one allocation,
// and the field-level param sources of fields initialized from parameters (recordFieldParamSource).
func (fc *functionCollector) enumerateConcreteReturnEffects(resultIdx int, structType *types.Struct, fieldInits []ast.Expr, prefix annotation.FieldPath) {
	for i := range structType.NumFields() {
		field := structType.Field(i)
		path := prefix.Child(field.Name())
		fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), structType.NumFields(), i)
		nilable := !typeshelper.TypeBarsNilness(field.Type())
		switch {
		case fieldVal == nil && !nilable:
			if innerType := typeshelper.AsDeeplyStruct(field.Type()); innerType != nil {
				fc.enumerateConcreteReturnEffects(resultIdx, innerType, nil, path)
			}
		case fieldVal == nil || fc.pass.IsNil(fieldVal):
			if nilable && resultFieldPathIsAcyclic(fc.funcObj, resultIdx, path) {
				fc.collected.addReturnEffect(fc.funcObj, IndexedFieldPath{Idx: resultIdx, Path: path})
			}
		default:
			fc.recordFieldParamSource(resultIdx, path, fieldVal)
			if source, ok := staticStructAllocation(fc.pass, fieldVal); ok {
				fc.enumerateConcreteReturnEffects(resultIdx, source.structType, source.fieldInits, path)
			}
		}
	}
}

// collectFieldReadDemand records the finite field-path demand implied by a single selector
// expression. There are two distinct demands:
//
//   - Base demand: to evaluate `base.Sel`, base must be non-nil, so the field path of base
//     (relative to the boundary value it roots at) is a read of that value. For `a.y.z` this
//     records `y`. A selector whose base is the boundary value itself (prefix "") only requires
//     the value's own top-level nilability, handled by the ordinary annotation machinery.
//   - Value demand: reading the selected field value itself may later flow through a local
//     snapshot (`w := a.y; return &D{v: w}`), so for a nilable selected field the full selector
//     path is recorded. This is what makes `a.y` demand `y`, not only deeper selectors like
//     `a.y.z`. Suppressed for assignment LHS selectors (skipValueDemand), which write rather
//     than read the selected field.
//
// If the root is a parameter/receiver of the function the demand goes to reads (param-in); if it
// roots at a local bound from a struct-returning call (resultVars) it goes to ReturnReads,
// attributed to that callee's result.
func (fc *functionCollector) collectFieldReadDemand(sel *ast.SelectorExpr, skipValueDemand bool) {
	record := func(base *ast.Ident, segments []string) {
		if base == nil || len(segments) == 0 {
			return
		}
		path := annotation.NewFieldPath(segments...)
		v, ok := fc.pass.TypesInfo.ObjectOf(base).(*types.Var)
		if !ok {
			return
		}
		if idx, ok := fc.paramIdx[v]; ok {
			fc.collected.addParamRead(fc.funcObj, IndexedFieldPath{Idx: idx, Path: path})
			return
		}
		if src, ok := fc.resultVars[v]; ok {
			fc.collected.addReturnRead(src.callee.Origin, IndexedFieldPath{Idx: src.idx, Path: path})
		}
	}

	// Base demand: `a.y.z` needs `a.y` to be non-nil.
	base, prefix := asthelper.SplitFieldChain(sel.X)
	record(base, prefix)

	// Value demand: `a.y` reads field `y` as a value. Only nilable field values need a field
	// nilability summary; non-nilable value-struct sub-fields are reached by their own later
	// selectors.
	field, ok := fc.pass.TypesInfo.ObjectOf(sel.Sel).(*types.Var)
	if skipValueDemand || !ok || typeshelper.TypeBarsNilness(field.Type()) {
		return
	}
	base, path := asthelper.SplitFieldChain(sel)
	record(base, path)
}

// collectParamFieldWrites records nilable fields assigned through a pointer parameter or receiver.
// The full path is retained for deep and explicit-deref writes. Recursive paths are skipped so the
// forwarding closure remains finite.
func (fc *functionCollector) collectParamFieldWrites(assign *ast.AssignStmt) {
	for _, lhs := range assign.Lhs {
		sel, ok := ast.Unparen(lhs).(*ast.SelectorExpr)
		if !ok {
			continue
		}
		field, ok := fc.pass.TypesInfo.ObjectOf(sel.Sel).(*types.Var)
		if !ok || typeshelper.TypeBarsNilness(field.Type()) {
			continue
		}
		base, segments := asthelper.SplitFieldChain(lhs)
		if base == nil || len(segments) == 0 {
			continue
		}
		path := annotation.NewFieldPath(segments...)
		param, ok := fc.pass.TypesInfo.ObjectOf(base).(*types.Var)
		if !ok {
			continue
		}
		idx, ok := fc.paramIdx[param]
		if !ok {
			continue
		}
		if _, ok := param.Type().Underlying().(*types.Pointer); !ok {
			continue
		}
		if !paramFieldPathIsAcyclic(fc.funcObj, idx, path) {
			continue
		}
		fc.collected.addParamWrite(fc.funcObj, IndexedFieldPath{Idx: idx, Path: path})
	}
}

// collectParamForwardEdges records an arg→param
// edge for each argument (and the receiver) of call that resolves — through a field chain — to a
// parameter/receiver of the function containing the call, and returns the resolved callee.
// Unresolvable (interface/func-value) callees contribute no edge or callee.
func (fc *functionCollector) collectParamForwardEdges(call *ast.CallExpr) *types.Func {
	target, ok := typeshelper.ResolveStaticCallTarget(fc.pass.TypesInfo, call)
	if !ok {
		return nil
	}
	record := func(calleeIdx int, arg ast.Expr) {
		base, prefix := asthelper.SplitFieldChain(arg)
		if base == nil {
			return
		}
		v, ok := fc.pass.TypesInfo.ObjectOf(base).(*types.Var)
		if !ok {
			return
		}
		callerIdx, ok := fc.paramIdx[v]
		if !ok {
			return
		}
		fc.collected.addParamForwardEdge(fc.funcObj, paramFieldForwardEdge{
			callerParamIdx: callerIdx,
			callerPrefix:   annotation.NewFieldPath(prefix...),
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
