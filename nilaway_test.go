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

// Go 1.22 [1] introduces a proper `types.Alias` type for type aliases. The current default is
// disabling such a feature. However, Go official doc suggests that it will be enabled in future Go
// releases. Therefore, here we explicitly set this to `1` to enable the feature to test NilAway's
// ability to handle it.
// [1]: https://tip.golang.org/doc/go1.22
//go:debug gotypesalias=1

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

func TestNilAway(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	// For descriptions of the purpose of each of the following tests, consult their source files
	// located in testdata/src/<package>.

	tests := []struct {
		name     string
		patterns []string
	}{
		{name: "Inference", patterns: []string{"go.uber.org/inference"}},
		{name: "Contracts", patterns: []string{"go.uber.org/contracts", "go.uber.org/contracts/namedtypes", "go.uber.org/contracts/inference"}},
		{name: "TrustedFunc", patterns: []string{"go.uber.org/trustedfunc", "go.uber.org/trustedfunc/inference"}},
		{name: "ErrorReturn", patterns: []string{"go.uber.org/errorreturn", "go.uber.org/errorreturn/inference"}},
		{name: "Maps", patterns: []string{"go.uber.org/maps"}},
		{name: "Slices", patterns: []string{"go.uber.org/slices", "go.uber.org/slices/inference"}},
		{name: "Arrays", patterns: []string{"go.uber.org/arrays"}},
		{name: "Channels", patterns: []string{"go.uber.org/channels"}},
		{name: "GoQuirks", patterns: []string{"go.uber.org/goquirks"}},
		{name: "GlobalVars", patterns: []string{"go.uber.org/globalvars"}},
		{name: "DeepNil", patterns: []string{"go.uber.org/deepnil", "go.uber.org/deepnil/inference"}},
		{name: "NilableTypes", patterns: []string{"go.uber.org/nilabletypes"}},
		{name: "HelloWorld", patterns: []string{"go.uber.org/helloworld"}},
		{name: "MultiFilePackage", patterns: []string{"go.uber.org/multifilepackage", "go.uber.org/multifilepackage/firstpackage", "go.uber.org/multifilepackage/secondpackage"}},
		{name: "MultipleAssignment", patterns: []string{"go.uber.org/multipleassignment"}},
		{name: "AnnotationParse", patterns: []string{"go.uber.org/annotationparse"}},
		{name: "NilCheck", patterns: []string{"go.uber.org/nilcheck"}},
		{name: "SimpleFlow", patterns: []string{"go.uber.org/simpleflow"}},
		{name: "LoopFlow", patterns: []string{"go.uber.org/loopflow"}},
		{name: "MethodImplementation", patterns: []string{"go.uber.org/methodimplementation", "go.uber.org/methodimplementation/mergedDependencies", "go.uber.org/methodimplementation/chainedDependencies", "go.uber.org/methodimplementation/multipackage", "go.uber.org/methodimplementation/embedding"}},
		{name: "NamedReturn", patterns: []string{"go.uber.org/namedreturn"}},
		{name: "IgnoreGenerated", patterns: []string{"go.uber.org/ignoregenerated"}},
		{name: "IgnorePackage", patterns: []string{"ignoredpkg1", "ignoredpkg2"}},
		{name: "Receivers", patterns: []string{"go.uber.org/receivers", "go.uber.org/receivers/inference"}},
		{name: "Generics", patterns: []string{"go.uber.org/generics"}},
		{name: "FunctionContracts", patterns: []string{"go.uber.org/functioncontracts", "go.uber.org/functioncontracts/inference"}},
		{name: "Constants", patterns: []string{"go.uber.org/consts"}},
		{name: "LoopRange", patterns: []string{"go.uber.org/looprange"}},
		{name: "AbnormalFlow", patterns: []string{"go.uber.org/abnormalflow"}},
		{name: "NoLint", patterns: []string{"go.uber.org/nolint"}},
		{name: "Templ", patterns: []string{"go.uber.org/templ"}},
		{name: "FuncVariable", patterns: []string{"go.uber.org/funcvariable"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			t.Logf("Running test for packages %s", tt.patterns)

			analysistest.Run(t, testdata, Analyzer, tt.patterns...)
		})
	}
}

func TestStructInit(t *testing.T) { //nolint:paralleltest
	// We specifically do not set this test to be parallel since we need to enable the
	// experimental support for struct initialization to test this feature.
	err := config.Analyzer.Flags.Set(config.ExperimentalStructInitEnableFlag, "true")
	require.NoError(t, err)
	defer func() {
		err := config.Analyzer.Flags.Set(config.ExperimentalStructInitEnableFlag, "false")
		require.NoError(t, err)
	}()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/structinit/funcreturnfields", "go.uber.org/structinit/local", "go.uber.org/structinit/global", "go.uber.org/structinit/paramfield", "go.uber.org/structinit/paramsideeffect", "go.uber.org/structinit/defaultfield")
}

func TestAnonymousFunction(t *testing.T) { //nolint:paralleltest
	// We specifically do not set this test to be parallel since we need to enable the
	// experimental support for anonymous function to test this feature.
	err := config.Analyzer.Flags.Set(config.ExperimentalAnonymousFunctionFlag, "true")
	require.NoError(t, err)
	defer func() {
		err := config.Analyzer.Flags.Set(config.ExperimentalAnonymousFunctionFlag, "false")
		require.NoError(t, err)
	}()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "go.uber.org/anonymousfunction")
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

func TestGroupErrorMessages(t *testing.T) { //nolint:paralleltest
	// We specifically do not set this test to be parallel such that this test is run separately
	// from the parallel tests. This makes it possible to test the group error messages flag independently
	// without affecting the other tests.
	testdata := analysistest.TestData()

	defaultValue := config.Analyzer.Flags.Lookup(config.GroupErrorMessagesFlag).Value.String()

	err := config.Analyzer.Flags.Set(config.GroupErrorMessagesFlag, "true")
	require.NoError(t, err)
	analysistest.Run(t, testdata, Analyzer, "grouping/enabled")
	analysistest.Run(t, testdata, Analyzer, "grouping/errormessage", "grouping/errormessage/inference")

	err = config.Analyzer.Flags.Set(config.GroupErrorMessagesFlag, "false")
	require.NoError(t, err)
	analysistest.Run(t, testdata, Analyzer, "grouping/disabled")

	// Reset the flag to its default value.
	defer func() {
		err := config.Analyzer.Flags.Set(config.GroupErrorMessagesFlag, defaultValue)
		require.NoError(t, err)
	}()
}

func TestPrintFullFilePath(t *testing.T) { //nolint:paralleltest
	// We specifically do not set this test to be parallel such that this test is run separately
	// from the parallel tests. This makes it possible to set the print-full-file-path flag to true for
	// testing and false for the other tests.
	err := config.Analyzer.Flags.Set(config.PrintFullFilePathFlag, "true")
	require.NoError(t, err)
	defer func() {
		err := config.Analyzer.Flags.Set(config.PrintFullFilePathFlag, "false")
		require.NoError(t, err)
	}()

	testdata := analysistest.TestData()
	analysistest.Run(t, testdata, Analyzer, "printfullfilepath")
}

func TestMain(m *testing.M) {
	flags := map[string]string{
		// Pretty print should be turned off for easier error message matching in test files.
		config.PrettyPrintFlag: "false",
		// Error message grouping should be turned off for easier matching in test files.
		config.GroupErrorMessagesFlag:    "false",
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
