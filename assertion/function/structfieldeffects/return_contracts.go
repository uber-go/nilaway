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

// This file owns return param-source collection: the syntactic result <- parameter relations
// (ReturnParamSource) recorded per function. The rest of the collector integrates through
// three hooks — collectWholeResultParamSource for whole-result sources, recordFieldParamSource for
// construction fields, and dropMixedResultParamSources in close(). The return-site iteration that
// feeds collectWholeResultParamSource lives in collectReturnSites.

package structfieldeffects

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/asthelper"
	"go.uber.org/nilaway/util/typeshelper"
)

// ReturnParamSource records a result (or result field) whose value is the caller's own argument:
// the Result endpoint is supplied by the Param endpoint (the receiver uses
// annotation.ReceiverParamIndex). A root Result path denotes the whole result value
// (`return p`, `return p.x`); a root Param path the bare parameter. Collection is syntactic
// and conservative: a source is recorded only when the relation holds on every path to the return
// (see the drops in collectUnstableParamVars and dropMixedResultParamSources). Collected so later
// revisions can resolve param-sourced results from the actual argument at each call site instead of the
// shared return summary, severing one caller's argument from every other caller's result.
type ReturnParamSource struct {
	Result IndexedFieldPath
	Param  IndexedFieldPath
}

// ReturnParamSources is the set of return parameter sources for a function.
type ReturnParamSources []ReturnParamSource

// paramPathFromResultPath maps a result path covered by s to the corresponding parameter path. For example,
// `Mid <- p.inner` maps result path `Mid.Child` to parameter path `inner.Child`.
func (s ReturnParamSource) paramPathFromResultPath(resultIdx int, resultPath annotation.FieldPath) (IndexedFieldPath, bool) {
	if s.Result.Idx != resultIdx {
		return IndexedFieldPath{}, false
	}
	suffix, ok := resultPath.TrimPrefix(s.Result.Path)
	if !ok {
		return IndexedFieldPath{}, false
	}
	return IndexedFieldPath{Idx: s.Param.Idx, Path: s.Param.Path.Join(suffix)}, true
}

// returnParamSourceSet maps each function to its set of return param sources.
type returnParamSourceSet map[*types.Func]map[ReturnParamSource]bool

// add records source for funcObj, allocating the inner set on first use. It reports whether
// the source was newly added.
func (s returnParamSourceSet) add(funcObj *types.Func, source ReturnParamSource) bool {
	if s[funcObj] == nil {
		s[funcObj] = make(map[ReturnParamSource]bool)
	}
	if s[funcObj][source] {
		return false
	}
	s[funcObj][source] = true
	return true
}

// sortedSources returns funcObj's return param sources in a deterministic order. The sources come
// from a map-backed set and reach an exported fact and trigger emission, so map iteration order
// must not affect either.
func (s returnParamSourceSet) sortedSources(funcObj *types.Func) []ReturnParamSource {
	if len(s[funcObj]) == 0 {
		return nil
	}
	sources := make([]ReturnParamSource, 0, len(s[funcObj]))
	for c := range s[funcObj] {
		sources = append(sources, c)
	}
	slices.SortFunc(sources, func(a, b ReturnParamSource) int {
		return cmp.Or(
			a.Result.compare(b.Result),
			a.Param.compare(b.Param),
		)
	})
	return sources
}

// ReturnParamSources returns funcObj's return param sources in the sortedSources order.
func (e *BoundaryFieldEffects) ReturnParamSources(funcObj *types.Func) ReturnParamSources {
	if e == nil {
		return nil
	}
	return e.returnParamSources.sortedSources(funcObj)
}

// HasSource reports whether any parameter source covers resultPath of resultIdx. A root source
// covers the whole result, while a field source covers its field and descendants.
func (s ReturnParamSources) HasSource(resultIdx int, resultPath annotation.FieldPath) bool {
	for _, source := range s {
		if source.Result.Idx != resultIdx {
			continue
		}
		if _, ok := resultPath.TrimPrefix(source.Result.Path); ok {
			return true
		}
	}
	return false
}

// ParamPathFromResultPath maps resultPath of resultIdx to its parameter endpoint. It returns false
// when no source covers the result path or when the covering sources map it to different endpoints;
// HasSource distinguishes those cases when a caller needs to keep a sourced path severed.
func (s ReturnParamSources) ParamPathFromResultPath(resultIdx int, resultPath annotation.FieldPath) (IndexedFieldPath, bool) {
	var param IndexedFieldPath
	found := false
	for _, source := range s {
		candidate, ok := source.paramPathFromResultPath(resultIdx, resultPath)
		if !ok {
			continue
		}
		if found && candidate != param {
			return IndexedFieldPath{}, false
		}
		param = candidate
		found = true
	}
	return param, found
}

// collectWholeResultParamSource records a direct parameter source. Forwarded call sources compose
// through the parameter edges retained on their return forwarding edge.
func (fc *functionCollector) collectWholeResultParamSource(resultIdx int, expr ast.Expr) bool {
	if source, ok := fc.resolveReturnParamSource(expr); ok {
		fc.collected.addReturnParamSource(fc.funcObj, ReturnParamSource{
			Result: IndexedFieldPath{Idx: resultIdx},
			Param:  source,
		})
		return true
	}
	return false
}

// returnCallParamForwardingEdges maps every supported call input to a stable parameter of the
// caller. Unsupported call shapes return no edges rather than risk relating the result to the
// wrong caller value.
func (fc *functionCollector) returnCallParamForwardingEdges(
	call *ast.CallExpr,
	target typeshelper.StaticCallTarget,
) []paramFieldForwardEdge {
	if target.Origin == nil || target.Signature.Variadic() {
		return nil
	}
	var receiver ast.Expr
	if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if selection := fc.pass.TypesInfo.Selections[sel]; selection != nil {
			if selection.Kind() != types.MethodVal || len(selection.Index()) != 1 {
				return nil
			}
			receiver = sel.X
		}
	}
	var edges []paramFieldForwardEdge
	add := func(idx int, expr ast.Expr) bool {
		source, ok := fc.resolveReturnParamSource(expr)
		if !ok {
			return false
		}
		edges = append(edges, paramFieldForwardEdge{
			callerParamIdx: source.Idx,
			callerPrefix:   source.Path,
			callee:         target.Origin,
			calleeParamIdx: idx,
		})
		return true
	}
	if recv := target.Origin.Signature().Recv(); recv != nil {
		if receiver == nil {
			return nil
		}
		if _, isInterface := recv.Type().Underlying().(*types.Interface); isInterface {
			return nil
		}
		if !add(annotation.ReceiverParamIndex, receiver) {
			return nil
		}
	}
	for i, arg := range call.Args {
		if i >= target.Signature.Params().Len() {
			break
		}
		if _, isInterface := target.Signature.Params().At(i).Type().Underlying().(*types.Interface); isInterface {
			return nil
		}
		if !add(i, arg) {
			return nil
		}
	}
	return edges
}

// closeReturnParamSources composes callee sources through return forwarding edges until a fixpoint.
// Edges form chains (`return f()` inside `return g()`), so newly composed wrapper sources must feed
// back in for callers of the wrapper to resolve. Conflicting sources remain in the set so lookups
// reject them as ambiguous.
func closeReturnParamSources(sources returnParamSourceSet, edges map[*types.Func][]returnForwardEdge) {
	changed := true
	for changed {
		changed = false
		for fn, es := range edges {
			for _, edge := range es {
				if composeReturnParamSources(sources, fn, edge) {
					changed = true
				}
			}
		}
	}
}

// composeReturnParamSources re-roots every composable source of edge's callee onto fn through
// the matching parameter forwarding edge, reporting whether any source was newly added.
func composeReturnParamSources(sources returnParamSourceSet, fn *types.Func, edge returnForwardEdge) bool {
	if len(edge.paramForwardingEdges) == 0 {
		return false
	}
	changed := false
	for source := range sources[edge.callee] {
		if source.Result.Idx != edge.calleeResultIdx {
			continue
		}
		for _, paramEdge := range edge.paramForwardingEdges {
			if paramEdge.calleeParamIdx != source.Param.Idx {
				continue
			}
			paramPath := paramEdge.callerPrefix.Join(source.Param.Path)
			if !paramPath.IsRoot() && !paramFieldPathIsAcyclic(fn, paramEdge.callerParamIdx, paramPath) {
				break
			}
			if sources.add(fn, ReturnParamSource{
				Result: IndexedFieldPath{Idx: edge.callerResultIdx, Path: source.Result.Path},
				Param:  IndexedFieldPath{Idx: paramEdge.callerParamIdx, Path: paramPath},
			}) {
				changed = true
			}
			break
		}
	}
	return changed
}

// recordFieldParamSource records the param source of one construction field: a field at path
// within the result initialized from a stable parameter or a field chain rooted at one
// (`return &T{f: p.x}` records result field `f` as supplied by param field `x`). Sources are
// recorded for any field, not just nilable ones: a source on a value-struct field re-roots deeper
// accesses (`result.V.Child` under source `V <- p.node` resolves to `p.node.Child`).
func (fc *functionCollector) recordFieldParamSource(resultIdx int, path annotation.FieldPath, fieldVal ast.Expr) {
	source, ok := fc.resolveReturnParamSource(fieldVal)
	if !ok {
		return
	}
	if !resultFieldPathIsAcyclic(fc.funcObj, resultIdx, path) ||
		(!source.Path.IsRoot() && !paramFieldPathIsAcyclic(fc.funcObj, source.Idx, source.Path)) {
		return
	}
	fc.collected.addReturnParamSource(fc.funcObj, ReturnParamSource{
		Result: IndexedFieldPath{Idx: resultIdx, Path: path},
		Param:  source,
	})
}

// dropMixedResultParamSources drops sources for results that are both constructed and forwarded.
// It runs before closure so the ambiguity cannot propagate into wrappers.
func (c *collectedFieldEffects) dropMixedResultParamSources() {
	for fn, results := range c.resultsWithConstructSite {
		for idx := range results {
			mixed := false
			for source := range c.summary.returnParamSources[fn] {
				if source.Result.Idx == idx && source.Result.Path.IsRoot() {
					mixed = true
					break
				}
			}
			if !mixed {
				for _, edge := range c.returnForwardingEdges[fn] {
					if edge.callerResultIdx == idx && len(edge.paramForwardingEdges) > 0 {
						mixed = true
						break
					}
				}
			}
			if mixed {
				c.dropReturnParamSources(fn, idx)
				edges := c.returnForwardingEdges[fn]
				for i := range edges {
					if edges[i].callerResultIdx == idx {
						edges[i].paramForwardingEdges = nil
					}
				}
				c.returnForwardingEdges[fn] = edges
			}
		}
	}
}

func (c *collectedFieldEffects) dropReturnParamSources(fn *types.Func, result int) {
	for key := range c.summary.returnParamSources[fn] {
		if key.Result.Idx == result {
			delete(c.summary.returnParamSources[fn], key)
		}
	}
}

func (c *collectedFieldEffects) dropWholeResultParamSources(fn *types.Func, result int) {
	for key := range c.summary.returnParamSources[fn] {
		if key.Result.Idx == result && key.Result.Path.IsRoot() {
			delete(c.summary.returnParamSources[fn], key)
		}
	}
}

// resolveReturnParamSource resolves expr as a stable parameter or a field chain rooted at one,
// yielding the parameter (or receiver) endpoint: the parameter index and the field path within
// it (the root path for the bare parameter). An unstable parameter (reassigned or address-taken)
// resolves to nothing: its value at the return may not be the caller's argument.
func (fc *functionCollector) resolveReturnParamSource(expr ast.Expr) (IndexedFieldPath, bool) {
	base, segments := asthelper.SplitFieldChain(expr)
	if base == nil {
		return IndexedFieldPath{}, false
	}
	v, ok := fc.pass.TypesInfo.ObjectOf(base).(*types.Var)
	if !ok {
		return IndexedFieldPath{}, false
	}
	idx, ok := fc.paramIdx[v]
	if !ok || fc.unstableParams[v] {
		return IndexedFieldPath{}, false
	}
	return IndexedFieldPath{Idx: idx, Path: annotation.NewFieldPath(segments...)}, true
}

// collectUnstableParamVars returns the parameters (and receiver) whose value at a return
// statement cannot be assumed to be the caller's argument: parameters reassigned anywhere in the
// body — including inside function literals, whose bodies run with the captured variable — or
// having their address taken. A param source rooted at such a parameter would attribute some
// other value to the caller's argument, so collection skips it (a sound under-report).
func (fc *functionCollector) collectUnstableParamVars() map[*types.Var]bool {
	var out map[*types.Var]bool
	markUnstable := func(expr ast.Expr) {
		ident, ok := ast.Unparen(expr).(*ast.Ident)
		if !ok {
			return
		}
		v, ok := fc.pass.TypesInfo.ObjectOf(ident).(*types.Var)
		if !ok {
			return
		}
		if _, isParam := fc.paramIdx[v]; !isParam {
			return
		}
		if out == nil {
			out = make(map[*types.Var]bool)
		}
		out[v] = true
	}
	ast.Inspect(fc.fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			// Only assignment targets destabilize; `:=` defines fresh variables whose objects
			// differ from the parameters, so they never resolve to a parameter here.
			for _, lhs := range n.Lhs {
				markUnstable(lhs)
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND {
				markUnstable(n.X)
			}
		case *ast.RangeStmt:
			if n.Tok == token.ASSIGN {
				markUnstable(n.Key)
				markUnstable(n.Value)
			}
		}
		return true
	})
	return out
}
