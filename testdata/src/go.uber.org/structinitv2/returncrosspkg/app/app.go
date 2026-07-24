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

// Package app is the consumer side of the cross-package return-shape tests: the deep dereferences
// land here, so the diagnostics are asserted with //want in this package. See package lib.
package app

import (
	"go.uber.org/structinitv2/returncrosspkg/lib"
	"go.uber.org/structinitv2/returncrosspkg/mid"
)

// ReturnDeepNil leaves Mid.Child nil; the deep deref must flag.
func useReturnDeepNil() {
	a := lib.ReturnDeepNil()
	print(a.Mid.Child.Ptr) //want "field `Mid.Child` of result 0 of `ReturnDeepNil`"
}

// ReturnDeepNonNil initializes Mid.Child; the same deref must NOT flag.
func useReturnDeepNonNil() {
	a := lib.ReturnDeepNonNil()
	print(a.Mid.Child.Ptr)
}

// Transitive across two package hops (app -> mid.ForwardImportedResult -> lib.ReturnDeepNil).
func useForwardImportedResult() {
	a := mid.ForwardImportedResult()
	print(a.Mid.Child.Ptr) //want "field `Mid.Child` of result 0 of `ForwardImportedResult`"
}

// `return g()` is as deep as g's own return shape.
func useReturnForwardedConstructor() {
	a := lib.ReturnForwardedConstructor()
	print(a.Mid.Child.Ptr) //want "field `Mid.Child` of result 0 of `ReturnForwardedConstructor`"
}
