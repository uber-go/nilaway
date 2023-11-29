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

// Package orderedmap implements a generic ordered map that supports iteration in insertion order.
package orderedmap

// Pair is a key-value pair stored in the ordered map.
type Pair[K comparable, V any] struct {
	Key   K
	Value V
}

// Note the design of OrderedMap is a little different from usual ordered map implementations. It
// might be more intuitive if we keep an inner map and another slice of keys in insertion order.
// It might even be better if we fully encapsulate (un-export) the fields to avoid leaky
// abstraction. However, both of these approaches require we define custom gob codec logic (i.e.,
// we define our own `GobEncode` and `GobDecode` methods for OrderedMap).
// Overriding the codec disallows re-uses of the same gob Encoder (stdlib does not pass the parent
// encoder via the `GobEncode` interface), which will eventually lead to worse performance, as well
// as larger binary size (a type will need to be encoded multiple times instead of just once if
// the encoder is shared). See [gob encoding] for more details.
// [gob encoding]: https://pkg.go.dev/encoding/gob

// OrderedMap is an ordered map that supports iteration in insertion order. It is an _internal_
// helper struct for NilAway and lacks some of the features of a full map.
type OrderedMap[K comparable, V any] struct {
	// Pairs is the list of pairs in insertion order. It should _never_ be modified directly (use
	// Store instead), but can be used for read-only purposes (e.g., iterations). It is
	// specifically exported to allow serialization via gob encoding.
	Pairs []*Pair[K, V]
	// inner keeps the mapping between key and the pointer to a particular pair. It is specifically
	// unexported to avoid serialization via gob encoding.
	inner map[K]*Pair[K, V]
}

// New creates a new OrderedMap.
func New[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{inner: make(map[K]*Pair[K, V])}
}

// Value returns the value stored in the map for the key, or the zero value if the key is not found.
// It is the same as Load, but without the additional bool.
func (m *OrderedMap[K, V]) Value(key K) V {
	m.rehydrate()

	if p := m.inner[key]; p != nil {
		return p.Value
	}
	var v V
	return v
}

// Load returns the value stored in the map for the key, with an additional bool indicating if
// the key was found.
func (m *OrderedMap[K, V]) Load(key K) (V, bool) {
	m.rehydrate()

	if p := m.inner[key]; p != nil {
		return p.Value, true
	}
	var v V
	return v, false
}

// Store stores the value in the map for the key, overwriting the previous value if the key exists.
func (m *OrderedMap[K, V]) Store(key K, value V) {
	m.rehydrate()

	if p := m.inner[key]; p != nil {
		p.Value = value
		return
	}
	p := &Pair[K, V]{Key: key, Value: value}
	m.Pairs = append(m.Pairs, p)
	m.inner[key] = p
}

// rehydrate ensures that the inner map is up-to-date with the Pairs slice. This can happen when
// the OrderedMap is serialized and deserialized via gob encoding (the inner map is unexported and
// hence ignored from serialization). rehydrate must be called before accessing the inner map
// after deserialization.
func (m *OrderedMap[K, V]) rehydrate() {
	if len(m.Pairs) == len(m.inner) {
		return
	}

	m.inner = make(map[K]*Pair[K, V], len(m.Pairs))
	for _, p := range m.Pairs {
		m.inner[p.Key] = p
	}
}
