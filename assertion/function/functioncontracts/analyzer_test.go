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
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	t.Parallel()

	// Intentionally give a nil pass variable to trigger a panic, but we should recover from it
	// and convert it to an error via the result struct.
	r, err := Analyzer.Run(nil /* pass */)
	require.NoError(t, err)
	require.ErrorContains(t, r.(*analysishelper.Result[Map]).Err, "INTERNAL PANIC")
}

func TestContractCollection(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/functioncontracts/parse")
	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, &analysishelper.Result[Map]{}, result)
	funcContractsMap := result.(*analysishelper.Result[Map]).Res
	require.NoError(t, result.(*analysishelper.Result[Map]).Err)

	require.NotNil(t, funcContractsMap)

	actualNameToContracts := map[*types.Func][]*FunctionContract{}
	for funcObj, contracts := range funcContractsMap {
		actualNameToContracts[funcObj] = contracts
	}

	expectedNameToContracts := map[*types.Func][]*FunctionContract{
		getFuncObj(pass, "f1"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "f2"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{True}},
		},
		getFuncObj(pass, "f3"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{False}},
		},
		getFuncObj(pass, "multipleValues"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, True}},
		},
		getFuncObj(pass, "multipleContracts"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, True}},
			&FunctionContract{Ins: []ContractVal{NonNil, Any}, Outs: []ContractVal{NonNil, True}},
		},
		// function contractCommentInOtherLine should not exist in the map as it has no contract.
	}
	if diff := cmp.Diff(expectedNameToContracts, actualNameToContracts); diff != "" {
		require.Fail(t, fmt.Sprintf("parsed contracts mismatch (-want +got):\n%s", diff))
	}
}
func TestInfer(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/functioncontracts/infer")

	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, &analysishelper.Result[Map]{}, result)
	funcContractsMap := result.(*analysishelper.Result[Map]).Res

	require.NotNil(t, funcContractsMap)

	actualNameToContracts := map[*types.Func][]*FunctionContract{}
	for funcObj, contracts := range funcContractsMap {
		actualNameToContracts[funcObj] = contracts
	}

	expectedNameToContracts := map[*types.Func][]*FunctionContract{
		getFuncObj(pass, "onlyLocalVar"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "unknownCondition"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "noLocalVar"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "learnUnderlyingFromOuterMakeInterface"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "twoCondsMerge"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "unknownToUnknownButSameValue"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		// other functions should not exist in the map as the contract nonnil->nonnil does not hold
		// for them.

		// TODO: uncomment this when we support field access when inferring contracts.
		// getFuncObj(pass, "field"): {
		//	&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		// },
		// TODO: uncomment this when we support nonempty slice to nonnil.
		// getFuncObj(pass, "nonEmptySliceToNonnil"): {
		//	&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		// },
	}
	if diff := cmp.Diff(expectedNameToContracts, actualNameToContracts); diff != "" {
		require.Fail(t, fmt.Sprintf("inferred contracts mismatch (-want +got):\n%s", diff))
	}
}

func getFuncObj(pass *analysis.Pass, name string) *types.Func {
	return pass.Pkg.Scope().Lookup(name).(*types.Func)
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
