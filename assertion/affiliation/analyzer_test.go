//  Copyright (c) 2024 Uber Technologies, Inc.
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

package affiliation

import (
	"bytes"
	"encoding/gob"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/orderedmap"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	// Intentionally give a nil pass variable to trigger a panic, but we should recover from it
	// and convert it to an error via the result struct.
	r, err := Analyzer.Run(nil /* pass */)
	require.NoError(t, err)
	require.ErrorContains(t, r.(*analysishelper.Result[[]annotation.FullTrigger]).Err, "INTERNAL PANIC")
}

func TestFact_Codec(t *testing.T) {
	t.Parallel()

	const n = 1000
	content := orderedmap.New[Pair, bool]()
	for i := range n {
		k := Pair{DeclaredID: "Declared" + strconv.Itoa(i), ImplementedID: "ImplementedID" + strconv.Itoa(i)}
		v := i%2 == 0
		content.Store(k, v)
	}
	cache := &Cache{Content: content}

	// Encode the fact 10 times and check that the result is always the same for determinism.
	var previous []byte
	for range 10 {
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(cache)
		require.NoError(t, err)
		require.NotEmpty(t, buf.Bytes())

		if len(previous) == 0 {
			previous = buf.Bytes()
			continue
		}
		require.Equal(t, previous, buf.Bytes(), "encoded fact must be deterministic")

		var decoded *Cache
		err = gob.NewDecoder(&buf).Decode(&decoded)
		require.NoError(t, err)
		require.NotNil(t, decoded)
		require.Equal(t, cache.Content.Pairs, decoded.Content.Pairs, "decoded fact is not the same as encoded fact")
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
