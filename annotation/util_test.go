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

package annotation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeepNilabilityIsLocalAndAsNamedType(t *testing.T) {
	t.Parallel()

	const src = `package testpkg

type NamedSlice []*int
type AliasOfNamed = NamedSlice
type AliasSlice = []*int

var (
	Slice      []*int
	NamedSl    NamedSlice
	AliasNamed AliasOfNamed
	AliasSl    AliasSlice
	Int        int
	SlicePtr   *[]int
)
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)
	pkg, err := (&types.Config{}).Check("testpkg", fset, []*ast.File{f}, nil)
	require.NoError(t, err)

	typeOf := func(name string) types.Type {
		obj := pkg.Scope().Lookup(name)
		require.NotNil(t, obj, "object %q not found", name)
		return obj.Type()
	}

	// Unnamed deep types are tracked at the expression's location.
	require.True(t, DeepNilabilityIsLocal(typeOf("Slice")))
	require.True(t, DeepNilabilityIsLocal(typeOf("AliasSl")))
	require.True(t, DeepNilabilityIsLocal(typeOf("SlicePtr")))
	// Named deep types (including via alias) own their deep nilability at the type declaration.
	require.False(t, DeepNilabilityIsLocal(typeOf("NamedSl")))
	require.False(t, DeepNilabilityIsLocal(typeOf("AliasNamed")))
	// Non-deep types have no deep nilability at all.
	require.False(t, DeepNilabilityIsLocal(typeOf("Int")))

	// DeepNilabilityAsNamedType must key the trigger on the type name, resolving aliases.
	for _, name := range []string{"NamedSl", "AliasNamed"} {
		trigger, ok := DeepNilabilityAsNamedType(typeOf(name)).(*SliceRead)
		require.True(t, ok, "expected SliceRead trigger for %q", name)
		key, ok := trigger.Ann.(*TypeNameAnnotationKey)
		require.True(t, ok)
		require.Equal(t, "NamedSlice", key.TypeDecl.Name())
	}
	require.IsType(t, &ProduceTriggerNever{}, DeepNilabilityAsNamedType(typeOf("Slice")))
	require.IsType(t, &ProduceTriggerNever{}, DeepNilabilityAsNamedType(typeOf("Int")))
}
