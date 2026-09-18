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

package annotation

import (
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/guard"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

func newFullTrigger(expr, producerExpr ast.Expr, guardMatched bool) FullTrigger {
	return FullTrigger{
		Producer: &ProduceTrigger{
			Annotation: &ProduceTriggerTautology{},
			Expr:       producerExpr,
		},
		Consumer: &ConsumeTrigger{
			Annotation:   &ConsumeTriggerTautology{},
			Expr:         expr,
			Guards:       guard.NoGuards(),
			GuardMatched: guardMatched,
		},
	}
}

func newFullTriggerPass(t *testing.T, producerExpr ast.Expr, consumerExpr ast.Expr, authenticProducer bool) *analysishelper.EnhancedPass {
	t.Helper()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	fset := token.NewFileSet()
	fset.AddFile(filepath.Join(cwd, "full_trigger.go"), -1, 100)

	pass := &analysis.Pass{
		Fset: fset,
		TypesInfo: &types.Info{
			Types: map[ast.Expr]types.TypeAndValue{
				consumerExpr: {
					Type: types.Typ[types.Int],
				},
			},
		},
		ResultOf: map[*analysis.Analyzer]any{
			config.Analyzer: &config.Config{PrintFullFilePath: true},
		},
	}

	if authenticProducer {
		pass.TypesInfo.Types[producerExpr] = types.TypeAndValue{Type: types.Typ[types.Int]}
	}

	return analysishelper.NewEnhancedPass(pass)
}

func TestFullTriggerReprs(t *testing.T) {
	t.Parallel()

	var (
		fset         = token.NewFileSet()
		file         = fset.AddFile("full_trigger.go", -1, 100)
		producerExpr = &ast.Ident{Name: "producer", NamePos: file.Pos(10)}
		consumerExpr = &ast.Ident{Name: "consumer", NamePos: file.Pos(20)}
		pass         = newFullTriggerPass(t, producerExpr, consumerExpr, true)
		trigger      = newFullTrigger(consumerExpr, producerExpr, false)
	)

	producer, consumer := trigger.Reprs(pass)
	_, producerLocated := producer.(LocatedRepr)
	_, consumerLocated := consumer.(LocatedRepr)
	require.True(t, producerLocated)
	require.True(t, consumerLocated)
	require.Contains(t, producer.String(), "nilable value")
	require.Contains(t, producer.String(), "full_trigger.go:1")
	require.Contains(t, consumer.String(), "must be nonnil")
	require.Contains(t, consumer.String(), "full_trigger.go:1")

	artificialPass := newFullTriggerPass(t, producerExpr, consumerExpr, false)
	producer, consumer = trigger.Reprs(artificialPass)
	_, producerLocated = producer.(LocatedRepr)
	_, consumerLocated = consumer.(LocatedRepr)
	require.False(t, producerLocated)
	require.True(t, consumerLocated)
	require.Equal(t, "nilable value", producer.String())
}

func TestFullTriggerPositionAndControl(t *testing.T) {
	t.Parallel()

	producerExpr := &ast.Ident{}
	consumerExpr := &ast.Ident{}

	trigger := newFullTrigger(consumerExpr, producerExpr, false)
	require.False(t, trigger.Controlled())

	trigger.Controller = &CallSiteParamAnnotationKey{}
	require.True(t, trigger.Controlled())
	require.Equal(t, consumerExpr.Pos(), trigger.Pos())
}

func TestFullTriggerEquality(t *testing.T) {
	t.Parallel()

	expr := &ast.Ident{}
	left := newFullTrigger(expr, &ast.Ident{}, false)
	right := newFullTrigger(expr, &ast.Ident{}, false)
	require.True(t, left.equals(right))

	right.Consumer.GuardMatched = true
	require.False(t, left.equals(right))
	require.True(t, left.equalsModuloGuardMatched(right))

	right.Consumer.Expr = &ast.Ident{}
	require.False(t, left.equalsModuloGuardMatched(right))
}

func TestFullTriggerSlicesEq(t *testing.T) {
	t.Parallel()

	expr := &ast.Ident{}
	trigger := newFullTrigger(expr, &ast.Ident{}, false)
	equal := newFullTrigger(expr, &ast.Ident{}, false)
	guardMismatch := newFullTrigger(expr, &ast.Ident{}, true)

	require.True(t, FullTriggerSlicesEq(nil, nil))
	require.True(t, FullTriggerSlicesEq([]FullTrigger{trigger}, []FullTrigger{equal}))
	require.False(t, FullTriggerSlicesEq([]FullTrigger{trigger}, []FullTrigger{guardMismatch}))
	require.False(t, FullTriggerSlicesEq([]FullTrigger{trigger, trigger}, []FullTrigger{equal}))
	require.False(t, FullTriggerSlicesEq([]FullTrigger{trigger}, nil))
}

func TestMergeFullTriggers(t *testing.T) {
	t.Parallel()

	expr := &ast.Ident{}
	left := newFullTrigger(expr, &ast.Ident{}, true)
	left.Consumer.Guards.Add(guard.Nonce(1))
	right := newFullTrigger(expr, &ast.Ident{}, false)

	merged := MergeFullTriggers([]FullTrigger{left}, right)
	require.Len(t, merged, 1)
	require.False(t, merged[0].Consumer.GuardMatched)
	require.Empty(t, merged[0].Consumer.Guards)

	additional := newFullTrigger(&ast.Ident{}, &ast.Ident{}, false)
	merged = MergeFullTriggers(merged, additional)
	require.Len(t, merged, 2)
}

func TestLocatedRepr(t *testing.T) {
	t.Parallel()

	require.Equal(t, "nilable value at \"file.go:3:4\"", (LocatedRepr{
		Contained: TriggerIfNilableRepr{},
		Location:  token.Position{Filename: "file.go", Line: 3, Column: 4},
	}).String())
}
