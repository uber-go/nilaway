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
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsDeeplyArray(t *testing.T) {
	t.Parallel()

	const src = `package testpkg

type NamedArray [8]int
type NamedArray2 NamedArray
type AliasArray = [8]int
type NamedArrayPtr *[8]int
type ArrayConstraint interface{ ~[8]int }
type ArrayConstraint16 interface{ ~[16]int }

var (
	Array      [8]int
	Slice      []int
	NamedArr   NamedArray
	NamedArr2  NamedArray2
	AliasArr   AliasArray
	Ptr        *[8]int
	NamedPtr   NamedArrayPtr
	Int        int
	PtrToSlice *[]int
)

func Generic[A ~[8]int, E ArrayConstraint, U ~[8]int | ~[16]int, X ~[8]int | ~[]int, S ~[]int, M any, IU ArrayConstraint | ArrayConstraint16, IX ArrayConstraint | ~[]int, P ~*[8]int](a A, e E, u U, x X, s S, m M, iu IU, ix IX, p P) {}
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
	params := pkg.Scope().Lookup("Generic").(*types.Func).Signature().Params()
	typeParamOf := func(name string) types.Type {
		for i := 0; i < params.Len(); i++ {
			if p := params.At(i); p.Name() == name {
				return p.Type()
			}
		}
		require.Failf(t, "parameter not found", "parameter %q", name)
		return nil
	}

	tests := []struct {
		name           string
		typ            types.Type
		wantArray      bool
		wantOrArrayPtr bool
	}{
		{"Nil", nil, false, false},
		{"Array", typeOf("Array"), true, true},
		{"Slice", typeOf("Slice"), false, false},
		{"NamedArray", typeOf("NamedArr"), true, true},
		{"NamedArrayOfNamedArray", typeOf("NamedArr2"), true, true},
		{"AliasArray", typeOf("AliasArr"), true, true},
		{"PtrToArray", typeOf("Ptr"), false, true},
		{"NamedPtrToArray", typeOf("NamedPtr"), false, true},
		{"Int", typeOf("Int"), false, false},
		{"PtrToSlice", typeOf("PtrToSlice"), false, false},
		{"TypeParamArray", typeParamOf("a"), true, true},
		{"TypeParamEmbeddedArrayConstraint", typeParamOf("e"), true, true},
		{"TypeParamArrayUnion", typeParamOf("u"), true, true},
		{"TypeParamMixedUnion", typeParamOf("x"), false, false},
		{"TypeParamSlice", typeParamOf("s"), false, false},
		{"TypeParamAny", typeParamOf("m"), false, false},
		// Unions whose terms are themselves (method-less) interfaces are not flattened by
		// go/types, so normalization must recurse into them.
		{"TypeParamInterfaceUnionArrays", typeParamOf("iu"), true, true},
		{"TypeParamInterfaceUnionMixed", typeParamOf("ix"), false, false},
		{"TypeParamPtrToArray", typeParamOf("p"), false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.wantArray, IsDeeplyType[*types.Array](tt.typ), "IsDeeplyType[*types.Array](%v)", tt.typ)
			require.Equal(t, tt.wantOrArrayPtr, IsDeeplyArrayOrArrayPtr(tt.typ), "IsDeeplyArrayOrArrayPtr(%v)", tt.typ)
		})
	}
}

func TestIsDeepAndFriends(t *testing.T) {
	t.Parallel()

	const src = `package testpkg

type NamedSlice []*int
type AliasSlice = []*int
type NamedPtr *[]int
type NamedStruct struct{ f *int }
type NamedFunc func() (*int, error)
type AliasFunc = func() (*int, error)

var (
	Slice        []*int
	NamedSl      NamedSlice
	AliasSl      AliasSlice
	Map          map[string]*int
	Chan         chan *int
	Int          int
	IntPtr       *int
	SlicePtr     *[]int
	NamedP       NamedPtr
	StructVal    NamedStruct
	StructPtr    *NamedStruct
	Iface        interface{}
	Func         NamedFunc
	AliasF       AliasFunc
	PlainFunc    func() (*int, error)
)

func Generic[P ~*int, S ~[]int, PS ~*int | ~[]int, M any](p P, s S, ps PS, m M) {}
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
	params := pkg.Scope().Lookup("Generic").(*types.Func).Signature().Params()
	typeParamOf := func(name string) types.Type {
		for i := 0; i < params.Len(); i++ {
			if p := params.At(i); p.Name() == name {
				return p.Type()
			}
		}
		require.Failf(t, "parameter not found", "parameter %q", name)
		return nil
	}

	t.Run("IsDeep", func(t *testing.T) {
		t.Parallel()
		require.False(t, IsDeep(nil))
		require.True(t, IsDeep(typeOf("Slice")))
		// IsDeep is a purely type-level property: named types and aliases resolve. Whether the
		// deep nilability is tracked locally or at the named type's own annotation site is
		// decided by annotation.DeepNilabilityIsLocal instead.
		require.True(t, IsDeep(typeOf("NamedSl")))
		require.True(t, IsDeep(typeOf("AliasSl")))
		require.True(t, IsDeep(typeOf("Map")))
		require.True(t, IsDeep(typeOf("Chan")))
		require.False(t, IsDeep(typeOf("Int")))
		require.False(t, IsDeep(typeOf("IntPtr")))
		require.True(t, IsDeep(typeOf("SlicePtr")))
		require.False(t, IsDeep(typeOf("StructPtr")))
	})

	t.Run("IsPointer", func(t *testing.T) {
		t.Parallel()
		require.True(t, IsPointer(typeOf("Slice")))
		require.True(t, IsPointer(typeOf("NamedSl")))
		require.True(t, IsPointer(typeOf("AliasSl")))
		require.True(t, IsPointer(typeOf("Map")))
		require.True(t, IsPointer(typeOf("IntPtr")))
		require.True(t, IsPointer(typeOf("NamedP")))
		require.False(t, IsPointer(typeOf("Int")))
		require.False(t, IsPointer(typeOf("StructVal")))
		require.True(t, IsPointer(typeParamOf("p")))
		require.True(t, IsPointer(typeParamOf("s")))
		// Mixed unions are fine as long as every term is pointer-like.
		require.True(t, IsPointer(typeParamOf("ps")))
		require.False(t, IsPointer(typeParamOf("m")))
	})

	t.Run("UnwrapPtr", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, UnwrapPtr(nil))
		require.Equal(t, typeOf("Int"), UnwrapPtr(typeOf("IntPtr")))
		require.Equal(t, typeOf("Slice"), UnwrapPtr(typeOf("Slice")))
		// Named pointer types resolve to their element type.
		require.IsType(t, &types.Slice{}, UnwrapPtr(typeOf("NamedP")))
		require.Equal(t, typeOf("NamedSl"), UnwrapPtr(typeOf("NamedSl")))
	})

	t.Run("GetFuncSignature", func(t *testing.T) {
		t.Parallel()
		require.Nil(t, GetFuncSignature(nil))
		require.Nil(t, GetFuncSignature(typeOf("Int")))
		require.NotNil(t, GetFuncSignature(typeOf("PlainFunc")))
		// Both named function types and aliases resolve to their signature.
		require.NotNil(t, GetFuncSignature(typeOf("Func")))
		require.NotNil(t, GetFuncSignature(typeOf("AliasF")))
	})

	t.Run("TypeBarsNilness", func(t *testing.T) {
		t.Parallel()
		require.True(t, TypeBarsNilness(typeOf("Int")))
		require.False(t, TypeBarsNilness(typeOf("IntPtr")))
		require.False(t, TypeBarsNilness(typeOf("NamedSl")))
		require.False(t, TypeBarsNilness(typeOf("AliasSl")))
		require.False(t, TypeBarsNilness(typeOf("Iface")))
		// Type parameters bar nil unless every term in their type set admits nil.
		require.False(t, TypeBarsNilness(typeParamOf("p")))
		require.False(t, TypeBarsNilness(typeParamOf("ps")))
		require.True(t, TypeBarsNilness(typeParamOf("m")))
	})
}

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
		{"InvalidFuncTooManyArgs", "func(func() bool, string)", false},
		{"InvalidFuncNotFuncType", "func(bool)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := types.NewPackage("testpkg", "testpkg")
			typeInfo, err := types.Eval(token.NewFileSet(), pkg, 0, tt.typeStr)
			if err != nil {
				t.Fatalf("failed to evaluate type: %v", err)
			}

			got := IsIterType(typeInfo.Type)
			require.Equal(t, tt.want, got, "IsIterType(%s) = %v, want %v", tt.typeStr, got, tt.want)
		})
	}
}
