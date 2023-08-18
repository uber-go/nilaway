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
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestParse(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/functioncontracts/parse")
	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, Result{}, result)
	funcContractsMap := result.(Result).FunctionContracts

	require.NotNil(t, funcContractsMap)

	actualNameToContracts := map[string][]*FunctionContract{}
	for funcID, contracts := range funcContractsMap {
		actualNameToContracts[funcID] = contracts
	}

	expectedNameToContracts := map[string][]*FunctionContract{
		getFuncID(pass, "f1"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "f2"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{True}},
		},
		getFuncID(pass, "f3"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{False}},
		},
		getFuncID(pass, "multipleValues"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, True}},
		},
		getFuncID(pass, "multipleContracts"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, True}},
			&FunctionContract{Ins: []ContractVal{NonNil, Any}, Outs: []ContractVal{NonNil, True}},
		},
		getFuncID(pass, "ExportedFromParse"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
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
	require.IsType(t, Result{}, result)
	funcContractsMap := result.(Result).FunctionContracts

	require.NotNil(t, funcContractsMap)

	actualNameToContracts := map[string][]*FunctionContract{}
	for funcObj, contracts := range funcContractsMap {
		actualNameToContracts[funcObj] = contracts
	}

	expectedNameToContracts := map[string][]*FunctionContract{
		getFuncID(pass, "onlyLocalVar"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "unknownCondition"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "noLocalVar"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "learnUnderlyingFromOuterMakeInterface"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "ExportedFromInfer"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "twoCondsMerge"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "unknownToUnknownButSameValue"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "nonnilAnyToNonnilAny"): {
			&FunctionContract{Ins: []ContractVal{NonNil, Any}, Outs: []ContractVal{NonNil, Any}},
		},
		getFuncID(pass, "nonnilAnyToAnyNonnil"): {
			&FunctionContract{Ins: []ContractVal{NonNil, Any}, Outs: []ContractVal{Any, NonNil}},
		},
		getFuncID(pass, "anyNonnilToNonnilAny"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, Any}},
		},
		getFuncID(pass, "anyNonnilToAnyNonnil"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{Any, NonNil}},
		},
		getFuncID(pass, "anyNonnilAnyToAnyAnyNonnilAny"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil, Any}, Outs: []ContractVal{Any, Any, NonNil, Any}},
		},
		getFuncID(pass, "mixType"): {
			&FunctionContract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{Any, NonNil}},
		},
		getFuncID(pass, "ExportedFromInfer"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		// other functions should not exist in the map as the contract nonnil->nonnil does not hold
		// for them.

		// TODO: uncomment this when we support field access when inferring contracts.
		//getFuncID(pass, "field"): {
		//	&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		//},
		// TODO: uncomment this when we support nonempty slice to nonnil.
		//getFuncID(pass, "nonEmptySliceToNonnil"): {
		//	&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		//},
	}
	if diff := cmp.Diff(expectedNameToContracts, actualNameToContracts); diff != "" {
		require.Fail(t, fmt.Sprintf("inferred contracts mismatch (-want +got):\n%s", diff))
	}
}

func TestUpstream(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/functioncontracts")
	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, Result{}, result)
	funcContractsMap := result.(Result).FunctionContracts

	require.NotNil(t, funcContractsMap)

	actualNameToContracts := map[string][]*FunctionContract{}
	for funcObj, contracts := range funcContractsMap {
		actualNameToContracts[funcObj] = contracts
	}

	expectedNameToContracts := map[string][]*FunctionContract{
		getFuncID(pass, "ExportedFromParse"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncID(pass, "ExportedFromInfer"): {
			&FunctionContract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
	}
	if diff := cmp.Diff(expectedNameToContracts, actualNameToContracts); diff != "" {
		require.Fail(t, fmt.Sprintf("inferred contracts mismatch (-want +got):\n%s", diff))
	}
}

func getFuncID(pass *analysis.Pass, name string) string {
	obj := pass.Pkg.Scope().Lookup(name)
	if obj == nil {
		for _, iPkg := range pass.Pkg.Imports() {
			obj = iPkg.Scope().Lookup(name)
			if obj != nil {
				break
			}
		}
	}
	return obj.(*types.Func).FullName()
}
