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

// Package app checks return projections and recursive return shapes from package lib.
package app

import "go.uber.org/structinitv2/returnprojection/lib"

// Field-projection return `return x.In`; x.In.Child is nil.
func useReturnProjection() {
	a := lib.ReturnProjection()
	print(a.Child.Ptr) //want "field `Child` of result 0 of `ReturnProjection`"
}

// ReturnProjectionSafe initializes Child; no flag.
func useReturnProjectionSafe() {
	a := lib.ReturnProjectionSafe()
	print(a.Child.Ptr)
}

// The recursive constructor's top-level Ptr is tracked.
func useRecTop() {
	a := lib.MkRec()
	print(*a.Ptr) //want "field `Ptr` of result 0 of `MkRec`"
}

// One unrolling of the recursive type is supported, and Self.Ptr is genuinely nil, so this flags.
// Paths beyond the first unrolling (Self.Self.Ptr) stay under-reports.
func useRecDeep() {
	a := lib.MkRec()
	print(*a.Self.Ptr) //want "field `Self.Ptr` of result 0 of `MkRec`"
}
