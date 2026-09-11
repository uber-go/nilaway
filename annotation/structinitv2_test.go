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

package annotation

import (
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStructFieldContextRepresentations(t *testing.T) {
	t.Parallel()

	fn := newTestFunc("fn", nil, nil, false)
	root := &StructFieldContextSite{FuncObj: fn, Kind: StructFieldReturnContext, Index: 0}
	path := &StructFieldContextSite{
		FuncObj:  fn,
		Kind:     StructFieldParamContext,
		Index:    1,
		Path:     NewFieldPath("Child"),
		Location: token.Position{Filename: "call.go", Line: 2, Column: 3},
	}

	require.Equal(t, "value of kind 0 result 0 of `fn`", root.String())
	require.Equal(t, "field `Child` of kind 1 param 1 of `fn` at call.go:2:3", path.String())
	require.Equal(t, "uninitialized field `Child`", (&StructFieldNil{FieldName: "Child"}).Repr().String())
	require.Equal(t, "result 0 of `fn`", (StructFieldFromContextRepr{Kind: StructFieldReturnContext, Index: 0, FuncName: "fn"}).String())
	require.Equal(t, "field `Child` reaches param 1 of `fn`", (StructFieldToContextRepr{Path: "Child", Kind: StructFieldParamContext, Index: 1, FuncName: "fn"}).String())
	require.Equal(t, "method receiver of `fn`", boundaryDesc(StructFieldParamContext, ReceiverParamIndex, "fn"))
}
