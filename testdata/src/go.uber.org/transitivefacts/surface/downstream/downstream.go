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

// Package downstream imports middle but not upstream. Each dereference below reads a result that
// upstream determined nilable, so it is reported only if middle forwards upstream's fact.
package downstream

import "go.uber.org/transitivefacts/surface/middle"

func useAlias() {
	var aliased middle.Alias
	print(*aliased.AliasedNil()) //want "result 0 of `AliasedNil\\(\\)` dereferenced"
}

func usePromoted() {
	print(*middle.MakeOuter().PromotedNil()) //want "result 0 of `PromotedNil\\(\\)` dereferenced"
}

func useGlobal() {
	print(*middle.Global.GlobalNil()) //want "result 0 of `GlobalNil\\(\\)` dereferenced"
}

func useChan() {
	print(*(<-middle.Chan()).ChanElemNil()) //want "result 0 of `ChanElemNil\\(\\)` dereferenced"
}

func useMap() {
	print(*middle.Map()["key"].MapElemNil()) //want "result 0 of `MapElemNil\\(\\)` dereferenced"
}

func useUnexportedHolder() {
	print(*middle.MakeHolder().Held().HeldNil()) //want "result 0 of `HeldNil\\(\\)` dereferenced"
}
