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

// A parameter field read as a value in another package: shared.Get returns a.Aptr without
// dereferencing it, so the imported read summary must carry the value demand for `Aptr` for the
// caller-side dereference of the escaped value to be flagged.

package app

import "structinitv2/crosspkg/shared"

func unsafeFieldValueEscape() {
	a := &shared.A{}
	// The imported source resolves the result from this call's argument.
	usePtr(shared.Get(a).Ptr) //want "result 0 of `Get`"
}

func safeFieldValueEscape() {
	a := &shared.A{Aptr: &shared.Leaf{}}
	usePtr(shared.Get(a).Ptr)
}
