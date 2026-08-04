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

package annotation

import (
	"cmp"
	"strings"
)

// FieldPath is an opaque, comparable chain of struct field selections below a boundary value (a
// parameter, receiver, or result). The zero value denotes the boundary value itself.
type FieldPath struct {
	// dotted is the field-name segments joined with ".". Field names are identifiers and cannot
	// contain the separator, and a single string keeps FieldPath usable as a map key.
	dotted string
}

// NewFieldPath builds the path selecting segments in order below a boundary value.
func NewFieldPath(segments ...string) FieldPath {
	return FieldPath{dotted: strings.Join(segments, ".")}
}

// IsRoot reports whether p is the boundary value itself.
func (p FieldPath) IsRoot() bool {
	return p.dotted == ""
}

// Child returns the path selecting field below p.
func (p FieldPath) Child(field string) FieldPath {
	if p.dotted == "" {
		return FieldPath{dotted: field}
	}
	return FieldPath{dotted: p.dotted + "." + field}
}

// Join concatenates two paths: the result selects p's segments, then sub's.
func (p FieldPath) Join(sub FieldPath) FieldPath {
	switch {
	case p.dotted == "":
		return sub
	case sub.dotted == "":
		return p
	}
	return FieldPath{dotted: p.dotted + "." + sub.dotted}
}

// TrimPrefix is Join's inverse: prefix.Join(sub) == p iff p.TrimPrefix(prefix) == (sub, true).
// The match is per segment (`Middle` is not below `Mid`); ok is false when prefix does not cover p.
func (p FieldPath) TrimPrefix(prefix FieldPath) (sub FieldPath, ok bool) {
	switch {
	case prefix.dotted == "":
		return p, true
	case p.dotted == prefix.dotted:
		return FieldPath{}, true
	case strings.HasPrefix(p.dotted, prefix.dotted+"."):
		return FieldPath{dotted: p.dotted[len(prefix.dotted)+1:]}, true
	default:
		return FieldPath{}, false
	}
}

// Segments returns p's field names in selection order; nil for the root path.
func (p FieldPath) Segments() []string {
	if p.dotted == "" {
		return nil
	}
	return strings.Split(p.dotted, ".")
}

// NumSegments returns the number of field selections in p.
func (p FieldPath) NumSegments() int {
	if p.dotted == "" {
		return 0
	}
	return strings.Count(p.dotted, ".") + 1
}

// Compare lexicographically orders paths for deterministic output.
func (p FieldPath) Compare(other FieldPath) int {
	return cmp.Compare(p.dotted, other.dotted)
}

// String renders p for diagnostics, e.g. "Mid.Child"; the root path renders empty.
func (p FieldPath) String() string {
	return p.dotted
}

// GobEncode implements gob.GobEncoder: FieldPath appears in exported analysis facts, and gob
// cannot see the unexported representation.
func (p FieldPath) GobEncode() ([]byte, error) {
	return []byte(p.dotted), nil
}

// GobDecode implements gob.GobDecoder.
func (p *FieldPath) GobDecode(data []byte) error {
	p.dotted = string(data)
	return nil
}
