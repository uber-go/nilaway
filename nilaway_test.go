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

package nilaway

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis/analysistest"
)

// For descriptions of the purpose of each of the following tests, consult their source files
// located in testdata/src/<testname>/<testname>.go

func TestInference(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/inference")
}

func TestContracts(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/contracts")
}

func TestTesting(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/testing")
}

func TestErrorReturn(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/errorreturn", "go.uber.org/errorreturn/inference")
}

func TestMaps(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/maps")
}

func TestSlices(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/slices", "go.uber.org/slices/inference")
}

func TestArrays(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/arrays")
}

func TestChannels(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/channels")
}

func TestGoQuirks(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/goquirks")
}
func TestStructInit(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/structinit/funcreturnfields", "go.uber.org/structinit/local", "go.uber.org/structinit/global", "go.uber.org/structinit/paramfield", "go.uber.org/structinit/paramsideeffect", "go.uber.org/structinit/defaultfield", "go.uber.org/structinit/optimization")
}

func TestGlobalVars(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/globalvars")
}

func TestDeepNil(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/deepnil")
}

func TestNilableTypes(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	analysistest.Run(t, testdata, Analyzer, "go.uber.org/nilabletypes")
}

func TestHelloWorld(t *testing.T) {
	t.Parallel()
	testdata := analysistest.TestData()

	analysistest.Run(t, testdata, Analyzer, "go.uber.org/helloworld")
}

func TestMultiFilePackage(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/multifilepackage", "go.uber.org/multifilepackage/firstpackage", "go.uber.org/multifilepackage/secondpackage")
}

func TestMultipleAssignment(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/multipleassignment")
}

func TestAnnotationParse(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/annotationparse")
}

func TestNilCheck(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/nilcheck")
}

func TestSimpleFlow(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/simpleflow")
}

func TestLoopFlow(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/loopflow")
}

func TestMethodImplementation(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/methodimplementation", "go.uber.org/methodimplementation/mergedDependencies", "go.uber.org/methodimplementation/chainedDependencies", "go.uber.org/methodimplementation/multipackage", "go.uber.org/methodimplementation/embedding")

}

func TestNamedReturn(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/namedreturn")
}

func TestIgnoreGenerated(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/ignoregenerated")
}

func TestIgnorePackage(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "ignoredpkg1", "ignoredpkg2")
}

func TestAnonymousFunction(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/anonymousfunction")
}

func TestReceivers(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/receivers", "go.uber.org/receivers/inference")
}

func TestGenerics(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/generics")
}

func TestFunctionContracts(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/functioncontracts", "go.uber.org/functioncontracts/inference")
}

func TestPrettyPrint(t *testing.T) { //nolint:paralleltest
	// We specifically do not set this test to be parallel such that this test is run separately
	// from the parallel tests. This makes it possible to set the pretty-print flag to true for
	// testing and false for the other tests.
	err := config.Analyzer.Flags.Set(config.PrettyPrintFlag, "true")
	require.NoError(t, err)
	defer func() {
		err := config.Analyzer.Flags.Set(config.PrettyPrintFlag, "false")
		require.NoError(t, err)
	}()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "prettyprint")
}

func TestMain(m *testing.M) {
	flags := map[string]string{
		// Pretty print should be turned off for easier error message matching in test files.
		config.PrettyPrintFlag:           "false",
		config.ExcludeFileDocStringsFlag: "@generated,Code generated by",
		config.ExcludePkgsFlag:           "ignoredpkg1,ignoredpkg2",
	}
	for f, v := range flags {
		if err := config.Analyzer.Flags.Set(f, v); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to set config flag %s with %s: %s", f, v, err)
			os.Exit(1)
		}
	}
	goleak.VerifyTestMain(m)
}
