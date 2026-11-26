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

package testmain

import (
	"os"
	"testing"
)

var _testMainInit *int
var _testMainInitHelper *int
var _testMainUnset *int

func TestMain(m *testing.M) {
	_testMainInit = new(int)
	testMainSetup()
	os.Exit(m.Run())
}

func testMainSetup() {
	_testMainInitHelper = new(int)
}

func TestUseTestMainGlobals(t *testing.T) {
	print(*_testMainInit)
	print(*_testMainInitHelper)
	print(*_testMainUnset) // want "dereferenced"
}
