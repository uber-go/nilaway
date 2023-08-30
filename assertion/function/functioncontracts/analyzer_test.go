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

package functioncontracts

import (
	"fmt"
	"go/types"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/functioncontracts")
	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, Result{}, result)
	funcContractsMap := result.(Result).FunctionContracts

	require.NotNil(t, funcContractsMap)

	actualNameToContracts := map[*types.Func][]*FunctionContract{}
	for funcObj, contracts := range funcContractsMap {
		actualNameToContracts[funcObj] = contracts
	}

	var getFuncObj = func(name string) *types.Func {
		return pass.Pkg.Scope().Lookup(name).(*types.Func)
	}
	expectedNameToContracts := map[*types.Func][]*FunctionContract{
		getFuncObj("f1"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj("f2"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{True}},
		},
		getFuncObj("f3"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{False}},
		},
		getFuncObj("multipleValues"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, True}},
		},
		getFuncObj("multipleContracts"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, True}},
			&FunctionContract{Ins: []ContractVal{NonNil, Any}, Outs: []ContractVal{NonNil, True}},
		},
		// function contractCommentInOtherLine should not exist in the map as it has no contract.
	}
	if diff := cmp.Diff(expectedNameToContracts, actualNameToContracts); diff != "" {
		require.Fail(t, fmt.Sprintf("parsed contracts mismatch (-want +got):\n%s", diff))
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
