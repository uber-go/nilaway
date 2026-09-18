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
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInterfaceFlowTriggers(t *testing.T) {
	t.Parallel()

	interfaceParam := types.NewVar(token.NoPos, nil, "value", types.Typ[types.Int])
	implementingParam := types.NewVar(token.NoPos, nil, "value", types.Typ[types.Int])
	interfaceResult := types.NewVar(token.NoPos, nil, "", types.Typ[types.Int])
	implementingResult := types.NewVar(token.NoPos, nil, "", types.Typ[types.Int])
	affiliation := AffiliationPair{
		InterfaceMethod:    newTestFunc("InterfaceMethod", []*types.Var{interfaceParam}, []*types.Var{interfaceResult}, false),
		ImplementingMethod: newTestFunc("ImplementingMethod", []*types.Var{implementingParam}, []*types.Var{implementingResult}, false),
	}

	paramFlow := FullTriggerForInterfaceParamFlow(affiliation, 0)
	require.Equal(t, "value", paramFlow.Producer.Annotation.(*InterfaceParamReachesImplementation).Ann.(*ParamAnnotationKey).ParamNameString())
	require.Equal(t, "value", paramFlow.Consumer.Annotation.(*MethodParamFromInterface).Ann.(*ParamAnnotationKey).ParamNameString())
	require.Equal(t, "InterfaceMethod", paramFlow.Producer.Expr.(*ast.Ident).Name)
	require.Equal(t, "ImplementingMethod", paramFlow.Consumer.Expr.(*ast.Ident).Name)

	resultFlow := FullTriggerForInterfaceResultFlow(affiliation, 0)
	require.Equal(t, 0, resultFlow.Producer.Annotation.(*MethodResultReachesInterface).Ann.(*RetAnnotationKey).RetNum)
	require.Equal(t, 0, resultFlow.Consumer.Annotation.(*InterfaceResultFromImplementation).Ann.(*RetAnnotationKey).RetNum)
	require.Equal(t, "ImplementingMethod", resultFlow.Producer.Expr.(*ast.Ident).Name)
	require.Equal(t, "InterfaceMethod", resultFlow.Consumer.Expr.(*ast.Ident).Name)
}
