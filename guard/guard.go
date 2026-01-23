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

// Package guard hosts the guard nonce types and functions to identify and track RichCheckEffects.
package guard

import (
	"go/ast"
)

// A Nonce is a unique token used to identify contracts that arise through the RichCheckEffect
// mechanism. Nonces are canonically tied to an AST node through the ExprNonceMap accumulated
// in their generator.
type Nonce int

// An ExprNonceMap maps AST nodes to nonces, to establish their canonical interpretation.
type ExprNonceMap = map[ast.Expr]Nonce

// A NonceGenerator is a stateful object used to ensure unique obtainment of nonces. It also keeps
// track of the expression with which they are associate, in a map of type ExprNonceMap to be later
// embedded into a RootAssertionNode
// nonnil(exprNonceMap)
type NonceGenerator struct {
	last         Nonce
	exprNonceMap ExprNonceMap
}

// NewNonceGenerator returns a fresh instance of NonceGenerator that can be subsequently used
// to identify and track RichCheckEffects.
func NewNonceGenerator() *NonceGenerator {
	return &NonceGenerator{
		last:         -1,
		exprNonceMap: make(ExprNonceMap),
	}
}

// Next for a NonceGenerator returns the first nonce that has not already been output. The
// method also takes an AST expression, to be tied to this nonce as its interpretation.
func (g *NonceGenerator) Next(expr ast.Expr) Nonce {
	next := g.last + 1
	g.last = next

	g.exprNonceMap[expr] = next

	return next
}

// GetExprNonceMap for a NonceGenerator returns the underlying ExprNonceMap
func (g *NonceGenerator) GetExprNonceMap() ExprNonceMap {
	return g.exprNonceMap
}

// Eq compares two Nonces for equality
func (g Nonce) Eq(g2 Nonce) bool {
	return g == g2
}

// NonceSet is a set of Nonces
type NonceSet map[Nonce]bool

// IsEmpty returns true if a given NonceSet is empty
func (g NonceSet) IsEmpty() bool {
	return len(g) == 0
}

// Add statefully adds one or more new Nonces to a NonceSet
// nonnil(result 0)
func (g NonceSet) Add(guards ...Nonce) NonceSet {
	for _, guard := range guards {
		g[guard] = true
	}
	return g
}

// Remove statefully removes one or more Nonces from a NonceSet
// nonnil(result 0)
func (g NonceSet) Remove(guards ...Nonce) NonceSet {
	for _, guard := range guards {
		delete(g, guard)
	}
	return g
}

// Contains returns true iff a given NonceSet contains a passed Nonce
func (g NonceSet) Contains(n Nonce) bool {
	return g[n]
}

// SubsetOf returns true iff a given NonceSet is a subset of a passed NonceSet
// nonnil(other)
func (g NonceSet) SubsetOf(other NonceSet) bool {
	for guard := range g {
		if !other.Contains(guard) {
			return false
		}
	}
	return true
}

// Union returns a new NonceSet that is the union of its two parameters without modifying either
// nonnil(result 0)
func (g NonceSet) Union(others ...NonceSet) NonceSet {
	out := make(NonceSet)
	for guard := range g {
		out.Add(guard)
	}
	for _, other := range others {
		for guard := range other {
			out.Add(guard)
		}
	}
	return out
}

// Intersection returns a new NonceSet that is the intersection of its two parameters without modifying either
// nonnil(others)
func (g NonceSet) Intersection(others ...NonceSet) NonceSet {
	out := g.Union(others...)
checkingOut:
	for guard := range out {
		if !g.Contains(guard) {
			out.Remove(guard)
			continue checkingOut
		}
		for _, other := range others {
			if !other.Contains(guard) {
				out.Remove(guard)
				continue checkingOut
			}
		}
	}
	return out
}

// Eq returns true iff a given NonceSet contains the same elements as a passed NonceSet
// nonnil(other)
func (g NonceSet) Eq(other NonceSet) bool {
	return g.SubsetOf(other) && other.SubsetOf(g)
}

// Copy returns a copy of the passed NonceSet without modifying the original
// nonnil(result 0)
func (g NonceSet) Copy() NonceSet {
	return g.Union(nil)
}

// NoGuards returns an empty NonceSet - to be used to indicate no guards
func NoGuards() NonceSet {
	return make(NonceSet)
}