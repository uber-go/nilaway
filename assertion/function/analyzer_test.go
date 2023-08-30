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
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/nilaway/assertion/anonymousfunc"
	"go.uber.org/nilaway/assertion/function/assertiontree"
	"go.uber.org/nilaway/assertion/function/functioncontracts"
	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
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
		StructInitCheckType: config.DepthOneFieldCheck,
		EnableAnonymousFunc: true,
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

	// Give a context that immediately times out, so backprop should return with an error.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now())
	defer cancel()
	ctrlflowResult := pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)
	go analyzeFunc(ctx, pass, funcDecl, funcContext, ctrlflowResult.FuncDecl(funcDecl), 0, resultChan, wg)

	// Spawn a goroutine to wait and close the result channel when the work is done.
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Since we have passed a timed out context, the goroutine should immediately return with a
	// DeadlineExceeded error. Here we wait up to 10 seconds before we force fail.
	select {
	case res := <-resultChan:
		require.Equal(t, res.index, 0)
		require.ErrorIs(t, res.err, context.DeadlineExceeded)
	case <-time.After(10 * time.Second):
		require.Fail(t, "A cancelled context was given to backprop, but it did not return within 10 seconds.")
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
