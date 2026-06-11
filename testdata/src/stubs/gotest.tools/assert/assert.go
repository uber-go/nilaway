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

// these stubs simulate the legacy v1/v2 `gotest.tools/assert` package (the pre-`/v3` import
// path), whose assertion functions have identical semantics to `gotest.tools/v3/assert`

type TestingT interface {
	FailNow()
}

// nilable(err)
func NilError(t TestingT, err error, msgAndArgs ...interface{}) {}

// nilable(err)
func Error(t TestingT, err error, expected string, msgAndArgs ...interface{}) {}
