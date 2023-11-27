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

package util

import (
	"go/ast"
)

// A GuardNonce is a unique token used to identify contracts that arise through the RichCheckEffect
// mechanism. GuardNonces are canonically tied to an AST node through the ExprNonceMap accumulated
// in their generator.
type GuardNonce int

// An ExprNonceMap maps AST nodes to nonces, to establish their canonical interpretation.
type ExprNonceMap = map[ast.Expr]GuardNonce

// A GuardNonceGenerator is a stateful object used to ensure unique obtainment of nonces. It also keeps
// track of the expression with which they are associate, in a map of type ExprNonceMap to be later
// embedded into a RootAssertionNode
// nonnil(exprNonceMap)
type GuardNonceGenerator struct {
	last         GuardNonce
	exprNonceMap ExprNonceMap
}

// NewGuardNonceGenerator returns a fresh instance of GuardNonceGenerator that can be subsequently used
// to identify and track RichCheckEffects.
func NewGuardNonceGenerator() *GuardNonceGenerator {
	return &GuardNonceGenerator{
		last:         -1,
		exprNonceMap: make(ExprNonceMap),
	}
}

// Next for a GuardNonceGenerator returns the first nonce that has not already been output. The
// method also takes an AST expression, to be tied to this nonce as its interpretation.
func (g *GuardNonceGenerator) Next(expr ast.Expr) GuardNonce {
	next := g.last + 1
	g.last = next

	g.exprNonceMap[expr] = next

	return next
}

// GetExprNonceMap for a GuardNonceGenerator returns the underlying ExprNonceMap
func (g *GuardNonceGenerator) GetExprNonceMap() ExprNonceMap {
	return g.exprNonceMap
}

// Eq compares two GuardNonces for equality
func (g GuardNonce) Eq(g2 GuardNonce) bool {
	return g == g2
}

// GuardNonceSet is a set of GuardNonces
type GuardNonceSet map[GuardNonce]bool

// IsEmpty returns true if a given GuardNonceSet is empty
func (g GuardNonceSet) IsEmpty() bool {
	return len(g) == 0
}

// Add statefully adds one or more new GuardNonces to a GuardNonceSet
// nonnil(result 0)
func (g GuardNonceSet) Add(guards ...GuardNonce) GuardNonceSet {
	for _, guard := range guards {
		g[guard] = true
	}
	return g
}

// Remove statefully removes one or more GuardNonces from a GuardNonceSet
// nonnil(result 0)
func (g GuardNonceSet) Remove(guards ...GuardNonce) GuardNonceSet {
	for _, guard := range guards {
		delete(g, guard)
	}
	return g
}

// Contains returns true iff a given GuardNonceSet contains a passed GuardNonce
func (g GuardNonceSet) Contains(n GuardNonce) bool {
	return g[n]
}

// SubsetOf returns true iff a given GuardNonceSet is a subset of a passed GuardNonceSet
// nonnil(other)
func (g GuardNonceSet) SubsetOf(other GuardNonceSet) bool {
	for guard := range g {
		if !other.Contains(guard) {
			return false
		}
	}
	return true
}

// Union returns a new GuardNonceSet that is the union of its two parameters without modifying either
// nonnil(result 0)
func (g GuardNonceSet) Union(others ...GuardNonceSet) GuardNonceSet {
	out := make(GuardNonceSet)
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

// Intersection returns a new GuardNonceSet that is the intersection of its two parameters without modifying either
// nonnil(others)
func (g GuardNonceSet) Intersection(others ...GuardNonceSet) GuardNonceSet {
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

// Eq returns true iff a given GuardNonceSet contains the same elements as a passed GuardNonceSet
// nonnil(other)
func (g GuardNonceSet) Eq(other GuardNonceSet) bool {
	return g.SubsetOf(other) && other.SubsetOf(g)
}

// Copy returns a copy of the passed GuardNonceSet without modifying the original
// nonnil(result 0)
func (g GuardNonceSet) Copy() GuardNonceSet {
	return g.Union(nil)
}

// NoGuards returns an empty GuardNonceSet - to be used to indicate no guards
func NoGuards() GuardNonceSet {
	return make(GuardNonceSet)
}
