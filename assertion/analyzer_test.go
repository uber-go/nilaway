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

package assertion

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/analysishelper"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	// Intentionally give a nil pass variable to trigger a panic, but we should recover from it
	// and convert it to an error via the result struct.
	r, err := Analyzer.Run(nil /* pass */)
	require.NoError(t, err)
	require.ErrorContains(t, r.(*analysishelper.Result[[]annotation.FullTrigger]).Err, "INTERNAL PANIC")
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
