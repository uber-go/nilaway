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

package analysishelper

import (
	"testing"

	"go/parser"
	"go/token"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
)

func TestEnhancedPass_Panic(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", "package p\nvar x = 1\n", 0)
	require.NoError(t, err)

	pass := NewEnhancedPass(&analysis.Pass{Fset: fset})

	msg := "test panic"
	require.PanicsWithValue(t, msg+" (sample.go:1)", func() { pass.Panic(msg, file.Pos()) })
}
