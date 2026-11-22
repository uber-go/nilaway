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

// Package middle exposes each upstream type in a different way, and says nothing itself about the
// nilability of their methods. Their facts reach downstream only if this package forwards them.
package middle

import "go.uber.org/transitivefacts/surface/upstream"

// Alias exposes upstream.Aliased through a type alias.
type Alias = upstream.Aliased

// MakeOuter exposes upstream.Inner only through the methods promoted to upstream.Outer.
func MakeOuter() upstream.Outer {
	return upstream.Outer{}
}

// Global exposes upstream.Global through an exported package-level variable.
var Global upstream.Global

// Chan exposes upstream.ChanElem as a channel element type.
func Chan() chan upstream.ChanElem {
	ch := make(chan upstream.ChanElem, 1)
	ch <- upstream.ChanElem{}
	return ch
}

// Map exposes upstream.MapElem as a map value type.
func Map() map[string]upstream.MapElem {
	return map[string]upstream.MapElem{"key": {}}
}

// holder is unexported, but its exported method is callable on values obtained from MakeHolder.
type holder struct{}

// Held exposes upstream.Held as the result of an exported method of an unexported type.
func (holder) Held() upstream.Held {
	return upstream.Held{}
}

// MakeHolder returns a value of an unexported type, whose exported methods remain callable.
func MakeHolder() holder {
	return holder{}
}
