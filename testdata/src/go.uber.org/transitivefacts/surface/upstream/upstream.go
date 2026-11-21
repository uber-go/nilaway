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

// Package upstream is the root of a chain in which downstream reaches upstream types only through
// middle. Each type below has a method that returns nil, so only this package's fact knows its
// result is nilable. Middle exposes each type in a different way (see
// primitivizer.upstreamAPISurface).
package upstream

// Aliased is exposed by middle through a type alias.
type Aliased struct{}

// AliasedNil always returns nil.
func (Aliased) AliasedNil() *int { return nil }

// Inner is embedded in Outer, so its methods are promoted to Outer.
type Inner struct{}

// PromotedNil always returns nil.
func (Inner) PromotedNil() *int { return nil }

// Outer is exposed by middle as a function result, making Inner reachable only through promotion.
type Outer struct{ Inner }

// Global is exposed by middle through an exported package-level variable.
type Global struct{}

// GlobalNil always returns nil.
func (Global) GlobalNil() *int { return nil }

// ChanElem is exposed by middle as the element type of a channel.
type ChanElem struct{}

// ChanElemNil always returns nil.
func (ChanElem) ChanElemNil() *int { return nil }

// MapElem is exposed by middle as the value type of a map.
type MapElem struct{}

// MapElemNil always returns nil.
func (MapElem) MapElemNil() *int { return nil }

// Held is exposed by middle through an exported method of an unexported type.
type Held struct{}

// HeldNil always returns nil.
func (Held) HeldNil() *int { return nil }
