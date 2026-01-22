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

/*
These tests check nil tracking through function variables.

<nilaway no inference>
*/
package funcvariable

type Config struct {
	Value string
}

// nilable(result 0)
func nilableFunc() *Config {
	return nil
}

func nonNilFunc() *Config {
	return &Config{Value: "test"}
}

// Package-level function variable pointing to a nilable function
var NilableFuncVar = nilableFunc

// Package-level function variable pointing to a non-nil function
var NonNilFuncVar = nonNilFunc

// Test 1: Direct call to nilable function - should be detected
func testDirectNilableCall() {
	cfg := nilableFunc()
	_ = cfg.Value //want "accessed field"
}

// Test 2: Call through function variable pointing to nilable function - should be detected
func testFuncVarNilableCall() {
	cfg := NilableFuncVar()
	_ = cfg.Value //want "accessed field"
}

// Test 3: Direct call to non-nil function - should NOT be detected
func testDirectNonNilCall() {
	cfg := nonNilFunc()
	_ = cfg.Value // OK - nonNilFunc always returns non-nil
}

// Test 4: Call through function variable pointing to non-nil function - should NOT be detected
func testFuncVarNonNilCall() {
	cfg := NonNilFuncVar()
	_ = cfg.Value // OK - NonNilFuncVar points to nonNilFunc which always returns non-nil
}

// Test 5: Local function variable assigned from nilable function
func testLocalFuncVarNilable() {
	f := nilableFunc
	cfg := f()
	_ = cfg.Value //want "accessed field"
}

// Test 6: Local function variable assigned from non-nil function
func testLocalFuncVarNonNil() {
	f := nonNilFunc
	cfg := f()
	_ = cfg.Value // OK - f points to nonNilFunc
}

// Test 7: Function that takes a pointer and dereferences it
func process(cfg *Config) {
	// The nil flow is detected at the call site, not here
	println(cfg.Value)
}

// Test 8: Package-level function variable for process
var ProcessFunc = process

// Test 9: Calling process through function variable with nil - should be detected
func testProcessFuncVarNil() {
	ProcessFunc(nil) //want "passed"
}

// Test 10: Calling process directly with nil - should be detected
func testProcessDirectNil() {
	process(nil) //want "passed"
}

// Test 11: Calling process with non-nil - should NOT be detected
func testProcessNonNil() {
	cfg := &Config{Value: "test"}
	process(cfg)     // OK
	ProcessFunc(cfg) // OK
}
