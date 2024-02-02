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

package function

import (
	"context"
	"go/ast"
	"go/types"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/assertion/anonymousfunc"
	"go.uber.org/nilaway/assertion/function/assertiontree"
	"go.uber.org/nilaway/assertion/function/functioncontracts"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
)

func TestTimeout(t *testing.T) {
	t.Parallel()

	testdata := analysistest.TestData()

	// First do an analysis test run just to get the pass variable.
	r := analysistest.Run(t, testdata, Analyzer, "go.uber.org/pkg")
	pass := r[0].Pass

	// Select the first function declaration node to run test.
	var funcDecl *ast.FuncDecl
	for _, file := range pass.Files {
		for _, decl := range file.Decls {
			if f, ok := decl.(*ast.FuncDecl); ok {
				funcDecl = f
				break
			}
		}
	}
	require.NotNil(t, funcDecl, "Cannot find a function declaration in test code")

	// Prepare the input variables:
	// (1) Enable all features flags (will not actually make a difference since our test code does
	// not really require such features).
	funcConfig := assertiontree.FunctionConfig{
		EnableStructInitCheck: true,
		EnableAnonymousFunc:   true,
	}
	// (2) Construct an empty function context. In normal NilAway execution the func lit map and
	// pkg fake ident map will be created from the separate anonymous function analyzer. However,
	// since our test code does not contain any anonymous function, an empty map will have the same
	// effect.
	emptyFuncLitMap := make(map[*ast.FuncLit]*anonymousfunc.FuncLitInfo)
	emptyPkgFakeIdentMap := make(map[*ast.Ident]types.Object)
	emptyFuncContracts := make(functioncontracts.Map)
	funcContext := assertiontree.NewFunctionContext(pass, funcDecl, nil, /* funcLit */
		funcConfig, emptyFuncLitMap, emptyPkgFakeIdentMap, emptyFuncContracts)
	// (3) Set up synchronization and communication for the goroutine we are going to spawn.
	resultChan := make(chan functionResult)
	wg := new(sync.WaitGroup)
	wg.Add(1)

	// Give a cancelled context, so back propagation should immediately return with an error.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ctrlflowResult := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	go analyzeFunc(ctx, pass, funcDecl, funcContext, ctrlflowResult.FuncDecl(funcDecl), 0, resultChan, wg)

	// Spawn a goroutine to wait and close the result channel when the work is done.
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Since we have passed a cancelled context, the goroutine should immediately return with a
	// Canceled error.
	res := <-resultChan
	require.Equal(t, res.index, 0)
	require.ErrorIs(t, res.err, context.Canceled)
}

func TestAnalyzeFuncPanic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	resultChan := make(chan functionResult)
	var wg sync.WaitGroup
	wg.Add(1)

	// Intentionally give bad input data to cause a panic. We should convert the panic to an error
	// and send it back to the original channel.
	go analyzeFunc(ctx,
		&analysis.Pass{},                /* pass */
		&ast.FuncDecl{},                 /* funcDecl */
		assertiontree.FunctionContext{}, /* funcContext */
		&cfg.CFG{},                      /* graph */
		0,                               /* index */
		resultChan,
		&wg,
	)
	// Fire up another goroutine that waits for the work to be done and closes the result channel.
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	res := <-resultChan
	require.Equal(t, res.index, 0)
	require.ErrorContains(t, res.err, "panic")
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
