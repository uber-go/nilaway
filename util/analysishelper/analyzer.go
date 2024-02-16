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

// Package analysishelper provides helper functions for the `go/analysis` package.
package analysishelper

import (
	"fmt"
	"runtime/debug"

	"golang.org/x/tools/go/analysis"
)

// Result is the result struct for the sub-analyzers where the actual result is accompanied by
// an optional error.
type Result[T any] struct {
	// Res is the actual result from the sub-analyzer.
	Res T
	// Err is the optional error from the sub-analyzer.
	Err error
}

// WrapRun wraps the run function of an analyzer to:
// (1) convert the return type to Result[T] and put the error in the Result[T].Err field in order
// to _not_ stop the analysis and let upper-level analyzer to decide what to do.
// (2) recover from a panic and convert it to an error with stack traces for easier debugging.
// This is to ensure that NilAway _never_ panics during the analysis.
// Moreover, it also wraps the error from the sub-analyzer with the name of the analyzer to make
// it easier to identify the source of the error.
func WrapRun[T any](f func(*analysis.Pass) (T, error)) func(*analysis.Pass) (any, error) {
	wrapped := func(pass *analysis.Pass) (result any, _ error) {
		result = &Result[T]{}
		analyzerName := ""
		if pass != nil && pass.Analyzer != nil {
			analyzerName = pass.Analyzer.Name
		}
		defer func() {
			if r := recover(); r != nil {
				result.(*Result[T]).Err = fmt.Errorf("INTERNAL PANIC from %q: %s\n%s", analyzerName, r, string(debug.Stack()))
			}
		}()

		r, err := f(pass)
		if err != nil {
			// Prefix the error with the name of the analyzer to make it easier to identify the source
			// of the error.
			err = fmt.Errorf("%s: %w", analyzerName, err)
		}
		result.(*Result[T]).Res = r
		result.(*Result[T]).Err = err
		return result, nil
	}

	return wrapped
}
