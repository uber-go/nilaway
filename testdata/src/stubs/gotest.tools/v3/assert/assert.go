//  Copyright (c) 2026 Uber Technologies, Inc.
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

// <nilaway no inference>
package assert

// these stubs simulate the real `gotest.tools/v3/assert` package because we can't import it in tests

type TestingT interface {
	FailNow()
}

// BoolOrComparison can be a bool, an error (nil means success), or a `cmp.Comparison`.
type BoolOrComparison interface{}

// nilable(comparison)
func Assert(t TestingT, comparison BoolOrComparison, msgAndArgs ...interface{}) {}

// nilable(err)
func NilError(t TestingT, err error, msgAndArgs ...interface{}) {}

// nilable(err)
func Error(t TestingT, err error, expected string, msgAndArgs ...interface{}) {}

// nilable(err)
func ErrorContains(t TestingT, err error, substring string, msgAndArgs ...interface{}) {}

// nilable(err, expected)
func ErrorIs(t TestingT, err error, expected error, msgAndArgs ...interface{}) {}
