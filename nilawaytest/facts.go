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
	"bytes"
	"encoding/gob"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/checker"
)

// FactCodecStats contains aggregate statistics for facts checked by RequireFactCodecs.
type FactCodecStats struct {
	TotalBytes int
}

// RequireFactCodecs checks every fact exported by NilAway reachable from the analysistest results.
//
// For each fact, it verifies that:
//   - gob encoding is non-empty;
//   - the encoded fact can be decoded; and
//   - re-encoding the decoded fact produces the original bytes.
//   - multiple encodings produce byte-identical results deterministically;
//
// It returns the statistics of the tested facts.
func RequireFactCodecs(t *testing.T, results []*analysistest.Result) FactCodecStats {
	t.Helper()

	// Visit the package action graph and collect all available facts.
	type exportedFact struct {
		action *checker.Action
		fact   analysis.Fact
	}
	var facts []exportedFact
	seen := make(map[*checker.Action]struct{})

	var visit func(*checker.Action)
	visit = func(action *checker.Action) {
		if _, ok := seen[action]; ok {
			return
		}
		seen[action] = struct{}{}

		for _, dependency := range action.Deps {
			visit(dependency)
		}

		// The action graph also contains prerequisite analyzers from go/analysis. Only collect
		// facts produced by NilAway analyzers.
		if !strings.HasPrefix(action.Analyzer.Name, "nilaway") {
			return
		}

		// AllPackageFacts and AllObjectFacts also include inherited facts. Only check facts owned
		// by this action's package so each exported fact is checked exactly once.
		for _, packageFact := range action.AllPackageFacts() {
			if packageFact.Package == action.Package.Types {
				facts = append(facts, exportedFact{action: action, fact: packageFact.Fact})
			}
		}
		for _, objectFact := range action.AllObjectFacts() {
			if objectFact.Object.Pkg() == action.Package.Types {
				facts = append(facts, exportedFact{action: action, fact: objectFact.Fact})
			}
		}
	}

	for _, result := range results {
		require.NotNil(t, result.Action)
		visit(result.Action)
	}

	// Now, run codec checks against collected NilAway facts.
	var stats FactCodecStats
	for _, exported := range facts {
		action, fact := exported.action, exported.fact

		encode := func(fact analysis.Fact) []byte {
			var buf bytes.Buffer
			require.NoErrorf(t, gob.NewEncoder(&buf).Encode(fact), "encoding %T exported by %s", fact, action)
			return buf.Bytes()
		}

		// Encode it multiple times, all encoded facts must be byte-identical.
		// We also collect stats in the first round of encoding.
		var encoded []byte
		for range 10 {
			current := encode(fact)
			require.NotEmptyf(t, current, "encoding %T exported by %s", fact, action)
			if encoded == nil {
				encoded = current
				stats.TotalBytes += len(encoded)
				continue
			}
			require.Equalf(t, encoded, current, "encoding of %T exported by %s must be deterministic", fact, action)
		}

		decoded := reflect.New(reflect.TypeOf(fact).Elem()).Interface().(analysis.Fact)
		require.NoErrorf(t, gob.NewDecoder(bytes.NewReader(encoded)).Decode(decoded), "decoding %T exported by %s", fact, action)
		require.Equalf(t, encoded, encode(decoded), "round-trip encoding of %T exported by %s changed", fact, action)
	}

	return stats
}
