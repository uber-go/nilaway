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

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

// sliceStringer is an uncomparable fmt.Stringer. Explanation identity must not use == on the
// interface values that carry these, or observation will panic.
type sliceStringer []string

func (s sliceStringer) String() string { return s[0] }

func TestSiteReportExplanations(t *testing.T) {
	t.Parallel()

	nilExp := shallowTrue(pos(1), "nil-a")
	c1 := shallowFalse(pos(10), "deref-1")
	c2 := shallowFalse(pos(11), "deref-2")
	wantTwo := []expPair{{nilExp, c1}, {nilExp, c2}}

	t.Run("permutations", func(t *testing.T) {
		t.Parallel()
		orders := [][]ExplainedBool{
			{nilExp, c1, c2},
			{nilExp, c2, c1},
			{c1, nilExp, c2}, // second consumer arrives after the conflict
			{c2, nilExp, c1},
			{c1, c2, nilExp},
			{c2, c1, nilExp},
		}
		for i, order := range orders {
			t.Run(fmt.Sprintf("order-%d", i), func(t *testing.T) {
				t.Parallel()
				e, h := newRecordingEngine()
				for _, exp := range order {
					e.observeSiteExplanation(testSite(1, false), exp)
				}
				assertPairs(t, h, wantTwo)
			})
		}
	})

	t.Run("reobserve", func(t *testing.T) {
		t.Parallel()
		e, h := newRecordingEngine()
		site := testSite(1, false)
		for _, exp := range []ExplainedBool{nilExp, c1, c1, c2, nilExp, c2} {
			e.observeSiteExplanation(site, exp)
		}
		assertPairs(t, h, wantTwo)
	})

	t.Run("three consumers", func(t *testing.T) {
		t.Parallel()
		c3 := shallowFalse(pos(12), "deref-3")
		e, h := newRecordingEngine()
		site := testSite(1, false)
		for _, exp := range []ExplainedBool{c1, c2, c3, nilExp} {
			e.observeSiteExplanation(site, exp)
		}
		assertPairs(t, h, []expPair{{nilExp, c1}, {nilExp, c2}, {nilExp, c3}})
	})

	t.Run("distinct nil sources", func(t *testing.T) {
		t.Parallel()
		nilB := shallowTrue(pos(2), "nil-b")
		// Both nils conflict with the nonnil consumers, so neither source is collapsed.
		conflicting := []expPair{{nilExp, c1}, {nilExp, c2}, {nilB, c1}, {nilB, c2}}
		// nilB agrees with a nilable canonical value, so it does not add reports.
		agreeing := []expPair{{nilExp, c1}, {nilExp, c2}}
		cases := []struct {
			order []ExplainedBool
			want  []expPair
		}{
			{[]ExplainedBool{c1, c2, nilExp, nilB}, conflicting},
			{[]ExplainedBool{c1, nilExp, c2, nilB}, conflicting}, // second source arrives after the conflict
			{[]ExplainedBool{nilExp, nilB, c1, c2}, agreeing},
		}
		for i, tc := range cases {
			t.Run(fmt.Sprintf("order-%d", i), func(t *testing.T) {
				t.Parallel()
				e, h := newRecordingEngine()
				for _, exp := range tc.order {
					e.observeSiteExplanation(testSite(1, false), exp)
				}
				assertPairs(t, h, tc.want)
			})
		}
	})

	t.Run("separate sites", func(t *testing.T) {
		t.Parallel()
		nilB := shallowTrue(pos(2), "nil-b")
		e, h := newRecordingEngine()
		e.observeSiteExplanation(testSite(1, false), nilExp)
		e.observeSiteExplanation(testSite(1, false), c1)
		e.observeSiteExplanation(testSite(2, false), nilB)
		e.observeSiteExplanation(testSite(2, false), c2)
		assertPairs(t, h, []expPair{{nilExp, c1}, {nilB, c2}})
	})

	t.Run("shallow and deep sites", func(t *testing.T) {
		t.Parallel()
		nilDeep := shallowTrue(pos(2), "nil-deep-site")
		cDeep := shallowFalse(pos(12), "deref-deep-site")
		e, h := newRecordingEngine()
		// Same position and repr; depth is part of the site key.
		e.observeSiteExplanation(testSite(1, false), c1)
		e.observeSiteExplanation(testSite(1, false), nilExp)
		e.observeSiteExplanation(testSite(1, true), nilDeep)
		e.observeSiteExplanation(testSite(1, true), cDeep)
		assertPairs(t, h, []expPair{{nilExp, c1}, {nilDeep, cDeep}})
	})

	t.Run("deep explanations", func(t *testing.T) {
		t.Parallel()
		// A deep nil chain is a different flow from the shallow source it wraps when both conflict.
		// A deep nonnil chain that ends at the same consumer is not a second panic site.
		// A deep nil that merely agrees with the canonical nil is not a second report.
		deepNil := deepTrue(pos(30), nilExp)
		deepSameConsumer := deepFalse(pos(31), c1)
		cOther := shallowFalse(pos(13), "deref-other")
		nilOther := shallowTrue(pos(3), "nil-other")
		conflicting := []expPair{
			{nilExp, c1}, {nilExp, cOther},
			{deepNil, c1}, {deepNil, cOther},
			{nilOther, c1}, {nilOther, cOther},
		}
		agreeing := []expPair{{nilExp, c1}, {nilExp, cOther}}
		cases := []struct {
			order []ExplainedBool
			want  []expPair
		}{
			{[]ExplainedBool{c1, deepSameConsumer, cOther, nilExp, deepNil, nilOther}, conflicting},
			{[]ExplainedBool{c1, c1, nilExp, nilExp, deepNil, deepSameConsumer, cOther, nilOther, deepNil}, conflicting},
			{[]ExplainedBool{nilExp, deepNil, c1, deepSameConsumer, cOther, nilOther}, agreeing},
		}
		for i, tc := range cases {
			t.Run(fmt.Sprintf("order-%d", i), func(t *testing.T) {
				t.Parallel()
				e, h := newRecordingEngine()
				for _, exp := range tc.order {
					e.observeSiteExplanation(testSite(1, false), exp)
				}
				assertPairs(t, h, tc.want)
			})
		}
	})

	t.Run("annotation positions", func(t *testing.T) {
		t.Parallel()
		// Annotation text does not include a position, so identity has to.
		annA := TrueBecauseAnnotation{AnnotationPos: pos(4)}
		annB := TrueBecauseAnnotation{AnnotationPos: pos(5)}
		nonA := FalseBecauseAnnotation{AnnotationPos: pos(14)}
		nonB := FalseBecauseAnnotation{AnnotationPos: pos(15)}
		e, h := newRecordingEngine()
		// Consumers first, so both annotations conflict and their positions stay distinct.
		for _, exp := range []ExplainedBool{nonA, annA, annB, nonB} {
			e.observeSiteExplanation(testSite(1, false), exp)
		}
		assertPairs(t, h, []expPair{{annA, nonA}, {annA, nonB}, {annB, nonA}, {annB, nonB}})
	})

	t.Run("nonnil only", func(t *testing.T) {
		t.Parallel()
		e, h := newRecordingEngine()
		site := testSite(1, false)
		e.observeSiteExplanation(site, c1)
		e.observeSiteExplanation(site, c2)
		require.Empty(t, h.over)
		val, ok := e.InferredMap().Load(site)
		require.True(t, ok)
		got := val.(*DeterminedVal)
		require.False(t, got.Bool.Val())
		require.Equal(t, c1.String(), got.Bool.String())
	})

	t.Run("canonical value", func(t *testing.T) {
		t.Parallel()
		site := testSite(1, false)

		e, _ := newRecordingEngine()
		e.observeSiteExplanation(site, c1)
		e.observeSiteExplanation(site, c2)
		e.observeSiteExplanation(site, nilExp)
		val, _ := e.InferredMap().Load(site)
		require.Equal(t, c1.String(), val.(*DeterminedVal).Bool.String())
		require.False(t, val.(*DeterminedVal).Bool.Val())

		e, _ = newRecordingEngine()
		e.observeSiteExplanation(site, nilExp)
		e.observeSiteExplanation(site, c1)
		e.observeSiteExplanation(site, c2)
		val, _ = e.InferredMap().Load(site)
		require.Equal(t, nilExp.String(), val.(*DeterminedVal).Bool.String())
		require.True(t, val.(*DeterminedVal).Bool.Val())
	})
}

func TestSiteReportDoesNotPropagateAlternates(t *testing.T) {
	t.Parallel()

	e, h := newRecordingEngine()
	producer := testSite(1, false)
	consumer := testSite(2, false)
	assertion := primitiveFullTrigger{
		Position:     pos(3),
		ProducerRepr: sliceStringer{"prod"},
		ConsumerRepr: sliceStringer{"cons"},
	}
	e.observeImplication(producer, consumer, assertion)

	nilA := shallowTrue(pos(1), "nil-a")
	nilB := shallowTrue(pos(2), "nil-b")
	c1 := shallowFalse(pos(10), "deref-1")
	c2 := shallowFalse(pos(11), "deref-2")
	propagated := TrueBecauseDeepConstraint{InternalAssertion: assertion, DeeperExplanation: nilA}

	e.observeSiteExplanation(producer, nilA)
	e.observeSiteExplanation(consumer, c1)
	e.observeSiteExplanation(consumer, c2)
	// A second nil source at the producer must not be pushed along the implication.
	e.observeSiteExplanation(producer, nilB)

	assertPairs(t, h, []expPair{{propagated, c1}, {propagated, c2}})
	prod, _ := e.InferredMap().Load(producer)
	require.Equal(t, nilA.String(), prod.(*DeterminedVal).Bool.String())
	cons, _ := e.InferredMap().Load(consumer)
	require.True(t, cons.(*DeterminedVal).Bool.Val())
}

func TestPackagePhasesKeepReportsAndActivation(t *testing.T) {
	t.Parallel()

	const src = `package order

func callee(p *int) {
	_ = *p
	_ = *p
}

func caller() {
	callee(nil)
}
`
	pass := typecheckedPass(t, src)
	file := pass.Files[0]
	var (
		callee  *types.Func
		stars   []*ast.StarExpr
		nilExpr *ast.Ident
	)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Name.Name == "callee" {
				callee = pass.TypesInfo.Defs[n.Name].(*types.Func)
			}
		case *ast.StarExpr:
			// Skip the *int parameter type; keep the two dereferences.
			if id, ok := n.X.(*ast.Ident); ok && id.Name == "p" {
				stars = append(stars, n)
			}
		case *ast.Ident:
			if n.Name == "nil" {
				nilExpr = n
			}
		}
		return true
	})
	require.NotNil(t, callee)
	require.Len(t, stars, 2)
	require.NotNil(t, nilExpr)

	param := annotation.ParamKeyFromArgNum(callee, 0)
	deref := func(star *ast.StarExpr) annotation.FullTrigger {
		return annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{
				Annotation: &annotation.FuncParam{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: param}},
				Expr:       star.X,
			},
			Consumer: &annotation.ConsumeTrigger{
				Annotation: &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}},
				Expr:       star.X,
			},
		}
	}
	nilPass := annotation.FullTrigger{
		Producer: &annotation.ProduceTrigger{
			Annotation: &annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}},
			Expr:       nilExpr,
		},
		Consumer: &annotation.ConsumeTrigger{
			Annotation: &annotation.ArgPass{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: param}},
			Expr:       nilExpr,
		},
	}
	calleeFirst := []annotation.FullTrigger{deref(stars[0]), deref(stars[1]), nilPass}
	callerFirst := []annotation.FullTrigger{nilPass, deref(stars[0]), deref(stars[1])}

	t.Run("both consumers through triggers", func(t *testing.T) {
		t.Parallel()
		for _, triggers := range [][]annotation.FullTrigger{calleeFirst, callerFirst} {
			e, h := newPassEngine(pass)
			e.buildPkgInferenceMap(triggers)
			require.Empty(t, h.single)
			assertConsumerOffsets(t, h, pass, stars[0].X.Pos(), stars[1].X.Pos())

			// The second phase observes the same triggers again.
			e.buildPkgInferenceMap(triggers)
			assertConsumerOffsets(t, h, pass, stars[0].X.Pos(), stars[1].X.Pos())

			// Canonical inference value stays the first explanation's nilability.
			site := e.primitive.site(param, false)
			val, ok := e.InferredMap().Load(site)
			require.True(t, ok)
			require.Equal(t, triggers[0].Producer.Annotation.Kind() == annotation.Always, val.(*DeterminedVal).Bool.Val())
		}
	})

	t.Run("controlled trigger reactivation", func(t *testing.T) {
		t.Parallel()
		controller := annotation.NewCallSiteParamKey(callee, 0, pass.Fset.Position(nilExpr.Pos()))
		nonnil := annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{
				Annotation: &annotation.FuncParam{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: controller}},
				Expr:       stars[0].X,
			},
			Consumer: &annotation.ConsumeTrigger{
				Annotation: &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}},
				Expr:       stars[0].X,
			},
		}
		controlled := annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{
				Annotation: &annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}},
				Expr:       nilExpr,
			},
			Consumer: &annotation.ConsumeTrigger{
				Annotation: &annotation.PtrLoad{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}},
				Expr:       stars[1].X,
			},
			Controller: controller,
		}
		nilable := annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{
				Annotation: &annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}},
				Expr:       nilExpr,
			},
			Consumer: &annotation.ConsumeTrigger{
				Annotation: &annotation.ArgPass{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: controller}},
				Expr:       nilExpr,
			},
		}
		// The nilable trigger is duplicated so the controller reactivates the controlled trigger.
		triggers := []annotation.FullTrigger{nonnil, controlled, nilable, nilable}

		e, h := newPassEngine(pass)
		e.ObservePackage(triggers)
		require.Equal(t, 2, h.single)
		require.Len(t, h.over, 1)
		site := e.primitive.site(controller, false)
		val, ok := e.InferredMap().Load(site)
		require.True(t, ok)
		require.False(t, val.(*DeterminedVal).Bool.Val())

		// A second package pass runs both inference phases again without a second overconstraint.
		e.ObservePackage(append([]annotation.FullTrigger(nil), triggers...))
		require.Equal(t, 4, h.single)
		require.Len(t, h.over, 1)
		val, _ = e.InferredMap().Load(site)
		require.False(t, val.(*DeterminedVal).Bool.Val())
	})
}

type expPair struct {
	nilExp ExplainedBool
	nonnil ExplainedBool
}

type recordingHandler struct {
	over   [][2]ExplainedBool
	single int
}

func (r *recordingHandler) AddSingleAssertionConflict(annotation.FullTrigger) { r.single++ }

func (r *recordingHandler) AddOverconstraintConflict(nilExp, nonnilExp ExplainedBool) {
	r.over = append(r.over, [2]ExplainedBool{nilExp, nonnilExp})
}

func newRecordingEngine() (*Engine, *recordingHandler) {
	h := &recordingHandler{}
	return &Engine{
		inferredMap:      newInferredMap(nil),
		diagnosticEngine: h,
	}, h
}

func newPassEngine(pass *analysishelper.EnhancedPass) (*Engine, *recordingHandler) {
	h := &recordingHandler{}
	return NewEngine(pass, h), h
}

func assertPairs(t *testing.T, h *recordingHandler, want []expPair) {
	t.Helper()
	got := make(map[string]int, len(h.over))
	for _, c := range h.over {
		got[pairSpec(c[0], c[1])]++
	}
	require.Len(t, h.over, len(want), "duplicate or missing conflict")
	require.Len(t, got, len(want))
	for _, pair := range want {
		spec := pairSpec(pair.nilExp, pair.nonnil)
		require.Equal(t, 1, got[spec], spec)
	}
	require.Zero(t, h.single)
}

func assertConsumerOffsets(t *testing.T, h *recordingHandler, pass *analysishelper.EnhancedPass, want ...token.Pos) {
	t.Helper()
	got := make([]int, 0, len(h.over))
	for _, c := range h.over {
		got = append(got, endNode(c[1]).Position().Offset)
	}
	wantOffsets := make([]int, len(want))
	for i, p := range want {
		wantOffsets[i] = pass.Fset.Position(p).Offset
	}
	require.ElementsMatch(t, wantOffsets, got)
}

// pairSpec is the test's view of a conflict: the full nil chain, and the terminal consumer.
func pairSpec(nilExp, nonnilExp ExplainedBool) string {
	var b strings.Builder
	for exp := nilExp; exp != nil; exp = exp.DeeperReason() {
		b.WriteString(nodeSpec(exp))
		b.WriteByte('|')
	}
	b.WriteString(" -> ")
	b.WriteString(nodeSpec(endNode(nonnilExp)))
	return b.String()
}

func nodeSpec(exp ExplainedBool) string {
	p := exp.Position()
	return fmt.Sprintf("%s@%s:%d:%d:%d", exp.String(), p.Filename, p.Line, p.Column, p.Offset)
}

func endNode(exp ExplainedBool) ExplainedBool {
	for exp.DeeperReason() != nil {
		exp = exp.DeeperReason()
	}
	return exp
}

func pos(offset int) token.Position {
	return token.Position{Filename: "order.go", Offset: offset, Line: offset, Column: 1}
}

func testSite(n int, deep bool) primitiveSite {
	return primitiveSite{
		Position: pos(100 + n),
		PkgPath:  "p",
		Repr:     fmt.Sprintf("site-%d", n),
		IsDeep:   deep,
	}
}

func located(text string, at token.Position) annotation.LocatedRepr {
	return annotation.LocatedRepr{Contained: sliceStringer{text}, Location: at}
}

func shallowTrue(at token.Position, producer string) TrueBecauseShallowConstraint {
	return TrueBecauseShallowConstraint{ExternalAssertion: primitiveFullTrigger{
		Position:     at,
		ProducerRepr: located(producer, at),
		ConsumerRepr: located("arg pass", at),
	}}
}

func shallowFalse(at token.Position, consumer string) FalseBecauseShallowConstraint {
	return FalseBecauseShallowConstraint{ExternalAssertion: primitiveFullTrigger{
		Position:     at,
		ProducerRepr: located("param", at),
		ConsumerRepr: located(consumer, at),
	}}
}

func deepTrue(at token.Position, deeper ExplainedBool) TrueBecauseDeepConstraint {
	return TrueBecauseDeepConstraint{
		InternalAssertion: primitiveFullTrigger{
			Position:     at,
			ProducerRepr: sliceStringer{"upstream"},
			ConsumerRepr: sliceStringer{"downstream"},
		},
		DeeperExplanation: deeper,
	}
}

func deepFalse(at token.Position, deeper ExplainedBool) FalseBecauseDeepConstraint {
	return FalseBecauseDeepConstraint{
		InternalAssertion: primitiveFullTrigger{
			Position:     at,
			ProducerRepr: sliceStringer{"upstream"},
			ConsumerRepr: sliceStringer{"downstream"},
		},
		DeeperExplanation: deeper,
	}
}

func typecheckedPass(t *testing.T, src string) *analysishelper.EnhancedPass {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "order.go", src, 0)
	require.NoError(t, err)

	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := (&types.Config{
		Importer: importer.Default(),
		Sizes:    types.SizesFor("gc", runtime.GOARCH),
	}).Check("order", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	return analysishelper.NewEnhancedPass(&analysis.Pass{
		Fset:       fset,
		Files:      []*ast.File{file},
		Pkg:        pkg,
		TypesInfo:  info,
		TypesSizes: types.SizesFor("gc", runtime.GOARCH),
		ResultOf: map[*analysis.Analyzer]any{
			config.Analyzer: &config.Config{},
		},
		AllPackageFacts: func() []analysis.PackageFact { return nil },
		Report:          func(analysis.Diagnostic) {},
	})
}
