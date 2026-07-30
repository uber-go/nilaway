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
package convey

// these stubs simulate the real goconvey `convey` package because we can't import it in tests

type assertion func(actual interface{}, expected ...interface{}) string

func alwaysPass(_ interface{}, _ ...interface{}) string { return "" }

// In the real package, the `Should*` assertions are package-level variables re-exported from the
// assertions package (e.g., `var ShouldBeNil = assertions.ShouldBeNil`); they are declared as
// variables here as well so that the resolution logic is exercised against vars, not funcs.
var (
	ShouldBeNil    assertion = alwaysPass
	ShouldNotBeNil assertion = alwaysPass
	ShouldBeError  assertion = alwaysPass
	ShouldBeTrue   assertion = alwaysPass
	ShouldBeFalse  assertion = alwaysPass
	ShouldEqual    assertion = alwaysPass
)

// nilable(actual, expected)
func So(actual interface{}, assert assertion, expected ...interface{}) {}

func Convey(items ...interface{}) {}
