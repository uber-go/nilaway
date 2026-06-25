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

// Package app checks cross-package parameter side effects: a function in package writer nils a
// field of its argument, and the caller must observe the post-call nil at the dereference.
package app

import "go.uber.org/structinitv2/crosspkgside/writer"

// tCrossPkgSideEffect passes a value with an initialized Child to a cross-package function that
// nils it; the post-call deref of the now-nil Child is reported.
func tCrossPkgSideEffect() *int {
	b := &writer.Node{Child: &writer.Leaf{}}
	writer.WriteNil(b) // sets b.Child = nil across the package boundary
	return b.Child.Ptr //want "field `Child` of param 0 of `WriteNil` accessed field `Ptr`"
}

// tCrossPkgDeepSideEffect passes a value with an initialized Mid.Child to a cross-package function
// that nils the nested field; the post-call deref of the now-nil Mid.Child is reported.
func tCrossPkgDeepSideEffect() *int {
	b := &writer.Outer{Mid: &writer.Node{Child: &writer.Leaf{}}}
	writer.WriteDeepNil(b) // sets b.Mid.Child = nil across the package boundary
	return b.Mid.Child.Ptr //want "field `Mid.Child` of param 0 of `WriteDeepNil` accessed field `Ptr`"
}
