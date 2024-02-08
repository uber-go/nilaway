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

// Package namedtypes: This package tests that named types are correctly handled for contract types (e.g., error
// returning functions and ok-form for functions)
package namedtypes

// the below test uses the built-in name `bool` for creating a user-defined named type. However, the logic for determining
// an ok-form function should not depend on the name `bool`, but the underlying type. This test ensures that the logic.
type bool int

func retPtrBoolNamed() (*int, bool) {
	return nil, 0
}

func testNamedBool() {
	if v, ok := retPtrBoolNamed(); ok == 0 {
		_ = *v // want "dereferenced"
	}
}

// Similar to the above test, but with the built-in name `error`
type error int

func retPtrErrorNamed() (*int, error) {
	return nil, 0
}

func testNamedError() {
	if v, ok := retPtrErrorNamed(); ok == 0 {
		_ = *v // want "dereferenced"
	}
}
