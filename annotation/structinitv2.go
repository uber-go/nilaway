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

package annotation

import (
	"fmt"
	"go/types"
)

// This file contains annotation triggers for the allocation-site-sensitive struct
// initialization analysis (v2).

// StructFieldNil is a v2 producer for a nil field in a specific struct allocation. It always
// produces nil. Because it is attached to the field node for that allocation, different allocations
// of the same struct type keep separate producers.
type StructFieldNil struct {
	*ProduceTriggerTautology

	// FieldName is the name of the field that is nil, used only for diagnostics.
	FieldName string
}

// equals returns true if the passed ProducingAnnotationTrigger is equal to this one.
func (s *StructFieldNil) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*StructFieldNil); ok {
		return s.FieldName == other.FieldName &&
			s.ProduceTriggerTautology.equals(other.ProduceTriggerTautology)
	}
	return false
}

// Prestring returns this StructFieldNil as a Prestring.
func (s *StructFieldNil) Prestring() Prestring {
	return StructFieldNilPrestring{FieldName: s.FieldName}
}

// StructFieldNilPrestring is a Prestring storing the information needed to compactly encode a
// StructFieldNil.
type StructFieldNilPrestring struct {
	FieldName string
}

func (s StructFieldNilPrestring) String() string {
	return fmt.Sprintf("uninitialized field `%s`", s.FieldName)
}

// Field-path context sites carry struct field nilability across function boundaries.

// StructFieldContextKind distinguishes whether a context site summarizes a field of a function
// return value or of a function parameter/receiver.
type StructFieldContextKind uint8

const (
	// StructFieldReturnContext is the nilability of a field of a function's return value.
	StructFieldReturnContext StructFieldContextKind = iota
	// StructFieldParamContext is the nilability of a field of a function's parameter/receiver,
	// as observed on entry to the function (the value passed in by the caller).
	StructFieldParamContext
	// StructFieldParamOutContext is the nilability of a field of a function's parameter/receiver
	// as it stands when the function returns (i.e. after any field writes the function performs).
	// It is the caller-visible side effect of the call on the argument's fields.
	StructFieldParamOutContext
)

func (k StructFieldContextKind) string() string {
	switch k {
	case StructFieldReturnContext:
		return "result"
	default:
		// Both param-in and param-out read as "param" / "argument" in diagnostics; the in/out
		// distinction is internal to inference and not surfaced to users.
		return "param"
	}
}

// boundaryDesc renders the boundary descriptor used in v2 diagnostics, e.g. "param 0 of `f`",
// "result 0 of `g`", or "method receiver of `m`" (when the param index is the receiver index).
func boundaryDesc(kind string, index int, funcName string) string {
	if kind == "param" && index == ReceiverParamIndex {
		return fmt.Sprintf("method receiver of `%s`", funcName)
	}
	return fmt.Sprintf("%s %d of `%s`", kind, index, funcName)
}

// StructFieldContextSite is a v2 annotation site (annotation.Key) representing the nilability
// of a nested field, identified by Path, of the Index-th return value or parameter of FuncObj.
// It is inference-only.
type StructFieldContextSite struct {
	FuncObj *types.Func
	Kind    StructFieldContextKind
	Index   int
	// Path is the dotted field path from the boundary value to the tracked field (e.g. "aptr").
	Path string
}

// Lookup always returns the non-annotated default.
// Syntactic annotation is not yet supported; their nilability comes purely from inference.
func (s *StructFieldContextSite) Lookup(Map) (Val, bool) {
	return nonAnnotatedDefault, false
}

// Object returns the function this site belongs to. Cross-package inference identity uses Object()
// together with String(); the function is unique across packages, while the field may come from a
// shared struct type.
func (s *StructFieldContextSite) Object() types.Object { return s.FuncObj }

func (s *StructFieldContextSite) equals(other Key) bool {
	if other, ok := other.(*StructFieldContextSite); ok {
		return *s == *other
	}
	return false
}

func (s *StructFieldContextSite) copy() Key {
	c := *s
	return &c
}

func (s *StructFieldContextSite) String() string {
	// The Kind value is included verbatim so that param-in and param-out sites (which render
	// identically in user-facing diagnostics) remain distinct inference sites: the inference
	// engine identifies sites by Position + this string. Without it the two would collide.
	return fmt.Sprintf("field `%s` of kind %d %s", s.Path, s.Kind, boundaryDesc(s.Kind.string(), s.Index, s.FuncObj.Name()))
}

// StructFieldFromContext is a v2 producer: the value of a field (read from a boundary, e.g.
// `b.f` where `b := give()`) is nil iff the corresponding context site is inferred nilable.
type StructFieldFromContext struct {
	*TriggerIfNilable
}

func (s *StructFieldFromContext) equals(other ProducingAnnotationTrigger) bool {
	if other, ok := other.(*StructFieldFromContext); ok {
		return s.TriggerIfNilable.equals(other.TriggerIfNilable)
	}
	return false
}

// Prestring returns this StructFieldFromContext as a Prestring.
func (s *StructFieldFromContext) Prestring() Prestring {
	site := s.Ann.(*StructFieldContextSite)
	return StructFieldFromContextPrestring{Path: site.Path, Kind: site.Kind.string(), Index: site.Index, FuncName: site.FuncObj.Name()}
}

// StructFieldFromContextPrestring is the compact encoding of a StructFieldFromContext.
type StructFieldFromContextPrestring struct {
	Path     string
	Kind     string
	Index    int
	FuncName string
}

func (s StructFieldFromContextPrestring) String() string {
	return fmt.Sprintf("field `%s` of %s", s.Path, boundaryDesc(s.Kind, s.Index, s.FuncName))
}

// StructFieldToContext is a v2 consumer: a value flows into a field of a boundary (e.g. a
// returned struct's field). It requires the context site to be nonnil; when a definitely-nil
// producer reaches it, inference marks the site nilable.
type StructFieldToContext struct {
	*TriggerIfNonNil
}

func (s *StructFieldToContext) equals(other ConsumingAnnotationTrigger) bool {
	if other, ok := other.(*StructFieldToContext); ok {
		return s.TriggerIfNonNil.equals(other.TriggerIfNonNil)
	}
	return false
}

// Copy returns a deep copy of this consumer.
func (s *StructFieldToContext) Copy() ConsumingAnnotationTrigger {
	c := *s
	c.TriggerIfNonNil = s.TriggerIfNonNil.Copy().(*TriggerIfNonNil)
	return &c
}

// Prestring returns this StructFieldToContext as a Prestring.
func (s *StructFieldToContext) Prestring() Prestring {
	site := s.Ann.(*StructFieldContextSite)
	return StructFieldToContextPrestring{Path: site.Path, Kind: site.Kind.string(), Index: site.Index, FuncName: site.FuncObj.Name()}
}

// StructFieldToContextPrestring is the compact encoding of a StructFieldToContext.
type StructFieldToContextPrestring struct {
	Path     string
	Kind     string
	Index    int
	FuncName string
}

func (s StructFieldToContextPrestring) String() string {
	return fmt.Sprintf("field `%s` reaches %s", s.Path, boundaryDesc(s.Kind, s.Index, s.FuncName))
}
