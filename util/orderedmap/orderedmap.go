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

import (
	"bytes"
	"encoding/gob"
	"io"
)

// OrderedMap is an ordered map that supports iteration in insertion order.
type OrderedMap[K comparable, V any] struct {
	inner map[K]V
	keys  []K
}

// New creates a new OrderedMap.
func New[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{inner: make(map[K]V)}
}

// Value returns the value stored in the map for the key, or the zero value if the key is not found.
// It is the same as Load, but without the additional bool.
func (m *OrderedMap[K, V]) Value(key K) V {
	return m.inner[key]
}

// Load returns the value stored in the map for the key, with an additional bool indicating if
// the key was found.
func (m *OrderedMap[K, V]) Load(key K) (V, bool) {
	v, ok := m.inner[key]
	return v, ok
}

// Store stores the value in the map for the key, overwriting the previous value if the key exists.
func (m *OrderedMap[K, V]) Store(key K, value V) {
	if _, ok := m.inner[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.inner[key] = value
}

// Len returns the length of the map.
func (m *OrderedMap[K, V]) Len() int {
	return len(m.inner)
}

// OrderedRange iterates over the map in insertion order, calling the passed function for each
// key/value pair.
func (m *OrderedMap[K, V]) OrderedRange(f func(key K, value V) bool) {
	for _, k := range m.keys {
		if !f(k, m.inner[k]) {
			return
		}
	}
}

// GobEncode encodes the map using gob encoding, it encodes each pair in insertion order.
func (m *OrderedMap[K, V]) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	// For each pair, we encode the key and then the value.
	for _, k := range m.keys {
		if err := enc.Encode(&k); err != nil {
			return nil, err
		}
		v := m.inner[k]
		if err := enc.Encode(&v); err != nil {
			return nil, err
		}
	}

	if buf.Len() == 0 {
		return nil, nil
	}
	return buf.Bytes(), nil
}

// GobDecode decodes the map using gob encoding.
func (m *OrderedMap[K, V]) GobDecode(b []byte) error {
	m.inner = make(map[K]V)
	m.keys = nil

	dec := gob.NewDecoder(bytes.NewBuffer(b))
	for {
		var k K
		if err := dec.Decode(&k); err == io.EOF {
			// The map is encoded as a stream of key/value pairs. So if we ever hit EOF when decoding
			// K, we know we're done.
			break
		} else if err != nil {
			return err
		}

		var v V
		if err := dec.Decode(&v); err != nil {
			return err
		}

		m.inner[k] = v
		m.keys = append(m.keys, k)
	}

	return nil
}
