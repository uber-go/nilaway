//  Copyright (c) 2023 Uber Technologies, Inc.
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

//go:build go1.23

package nilaway

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestNilAway_Go123(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	// For descriptions of the purpose of each of the following tests, consult their source files
	// located in testdata/src/<package>.

	tests := []struct {
		name     string
		patterns []string
	}{
		{name: "LoopRangeGo123", patterns: []string{"go.uber.org/looprangego123"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("Running test for packages %s", tt.patterns)

			analysistest.Run(t, testdata, Analyzer, tt.patterns...)
		})
	}
}
