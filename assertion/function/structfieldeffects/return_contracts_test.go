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
)

func TestResolveResultPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     ReturnParamSource
		resultIdx  int
		resultPath string
		wantParam  IndexedFieldPath
		wantOK     bool
	}{
		{
			name:       "field-level source at its exact path",
			source:     ReturnParamSource{Result: IndexedFieldPath{Idx: 0, Path: "Mid"}, Param: IndexedFieldPath{Idx: 1, Path: "inner"}},
			resultIdx:  0,
			resultPath: "Mid",
			wantParam:  IndexedFieldPath{Idx: 1, Path: "inner"},
			wantOK:     true,
		},
		{
			name:       "field-level source re-roots a deeper path",
			source:     ReturnParamSource{Result: IndexedFieldPath{Idx: 0, Path: "Mid"}, Param: IndexedFieldPath{Idx: 1, Path: "inner"}},
			resultIdx:  0,
			resultPath: "Mid.Child",
			wantParam:  IndexedFieldPath{Idx: 1, Path: "inner.Child"},
			wantOK:     true,
		},
		{
			name:       "bare-param source keeps the bare param at the exact path",
			source:     ReturnParamSource{Result: IndexedFieldPath{Idx: 0, Path: "Existing"}, Param: IndexedFieldPath{Idx: 0}},
			resultIdx:  0,
			resultPath: "Existing",
			wantParam:  IndexedFieldPath{Idx: 0},
			wantOK:     true,
		},
		{
			name:       "whole-result source re-roots the full path",
			source:     ReturnParamSource{Result: IndexedFieldPath{Idx: 0}, Param: IndexedFieldPath{Idx: 0, Path: "inner"}},
			resultIdx:  0,
			resultPath: "Mid.Child",
			wantParam:  IndexedFieldPath{Idx: 0, Path: "inner.Mid.Child"},
			wantOK:     true,
		},
		{
			name:       "whole-result source at the result value itself",
			source:     ReturnParamSource{Result: IndexedFieldPath{Idx: 0}, Param: IndexedFieldPath{Idx: 0, Path: "inner"}},
			resultIdx:  0,
			resultPath: "",
			wantParam:  IndexedFieldPath{Idx: 0, Path: "inner"},
			wantOK:     true,
		},
		{
			name:       "sibling field sharing the source prefix is not supplied",
			source:     ReturnParamSource{Result: IndexedFieldPath{Idx: 0, Path: "Mid"}, Param: IndexedFieldPath{Idx: 1, Path: "inner"}},
			resultIdx:  0,
			resultPath: "Middle",
			wantOK:     false,
		},
		{
			name:       "different result index is not supplied",
			source:     ReturnParamSource{Result: IndexedFieldPath{Idx: 0, Path: "Mid"}, Param: IndexedFieldPath{Idx: 1, Path: "inner"}},
			resultIdx:  1,
			resultPath: "Mid",
			wantOK:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			param, ok := tt.source.ResolveResultPath(tt.resultIdx, tt.resultPath)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantParam, param)
		})
	}
}
