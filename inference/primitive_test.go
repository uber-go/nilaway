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
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
)

func TestUpstreamObjectPositionsFactOrder(t *testing.T) {
	t.Parallel()

	// Nogo can report different positions for the same object in different package facts.
	site := primitiveSite{PkgPath: "testpkg", ObjectPath: "Request.M0"}
	facts := make([]analysis.PackageFact, 0, 2)
	for i, path := range []string{"testpkg1", "testpkg2"} {
		site.Position = token.Position{Filename: "proto.go", Line: 10, Column: i + 1}
		inferred := newInferredMap(nil)
		inferred.StoreDetermined(site, TrueBecauseAnnotation{})
		facts = append(facts, analysis.PackageFact{Package: types.NewPackage(path, path), Fact: inferred})
	}

	for _, order := range []struct {
		name  string
		facts []analysis.PackageFact
	}{
		{"forward", []analysis.PackageFact{facts[0], facts[1]}},
		{"reverse", []analysis.PackageFact{facts[1], facts[0]}},
	} {
		t.Run(order.name, func(t *testing.T) {
			t.Parallel()

			pass := &analysishelper.EnhancedPass{Pass: &analysis.Pass{
				AllPackageFacts: func() []analysis.PackageFact { return order.facts },
			}}
			primitive := newPrimitivizer(pass)
			require.Equal(t, site.Position, primitive.upstreamObjPositions["testpkg.Request.M0"])
		})
	}
}
