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
	"strings"

	"go.uber.org/nilaway/util/asthelper"
)

// ReturnParamSource records a result (or result field) whose value is the caller's own argument:
// the Result endpoint is supplied by the Param endpoint (the receiver uses
// annotation.ReceiverParamIndex). An empty Result path denotes the whole result value
// (`return p`, `return p.x`); an empty Param path the bare parameter. Collection is syntactic
// and conservative: a source is recorded only when the relation holds on every path to the return
// (see the drops in collectUnstableParamVars and dropMixedResultParamSources). Collected so later
// revisions can resolve param-sourced results from the actual argument at each call site instead of the
// shared return summary, severing one caller's argument from every other caller's result.
type ReturnParamSource struct {
	Result IndexedFieldPath
	Param  IndexedFieldPath
}

// SuppliesResultPath reports whether s supplies the value at (resultIdx, resultPath): a
// whole-result source (empty result path) supplies every path of that result, and a field-level
// source supplies its exact path and everything beneath it. The prefix match is per segment —
// source `Mid` supplies `Mid.Child` but not the sibling field `Middle`.
func (s ReturnParamSource) SuppliesResultPath(resultIdx int, resultPath string) bool {
	if s.Result.Idx != resultIdx {
		return false
	}
	if s.Result.Path == "" {
		return true
	}
	return resultPath == s.Result.Path || strings.HasPrefix(resultPath, s.Result.Path+".")
}

// returnParamSourceSet maps each function to its set of return param sources.
type returnParamSourceSet map[*types.Func]map[ReturnParamSource]bool

// add records source for funcObj, allocating the inner set on first use.
func (s returnParamSourceSet) add(funcObj *types.Func, source ReturnParamSource) {
	if s[funcObj] == nil {
		s[funcObj] = make(map[ReturnParamSource]bool)
	}
	s[funcObj][source] = true
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
func (e *BoundaryFieldEffects) ReturnParamSources(funcObj *types.Func) []ReturnParamSource {
	if e == nil {
		return nil
	}
	return e.returnParamSources.sortedSources(funcObj)
}

// collectWholeResultParamSource records the whole-result param source of one return operand: a
// returned parameter or parameter field chain (`return p`, `return p.x`, receiver variants).
// Disagreeing sources for the same result (a different parameter per return site) are all kept —
// consumers refuse to answer when sources disagree, which keeps the collected set deterministic. A
// result index that both constructs in-package and carries a whole-result source is ambiguous and
// dropped in close(); field-level sources of constructed results are recorded by
// recordFieldParamSource instead.
func (fc *functionCollector) collectWholeResultParamSource(resultIdx int, expr ast.Expr) {
	source, ok := fc.resolveReturnParamSource(expr)
	if !ok {
		return
	}
	fc.collected.addReturnParamSource(fc.funcObj, ReturnParamSource{
		Result: IndexedFieldPath{Idx: resultIdx},
		Param:  source,
	})
}

// recordFieldParamSource records the param source of one construction field: a field at path
// within the result initialized from a stable parameter or a field chain rooted at one
// (`return &T{f: p.x}` records result field `f` as supplied by param field `x`). Sources are
// recorded for any field, not just nilable ones: a source on a value-struct field re-roots deeper
// accesses (`result.V.Child` under source `V <- p.node` resolves to `p.node.Child`).
func (fc *functionCollector) recordFieldParamSource(resultIdx int, path string, fieldVal ast.Expr) {
	source, ok := fc.resolveReturnParamSource(fieldVal)
	if !ok {
		return
	}
	if !resultFieldPathIsAcyclic(fc.funcObj, resultIdx, path) ||
		(source.Path != "" && !paramFieldPathIsAcyclic(fc.funcObj, source.Idx, source.Path)) {
		return
	}
	fc.collected.addReturnParamSource(fc.funcObj, ReturnParamSource{
		Result: IndexedFieldPath{Idx: resultIdx, Path: path},
		Param:  source,
	})
}

// dropMixedResultParamSources removes every param source of a result index that has an in-package
// construction return site alongside a whole-result source. Such a result sometimes carries the
// parameter and sometimes a fresh object, so no context-insensitive source is sound for it;
// dropping is a deliberate under-report.
func (c *collectedFieldEffects) dropMixedResultParamSources() {
	for fn, results := range c.resultsWithConstructSite {
		for idx := range results {
			mixed := false
			for source := range c.summary.returnParamSources[fn] {
				if source.Result.Idx == idx && source.Result.Path == "" {
					mixed = true
					break
				}
			}
			if mixed {
				c.dropReturnParamSources(fn, idx)
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

// resolveReturnParamSource resolves expr as a stable parameter or a field chain rooted at one,
// yielding the parameter (or receiver) endpoint: the parameter index and the dotted path within
// it ("" for the bare parameter). An unstable parameter (reassigned or address-taken) resolves
// to nothing: its value at the return may not be the caller's argument.
func (fc *functionCollector) resolveReturnParamSource(expr ast.Expr) (IndexedFieldPath, bool) {
	base, path := asthelper.SplitFieldChain(expr)
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
	return IndexedFieldPath{Idx: idx, Path: path}, true
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
