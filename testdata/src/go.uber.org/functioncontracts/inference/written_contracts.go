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

package inference

// This file tests if NilAway's capability to respect the written contracts in full inference mode.

import "math/rand"

// We set an incorrect manual contract here, but NilAway should still respect it.
// TODO: NilAway should validate and warn users of incorrect manual contract annotations.
// contract(nonnil -> nonnil)
func incorrectContract(x *int) *int {
	if x != nil {
		// Returns nil if the input is nonnil, violating the contract.
		return nil
	}
	// Return nonnil or nil randomly
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		return nil
	}
}

func useIncorrectContract1() {
	n := 1
	a1 := &n
	b1 := incorrectContract(a1)
	print(*b1) // No error here due to the contract.
}

func useIncorrectContract2() {
	var a2 *int
	b2 := incorrectContract(a2)
	// TODO: NilAway is reporting the same error below twice. This will be grouped into one error
	//  in real world due to our diagnostic engine. However, this is still a bug in contract
	//  handling. We should investigate and fix this.
	print(*b2) // want "result 0 of `incorrectContract.*` .* dereferenced" "result 0 of `incorrectContract.*` .* dereferenced"
}

// Contract below isn't useful, since return is always nonnil and argument is ignored, but added to
// check we don't crash on unnamed parameters.
// contract(nonnil -> nonnil)
func fooUnnamedParam(_ *int) *int {
	return new(int)
}

func barUnnamedParam1() {
	var a1 *int
	b1 := fooUnnamedParam(a1)
	print(*b1) // No error here.
}

func barUnnamedParam2() {
	var a2 *int
	b2 := fooUnnamedParam(a2)
	print(*b2) // No error here.
}
