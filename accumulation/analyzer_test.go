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

package accumulation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
)

func TestInternalErrorPos(t *testing.T) {
	t.Parallel()

	// Drivers may share one FileSet across packages, so a file from another package can be
	// parsed before the files of the current package.
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "other/other.go", "package other\n", 0)
	require.NoError(t, err)
	f, err := parser.ParseFile(fset, "a/a.go", "// Comment.\n\npackage a\n", 0)
	require.NoError(t, err)

	pass := &analysis.Pass{Fset: fset, Files: []*ast.File{f}}
	pos := internalErrorPos(pass)
	require.Equal(t, "a/a.go", fset.Position(pos).Filename)
	require.Equal(t, 3, fset.Position(pos).Line)

	// Without any files, we fall back to a valid position so that the diagnostic is not dropped.
	require.Equal(t, token.Pos(1), internalErrorPos(&analysis.Pass{Fset: fset}))
}
