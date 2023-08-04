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

package assertiontree

import (
	"go/ast"

	"go.uber.org/nilaway/annotation"
	"golang.org/x/tools/go/analysis"
)

// An AssertionNode is the root of a tree of assertions, so it contains parent and child pointers, as well as a set
// of "ConsumeTriggers" - these are half assertions representing a point at which nil may be erroneously consumed and
// which annotation should be checked to see if that consumption is in fact erroneous. Their position in the tree gives
// the expression that they are asserting should possibly be non-nil
// TODO: make more efficient by having children and triggers be a keyed map instead of a slice
type AssertionNode interface {
	Parent() AssertionNode
	Children() []AssertionNode
	ConsumeTriggers() []*annotation.ConsumeTrigger

	SetParent(AssertionNode)
	SetChildren([]AssertionNode)
	SetConsumeTriggers([]*annotation.ConsumeTrigger)

	// DefaultTrigger determines the ProducingAnnotationTrigger that produces this
	// value as a last resort - called when a tracked value is determined to only be
	// producible only by read of its default. An example case of calling this method
	// is that a lingering ConsumeTrigger resides at x.f in the tree when x is assigned
	// into by a non-trackable expression. Then that ConsumeTrigger will be matched with
	// the result of this method as a ProduceTrigger, which in particular will be an
	// annotation.FldRead. See implementations for full range of cases.
	// It is called from two places. First, at the process entry it is used to match all
	// the unmatched consumers. Second, is for consuming all the nodes in subtree of the
	// node if the node gets matched.
	DefaultTrigger() annotation.ProducingAnnotationTrigger

	// BuildExpr takes an expression, and builds a new one by wrapping it in a new AST expression
	// corresponding to this node
	// nilable(param 1)
	BuildExpr(*analysis.Pass, ast.Expr) ast.Expr

	// Root returns the RootAssertionNode at the root of the tree this assertion node is part of,
	// if it is part of such a tree - otherwise returns nil
	// nilable(result 0)
	Root() *RootAssertionNode

	// Size returns an integer representing the number of objects in the tree
	// rooted at this AssertionNode - its use case is to determine whether
	// an AssertionNode has grown after a merge
	Size() int

	// MinimalString returns a minimal string representation of this assertion node
	// This is primarily for use when printing as part of a trackable expression chain,
	// such as f.g()[i].x
	MinimalString() string
}

type assertionNodeCommon struct {
	parent          AssertionNode // this should be nil for the root
	children        []AssertionNode
	consumeTriggers []*annotation.ConsumeTrigger

	// originalExpr stores the original call expression that prompted the creation of this assertion node
	originalExpr ast.Expr // this should be nil for the root

}

func (n *assertionNodeCommon) Parent() AssertionNode { return n.parent }

func (n *assertionNodeCommon) Children() []AssertionNode { return n.children }

func (n *assertionNodeCommon) ConsumeTriggers() []*annotation.ConsumeTrigger {
	return n.consumeTriggers
}

func (n *assertionNodeCommon) SetParent(other AssertionNode) { n.parent = other }

func (n *assertionNodeCommon) SetChildren(nodes []AssertionNode) { n.children = nodes }

func (n *assertionNodeCommon) SetConsumeTriggers(triggers []*annotation.ConsumeTrigger) {
	n.consumeTriggers = triggers
}

func (n *assertionNodeCommon) Root() *RootAssertionNode {
	if n == nil || n.parent == nil {
		return nil
	}
	return n.parent.Root()
}

func (n *assertionNodeCommon) Size() int {
	size := 1 + len(n.ConsumeTriggers())
	for _, child := range n.Children() {
		size += child.Size()
	}
	return size
}

func (n *assertionNodeCommon) BuildExpr(pass *analysis.Pass, expr ast.Expr) ast.Expr {
	return n.originalExpr
}
