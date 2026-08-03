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

package nilawaytest

import (
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis"
)

// AllAnalyzers returns all NilAway analyzers reachable from root.
// The result slice is ordered by analyzer names.
func AllAnalyzers(t *testing.T, root *analysis.Analyzer) []*analysis.Analyzer {
	t.Helper()

	var analyzers []*analysis.Analyzer
	seen := make(map[*analysis.Analyzer]struct{})

	var visit func(*analysis.Analyzer)
	visit = func(analyzer *analysis.Analyzer) {
		require.NotNil(t, analyzer, "analyzer should not be nil")
		if _, ok := seen[analyzer]; ok {
			return
		}
		seen[analyzer] = struct{}{}

		require.NotNil(t, analyzer.Run, "analyzer %q Run func should not be nil", analyzer.Name)
		run := runtime.FuncForPC(reflect.ValueOf(analyzer.Run).Pointer())
		require.NotNil(t, run, "reflection on analyzer %q Run func should not fail", analyzer.Name)

		// Stop traversing on non-NilAway analyzer.
		name := run.Name()
		if !strings.HasPrefix(name, config.NilAwayPkgPathPrefix+".") &&
			!strings.HasPrefix(name, config.NilAwayPkgPathPrefix+"/") {
			return
		}

		analyzers = append(analyzers, analyzer)
		for _, required := range analyzer.Requires {
			visit(required)
		}
	}

	// Start visiting from root.
	visit(root)
	slices.SortFunc(analyzers, func(left, right *analysis.Analyzer) int { return strings.Compare(left.Name, right.Name) })
	return analyzers
}
