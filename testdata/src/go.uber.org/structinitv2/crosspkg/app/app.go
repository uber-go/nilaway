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

// Package app checks that two sibling packages with same-named constructors returning the same
// shared type keep distinct cross-package inference sites: p1.New always sets Aptr while p2.New
// leaves it nil, so only the deref of p2.New().Aptr must be flagged.
package app

import (
	"go.uber.org/structinitv2/crosspkg/p1"
	"go.uber.org/structinitv2/crosspkg/p2"
)

func usePtr(p *int) { print(p) }

// safeUsesP1 dereferences a field that p1.New always initializes: no error.
func safeUsesP1() {
	a := p1.New()
	usePtr(a.Aptr.Ptr)
}

// unsafeUsesP2 dereferences a field that p2.New leaves nil: error.
func unsafeUsesP2() {
	a := p2.New()
	usePtr(a.Aptr.Ptr) //want "uninitialized field `Aptr`"
}
