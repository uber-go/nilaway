//  Copyright (c) 2025 Uber Technologies, Inc.
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

package typeshelper

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsIterType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typeStr string
		want    bool
	}{
		{"ValidIterator0", "func(func() bool)", true},
		{"ValidIterator1", "func(func(int) bool)", true},
		{"ValidIterator2", "func(func(int, string) bool)", true},
		{"InvalidNonFunc", "int", false},
		{"InvalidFuncWrongReturn", "func(func(int) int)", false},
		{"InvalidFuncNoBool", "func(func(int, string))", false},
	}

	fset := token.NewFileSet()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := types.NewPackage("testpkg", "testpkg")
			typeInfo, err := types.Eval(fset, pkg, 0, tt.typeStr)
			if err != nil {
				t.Fatalf("failed to evaluate type: %v", err)
			}

			got := IsIterType(typeInfo.Type)
			require.Equal(t, tt.want, got, "IsIterType(%s) = %v, want %v", tt.typeStr, got, tt.want)
		})
	}
}
