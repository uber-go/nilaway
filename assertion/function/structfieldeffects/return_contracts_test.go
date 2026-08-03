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

package structfieldeffects

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/annotation"
)

func TestReturnParamSources(t *testing.T) {
	t.Parallel()

	field := func(idx int, path ...string) IndexedFieldPath {
		return IndexedFieldPath{Idx: idx, Path: annotation.NewFieldPath(path...)}
	}
	source := func(result, param IndexedFieldPath) ReturnParamSource {
		return ReturnParamSource{Result: result, Param: param}
	}

	tests := []struct {
		name       string
		sources    ReturnParamSources
		resultIdx  int
		resultPath annotation.FieldPath
		wantParam  IndexedFieldPath
		wantSource bool
		wantPath   bool
	}{
		{
			name:       "no source",
			resultPath: annotation.NewFieldPath("Mid"),
		},
		{
			name:       "field source at exact path",
			sources:    ReturnParamSources{source(field(0, "Mid"), field(1, "inner"))},
			resultPath: annotation.NewFieldPath("Mid"),
			wantParam:  field(1, "inner"),
			wantSource: true,
			wantPath:   true,
		},
		{
			name:       "field source maps descendant",
			sources:    ReturnParamSources{source(field(0, "Mid"), field(1, "inner"))},
			resultPath: annotation.NewFieldPath("Mid", "Child"),
			wantParam:  field(1, "inner", "Child"),
			wantSource: true,
			wantPath:   true,
		},
		{
			name: "covering sources agree",
			sources: ReturnParamSources{
				source(field(0, "Mid"), field(1, "inner")),
				source(field(0, "Mid", "Child"), field(1, "inner", "Child")),
			},
			resultPath: annotation.NewFieldPath("Mid", "Child"),
			wantParam:  field(1, "inner", "Child"),
			wantSource: true,
			wantPath:   true,
		},
		{
			name: "covering sources disagree",
			sources: ReturnParamSources{
				source(field(0, "Mid"), field(1, "other")),
				source(field(0, "Mid", "Child"), field(1, "inner", "Child")),
			},
			resultPath: annotation.NewFieldPath("Mid", "Child"),
			wantSource: true,
		},
		{
			name:       "root source maps descendant",
			sources:    ReturnParamSources{source(field(0), field(1, "whole"))},
			resultPath: annotation.NewFieldPath("Mid", "Child"),
			wantParam:  field(1, "whole", "Mid", "Child"),
			wantSource: true,
			wantPath:   true,
		},
		{
			name:       "root source maps root",
			sources:    ReturnParamSources{source(field(0), field(1, "whole"))},
			wantParam:  field(1, "whole"),
			wantSource: true,
			wantPath:   true,
		},
		{
			name:    "field source does not cover root",
			sources: ReturnParamSources{source(field(0, "Mid"), field(1, "inner"))},
		},
		{
			name:       "sibling field is not covered",
			sources:    ReturnParamSources{source(field(0, "Mid"), field(1, "inner"))},
			resultPath: annotation.NewFieldPath("Middle"),
		},
		{
			name:       "different result is not covered",
			sources:    ReturnParamSources{source(field(0, "Mid"), field(1, "inner"))},
			resultIdx:  1,
			resultPath: annotation.NewFieldPath("Mid"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.wantSource, tt.sources.HasSource(tt.resultIdx, tt.resultPath))
			param, ok := tt.sources.ParamPathFromResultPath(tt.resultIdx, tt.resultPath)
			require.Equal(t, tt.wantPath, ok)
			require.Equal(t, tt.wantParam, param)
		})
	}
}
