//  Copyright (c) 2024 Uber Technologies, Inc.
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

package analysishelper

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"
)

func TestWrapPanic_Panic(t *testing.T) {
	t.Parallel()

	// Test that WrapRun recovers from a panic and returns the panic as an error.
	panickingFunc := func(*analysis.Pass) (int, error) { panic("panic") }
	wrapped := WrapRun(panickingFunc)
	r, err := wrapped(nil /* pass */)
	// No error should be returned, since the converted error (from the recovered panic) is
	// returned via the result struct.
	require.NoError(t, err)

	require.IsType(t, &Result[int]{}, r)
	require.Empty(t, r.(*Result[int]).Res)
	require.ErrorContains(t, r.(*Result[int]).Err, "INTERNAL PANIC")
}

func TestWrapPanic_Error(t *testing.T) {
	t.Parallel()

	// Test that WrapRun does not recover from a panic and returns the panic as an error.
	errFunc := func(*analysis.Pass) (int, error) { return 0, errors.New("my error") }
	wrapped := WrapRun(errFunc)
	r, err := wrapped(nil /* pass */)
	require.NoError(t, err)

	require.IsType(t, &Result[int]{}, r)
	require.Empty(t, r.(*Result[int]).Res)
	require.ErrorContains(t, r.(*Result[int]).Err, "my error")
}

func TestWrapPanic_NoPanic(t *testing.T) {
	t.Parallel()

	// Test that WrapRun does not recover from a panic and returns the panic as an error.
	nonPanickingFunc := func(*analysis.Pass) (int, error) { return 42, nil }
	wrapped := WrapRun(nonPanickingFunc)
	r, err := wrapped(nil)
	// No error should be returned, since the converted error (from the recovered panic) is
	// returned via the result struct.
	require.NoError(t, err)

	require.IsType(t, &Result[int]{}, r)
	require.NoError(t, r.(*Result[int]).Err)
	require.Equal(t, 42, r.(*Result[int]).Res)
}
