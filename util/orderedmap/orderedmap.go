package orderedmap

import (
	"bytes"
	"encoding/gob"
	"io"
)

type OrderedMap[K comparable, V any] struct {
	inner map[K]V
	keys  []K
}

func New[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{inner: make(map[K]V)}
}

func (m *OrderedMap[K, V]) Load(key K) (V, bool) {
	v, ok := m.inner[key]
	return v, ok
}

func (m *OrderedMap[K, V]) Store(key K, value V) {
	if _, ok := m.inner[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.inner[key] = value
}

func (m *OrderedMap[K, V]) OrderedRange(f func(key K, value V) bool) {
	for _, k := range m.keys {
		if !f(k, m.inner[k]) {
			return
		}
	}
}

func (m *OrderedMap[K, V]) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, k := range m.keys {
		if err := enc.Encode(k); err != nil {
			return nil, err
		}
		if err := enc.Encode(m.inner[k]); err != nil {
			return nil, err
		}
	}

	if buf.Len() == 0 {
		return nil, nil
	}
	return buf.Bytes(), nil
}

func (m *OrderedMap[K, V]) GobDecode(b []byte) error {
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	for {
		var k K
		if err := dec.Decode(&k); err == io.EOF {
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
