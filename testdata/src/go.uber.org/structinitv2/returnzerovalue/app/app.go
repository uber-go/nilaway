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

// Package app is the consumer side of the zero-value return tests.
package app

import "go.uber.org/structinitv2/returnzerovalue/lib"

// Zero-value return `var x Outer; return x`; Mid is nil.
func useReturnZeroValue() {
	a := lib.ReturnZeroValue()
	print(a.Mid.Child) //want "field `Mid` of result 0 of `ReturnZeroValue`"
}

// Naked named return is a documented under-report — NOT flagged.
func useNaked() {
	a := lib.NakedRet()
	print(a.Mid.Child)
}
