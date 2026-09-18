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

package annotation

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/nilaway/nilawaytest"
)

const _interfaceNameKey = "Key"

// initStructsKey initializes all structs that implement the Key interface
var initStructsKey = []Key{
	&FieldAnnotationKey{},
	&CallSiteParamAnnotationKey{},
	&ParamAnnotationKey{},
	&CallSiteRetAnnotationKey{},
	&RetAnnotationKey{},
	&TypeNameAnnotationKey{},
	&GlobalVarAnnotationKey{},
	&RecvAnnotationKey{},
	&RetFieldAnnotationKey{},
	&EscapeFieldAnnotationKey{},
	&ParamFieldAnnotationKey{},
	&LocalVarAnnotationKey{},
	&StructFieldContextSite{},
}

// TestKeyEqualsSuite runs the test suite for the `equals` method of all the structs that implement
// the `Key` interface.
func TestKeyEqualsSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, nilawaytest.NewEqualsTestSuite(
		_interfaceNameKey,
		".",
		initStructsKey,
		func(k1, k2 Key) bool { return k1.equals(k2) },
	))
}

// TestKeyCopySuite runs the test suite for the `copy` method of all the structs that implement the `Key` interface.
func TestKeyCopySuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, nilawaytest.NewCopyTestSuite(_interfaceNameKey, ".", initStructsKey, func(k Key) Key { return k.copy() }))
}

func newTestFunc(name string, params, results []*types.Var, variadic bool) *types.Func {
	pkg := types.NewPackage("example.com/test", "test")
	sig := types.NewSignatureType(nil, nil, nil, types.NewTuple(params...), types.NewTuple(results...), variadic)
	return types.NewFunc(token.NoPos, pkg, name, sig)
}

func TestParamAnnotationKeyConstructors(t *testing.T) {
	t.Parallel()

	first := types.NewVar(token.NoPos, nil, "value", types.Typ[types.Int])
	second := types.NewVar(token.NoPos, nil, "", types.Typ[types.Int])
	fn := newTestFunc("fn", []*types.Var{first, second}, nil, false)

	byNum := ParamKeyFromArgNum(fn, 0)
	require.Equal(t, 0, byNum.ParamNum)
	require.Equal(t, "Param 0: 'value' of Function fn", byNum.String())
	require.Equal(t, "arg `value`", byNum.MinimalString())
	require.Equal(t, "value", byNum.ParamNameString())

	byName := ParamKeyFromName(fn, first)
	require.Equal(t, byNum, byName)
	require.Panics(t, func() { ParamKeyFromName(fn, types.NewVar(token.NoPos, nil, "missing", types.Typ[types.Int])) })
	require.Panics(t, func() { ParamKeyFromArgNum(fn, 2) })
}

func TestCallSiteParamKeyVariadicNormalization(t *testing.T) {
	t.Parallel()

	value := types.NewVar(token.NoPos, nil, "value", types.Typ[types.Int])
	variadic := types.NewVar(token.NoPos, nil, "values", types.NewSlice(types.Typ[types.Int]))
	fn := newTestFunc("fn", []*types.Var{value, variadic}, nil, true)
	location := token.Position{Filename: "call.go", Line: 4, Column: 2}

	key := NewCallSiteParamKey(fn, 5, location)
	require.Equal(t, 1, key.ParamNum)
	require.Equal(t, "values", key.ParamNameString())
	require.Equal(t, "arg `values`", key.MinimalString())
	require.Equal(t, "Param 1: 'values' of Function fn at Location call.go:4:2", key.String())
}
