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
	"strings"
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

	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/parse")
	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, &analysishelper.Result[Map]{}, result)
	funcContractsMap := result.(*analysishelper.Result[Map]).Res
	require.NoError(t, result.(*analysishelper.Result[Map]).Err)

	require.NotNil(t, funcContractsMap)

	actual := make(Map)
	for funcObj, contracts := range funcContractsMap {
		actual[funcObj] = contracts
	}

	expected := Map{
		getFuncObj(pass, "f1"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "f2"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{True}},
		},
		getFuncObj(pass, "f3"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{False}},
		},
		getFuncObj(pass, "multipleValues"): {
			Contract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, True}},
		},
		getFuncObj(pass, "multipleContracts"): {
			Contract{Ins: []ContractVal{Any, NonNil}, Outs: []ContractVal{NonNil, True}},
			Contract{Ins: []ContractVal{NonNil, Any}, Outs: []ContractVal{NonNil, True}},
		},
		// function contractCommentInOtherLine should not exist in the map as it has no contract.
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		require.Fail(t, fmt.Sprintf("parsed contracts mismatch (-want +got):\n%s", diff))
	}
}
func TestInfer(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/infer")

	require.Equal(t, 1, len(r))
	require.NotNil(t, r[0])

	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, &analysishelper.Result[Map]{}, result)
	funcContractsMap := result.(*analysishelper.Result[Map]).Res
	require.NoError(t, result.(*analysishelper.Result[Map]).Err)

	require.NotNil(t, funcContractsMap)

	actual := make(Map)
	for funcObj, contracts := range funcContractsMap {
		actual[funcObj] = contracts
	}

	expected := Map{
		getFuncObj(pass, "onlyLocalVar"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "unknownCondition"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "noLocalVar"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "learnUnderlyingFromOuterMakeInterface"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "twoCondsMerge"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "unknownToUnknownButSameValue"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		// other functions should not exist in the map as the contract nonnil->nonnil does not hold
		// for them.

		// TODO: uncomment this when we support field access when inferring contracts.
		// getFuncObj(pass, "field"): {
		//	Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		// },
		// TODO: uncomment this when we support nonempty slice to nonnil.
		// getFuncObj(pass, "nonEmptySliceToNonnil"): {
		//	Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		// },
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		require.Fail(t, fmt.Sprintf("inferred contracts mismatch (-want +got):\n%s", diff))
	}
}

func TestFactExport(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	// The exported facts are asserted in the testdata file themselves in "want" strings.
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/factexport/upstream")
}

func TestFactImport(t *testing.T) {
	t.Parallel()

	// Now we test the import of the contract facts. The downstream package has a dependency on
	// the upstream package (which contains several contracted functions). It should be able to
	// import those facts, combine them with its own contracts, and return the combined map.

	testdata := analysistest.TestData()
	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/factexport/downstream")
	require.Len(t, r, 1)
	pass, result := r[0].Pass, r[0].Result
	require.IsType(t, &analysishelper.Result[Map]{}, result)
	require.NoError(t, result.(*analysishelper.Result[Map]).Err)
	actual := result.(*analysishelper.Result[Map]).Res

	expected := Map{
		getFuncObj(pass, "localManual"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "upstream.ExportedManual"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
		getFuncObj(pass, "upstream.ExportedInferred"): {
			Contract{Ins: []ContractVal{NonNil}, Outs: []ContractVal{NonNil}},
		},
	}
	if diff := cmp.Diff(expected, actual); diff != "" {
		require.Fail(t, fmt.Sprintf("inferred contracts mismatch (-want +got):\n%s", diff))
	}
}

func getFuncObj(pass *analysis.Pass, name string) *types.Func {
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		return pass.Pkg.Scope().Lookup(parts[0]).(*types.Func)
	}
	if len(parts) > 2 {
		panic(fmt.Sprintf("invalid function name to look up, expected name or pkg.name, got %q", name))
	}
	for _, imported := range pass.Pkg.Imports() {
		if imported.Name() == parts[0] {
			return imported.Scope().Lookup(parts[1]).(*types.Func)
		}
	}

	panic(fmt.Sprintf("cannot find function %q", name))
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
