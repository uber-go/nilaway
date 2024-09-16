//  Copyright (c) 2024 Uber Technologies, Inc.
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

package preprocess

import (
	"fmt"
	"go/ast"
	"go/token"

	"go.uber.org/nilaway/hook"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/cfg"
)

// CFG performs several passes on the CFG of utility to our analysis and returns a shallow copy of
// the modified CFG, with the original CFG untouched.
//
// Specifically, it performs the following modifications to the CFG:
//
// Canonicalize conditionals:
// - replace `if !cond {T} {F}` with `if cond {F} {T}` (swap successors)
// - replace `if cond1 && cond2 {T} {F}` with `if cond1 {if cond2 {T} else {F}}{F}` (nesting)
// - replace `if cond1 || cond2 {T} {F}` with `if cond1 {T} else {if cond2 {T} else {F}}` (nesting)
//
// Canonicalize nil comparisons:
// It also performs the following useful transformation:
// - replace `if x != nil {T} {F}` with `if x == nil {F} {T}` (swap successors)
// - replace `nil == x {T} {F}` with `if x == nil {T} {F}` (swap comparison order)
//
// Canonicalize explicit boolean comparisons:
// - replace `if x == true {T} {F}` with `if x {T} {F}`
// - replace `if x == false {T} {F}` with `if !x {T} {F}`
func (p *Preprocessor) CFG(graph *cfg.CFG, funcDecl *ast.FuncDecl) *cfg.CFG {
	// The ASTs and CFGs are shared across all analyzers in the nogo framework, so we should never
	// modify them directly. Here, we make a copy of the graph (and all blocks in it) and modify
	// the copied graph instead.
	graph = copyGraph(graph)

	// Important: add all new blocks to the end, don't try to "move around" any existing blocks
	// because they're all referenced by index!

	// Create a failure block at the end of the blocks list to be used for trusted functions.
	failureBlock := &cfg.Block{Index: int32(len(graph.Blocks))}
	graph.Blocks = append(graph.Blocks, failureBlock)

	// Perform a series of CFG transformations here (for hooks and canonicalization). The order of
	// these transformations matters due to canonicalization. Some transformations may expect the
	// CFG to be in canonical form, and some transformations may change the CFG structure in a way
	// that it needs to be re-canonicalized.

	// split blocks do not require the CFG to be in canonical form, and it may modify the CFG
	// structure in a way that it needs to be re-canonicalized. Here, we cleverly bundles the two
	// operations together such that we only need to run canonicalization once.
	for _, block := range graph.Blocks {
		if block.Live {
			p.splitBlockOnTrustedFuncs(graph, block, failureBlock)
		}
	}
	for _, block := range graph.Blocks {
		if block.Live {
			p.canonicalizeConditional(graph, block)
		}
	}
	// Replacing conditionals in the CFG requires the CFG to be in canonical form (such that it
	// does not have to handle "trustedFunc() && trustedFunc()"), and it will canonicalize the
	// modified block by itself.
	for _, block := range graph.Blocks {
		if block.Live {
			p.replaceConditional(graph, block)
		}
	}

	// Next, we need to re-insert information that is lost during CFG build for *ast.RangeStmt
	// and *ast.SwitchStmt by iterating through all blocks. This requires knowing the links between
	// the nodes contained within a block to their parents (*ast.RangeStmt or *ast.SwitchStmt nodes).
	// So, here establish the link and then do the work.
	rangeChildren, switchChildren := collectChildren(funcDecl)
	markRangeStatements(graph, rangeChildren)
	markSwitchStatements(graph, switchChildren)

	return graph
}

// copyGraph makes a semi-deep copy of the CFG and returns the copied graph. Note that only the
// graph itself is copied, i.e., the blocks and their edges (via block.Succs). The referenced AST
// nodes are _not_ copied (meaning we still should not modify the underlying AST nodes), but the
// slice storing the AST nodes (i.e., cfg.Block.Nodes) in each block is shallow-copied for modifications.
func copyGraph(graph *cfg.CFG) *cfg.CFG {
	// For some large graphs, a recursion-based approach will exceed the runtime stack size limit
	// and a stack-based approach will have many allocations / de-allocations. For best performance
	// (both in time and space), we run two iterations, one simply copying the blocks without
	// copying the edges (Succs), and another that copies the edges.
	newGraph := &cfg.CFG{}

	// Keep track of the mapping between original block ptr -> copied block ptr.
	copiedBlocks := make(map[*cfg.Block]*cfg.Block)
	for _, block := range graph.Blocks {
		// Shallow copy the slice that stores the AST nodes.
		newNodes := make([]ast.Node, len(block.Nodes))
		copy(newNodes, block.Nodes)

		// Copy the block and put it in the new graph.
		newBlock := &cfg.Block{
			Nodes: newNodes,
			Live:  block.Live,
			Index: block.Index,
		}
		newGraph.Blocks = append(newGraph.Blocks, newBlock)

		// Store the mapping.
		copiedBlocks[block] = newBlock
	}

	// Now, we iterate through the blocks again to fill in the edges (block.Succs). All blocks
	// must have already been copied.
	for i, newBlock := range newGraph.Blocks {
		for _, succ := range graph.Blocks[i].Succs {
			newBlock.Succs = append(newBlock.Succs, copiedBlocks[succ])
		}
	}

	return newGraph
}

// splitBlockOnTrustedFuncs splits the CFG block into two parts upon seeing a trusted function
// from the hook framework (e.g., "require.Nil(t, arg)" to "if arg == nil { <all code after> }".
// This does not expect the CFG to be in canonical form, and it may change the CFG structure in a
// way that it needs to be re-canonicalized.
func (p *Preprocessor) splitBlockOnTrustedFuncs(graph *cfg.CFG, thisBlock, failureBlock *cfg.Block) {
	for i, node := range thisBlock.Nodes {
		expr, ok := node.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		trustedCond := hook.SplitBlockOn(p.pass, call)
		if trustedCond == nil {
			continue
		}

		newBlockIndex := int32(len(graph.Blocks))
		newBlock := &cfg.Block{
			Nodes: append([]ast.Node{}, thisBlock.Nodes[i+1:]...),
			Succs: thisBlock.Succs,
			Index: newBlockIndex,
			Live:  true,
		}
		graph.Blocks = append(graph.Blocks, newBlock)
		thisBlock.Nodes = append(thisBlock.Nodes[:i+1], trustedCond)
		thisBlock.Succs = []*cfg.Block{
			newBlock,
			failureBlock,
		}
		failureBlock.Live = true
		p.splitBlockOnTrustedFuncs(graph, newBlock, failureBlock)
		return
	}
}

// replaceConditional calls the hook functions and replaces the conditional expressions in the CFG
// with the returned equivalent expression for analysis.
//
// This function expects the CFG to be in canonical form to fully function (otherwise it may miss
// cases like "trustedFunc() && trustedFunc()").
//
// It also calls canonicalizeConditional to canonicalize the transformed block such that the CFG
// is still canonical.
func (p *Preprocessor) replaceConditional(graph *cfg.CFG, block *cfg.Block) {
	// We only replace conditionals on branching blocks.
	if len(block.Nodes) == 0 || len(block.Succs) != 2 {
		return
	}
	call, ok := block.Nodes[len(block.Nodes)-1].(*ast.CallExpr)
	if !ok {
		return
	}
	replaced := hook.ReplaceConditional(p.pass, call)
	if replaced == nil {
		return
	}

	block.Nodes[len(block.Nodes)-1] = replaced
	// The returned expression may be a binary expression, so we need to canonicalize the CFG again
	// after such replacement.
	p.canonicalizeConditional(graph, block)
}

// canonicalizeConditional canonicalizes the conditional CFG structures to make it easier to reason
// about control flows later. For example, it rewrites
// `if !cond {T} {F}` to `if cond {F} {T}` (swap successors), and rewrites
// `if cond1 && cond2 {T} {F}` to `if cond1 {if cond2 {T} else {F}}{F}` (nesting).
func (p *Preprocessor) canonicalizeConditional(graph *cfg.CFG, thisBlock *cfg.Block) {
	// We only restructure non-empty branching blocks.
	if len(thisBlock.Nodes) == 0 || len(thisBlock.Succs) != 2 {
		return
	}

	trueBranch := thisBlock.Succs[0]  // type *cfg.Block
	falseBranch := thisBlock.Succs[1] // type *cfg.Block

	// A few helper functions to make the code more readable.
	replaceCond := func(node ast.Node) { thisBlock.Nodes[len(thisBlock.Nodes)-1] = node } // The conditional expr is the last node in the block.
	replaceTrueBranch := func(block *cfg.Block) { thisBlock.Succs[0] = block }
	replaceFalseBranch := func(block *cfg.Block) { thisBlock.Succs[1] = block }
	swapTrueFalseBranches := func() { replaceTrueBranch(falseBranch); replaceFalseBranch(trueBranch) }

	cond, ok := thisBlock.Nodes[len(thisBlock.Nodes)-1].(ast.Expr)
	if !ok {
		return
	}

	switch cond := cond.(type) {
	case *ast.ParenExpr:
		// if a parenexpr, strip and restart - this is done with recursion to account for ((((x)))) case
		replaceCond(cond.X)
		p.canonicalizeConditional(graph, thisBlock) // recur within parens
	case *ast.UnaryExpr:
		if cond.Op == token.NOT {
			// swap successors - i.e. swap true and false branches
			swapTrueFalseBranches()
			replaceCond(cond.X)
			p.canonicalizeConditional(graph, thisBlock) // recur within NOT
		}
	case *ast.BinaryExpr:
		// Logical AND and Logical OR actually require the exact same short circuiting behavior
		// except for whether the true or false branch leads to the short circuiting. This split
		// is captured by the following switch, and, as can be observed, all other logic is the
		// same
		binShortCircuit := func(replaceWhichBranch bool) {
			replaceCond(cond.X)
			newBlock := &cfg.Block{
				Nodes: []ast.Node{cond.Y},
				Succs: []*cfg.Block{trueBranch, falseBranch},
				Index: int32(len(graph.Blocks)),
				Live:  true,
			}
			if replaceWhichBranch {
				replaceTrueBranch(newBlock)
			} else {
				replaceFalseBranch(newBlock)
			}
			graph.Blocks = append(graph.Blocks, newBlock)
			p.canonicalizeConditional(graph, thisBlock)
			p.canonicalizeConditional(graph, newBlock)
		}

		// Standardize binary expressions to be of the form `expr OP literal` by swapping `x` and `y`, if `x` is a literal.
		// For example, standardizes `nil == v` to the `v == nil` form
		x, y := cond.X, cond.Y
		if util.IsLiteral(x, "nil", "true", "false") {
			newCond := &ast.BinaryExpr{
				// Swap X and Y
				X:     y,
				Y:     x,
				Op:    cond.Op,
				OpPos: cond.OpPos,
			}
			replaceCond(newCond)
			x, y = y, x
		}

		switch cond.Op {
		case token.LAND:
			binShortCircuit(true)
		case token.LOR:
			binShortCircuit(false)

		// The NEQ and EQL cases here rewrite the ASTs to ensure all _nil comparisons_ are
		// standardized to the form of `x == nil` (i.e., "variable" == "literal nil"). For example:
		// (1) x != nil -> x == nil, and swapping the true and false branches in the CFG.

		// Similarly, we also rewrite the ASTs for explicit _boolean comparisons_. For example,
		// (1) `ok == true` and `ok != false` -> `ok`
		// (2) `ok == false` and `ok != true` -> `!ok`, and swapping the true and false branches in the CFG.

		// Note that we _should not_ directly modify the AST nodes, since they are shared across
		// other nogo analyzers. Instead, whenever a rewrite is needed we create a new AST node
		// and replace the original node pointer with the clone in the block.Nodes slice instead.
		case token.NEQ:
			// Rewrite when operand `y` is a literal `nil`.
			if util.IsLiteral(y, "nil") {
				// Copy the AST Node first.
				newCond := &ast.BinaryExpr{
					X:     x,
					Y:     y,
					Op:    token.EQL, // As discussed, we change the operator to EQL here.
					OpPos: cond.OpPos,
				}
				// Replace the condition, and swap the branches since we modified a NEQ conditional
				// to a EQL one.
				replaceCond(newCond)
				swapTrueFalseBranches()
				break
			}

			// For explicit boolean NEQ checks, we replace the AST nodes for `ok != true` and `ok != false`
			// (also, `true != ok` and `false != ok`) with `ok` and `!ok` form for the true and false cases, respectively.
			if util.IsLiteral(y, "false") {
				replaceCond(x) // replaces `ok != false` with `ok`
			} else if util.IsLiteral(y, "true") {
				newCond := &ast.UnaryExpr{
					OpPos: y.Pos(),
					Op:    token.NOT,
					X:     x,
				}
				replaceCond(newCond)                        // replaces `ok != true` with `!ok`
				p.canonicalizeConditional(graph, thisBlock) // recur to swap true and false branches for the unary expr `!ok`
			}

		case token.EQL:
			// For explicit boolean EQL checks, we replace the AST nodes for `ok == true` and `ok == false`
			// (also, `true == ok` and `false == ok`) with `ok` and `!ok` form for the true and false cases, respectively.
			if util.IsLiteral(y, "true") {
				replaceCond(x) // replaces `ok == true` with `ok`
			} else if util.IsLiteral(y, "false") {
				newCond := &ast.UnaryExpr{
					OpPos: y.Pos(),
					Op:    token.NOT,
					X:     x,
				}
				replaceCond(newCond)                        // replaces `ok == false` with `!ok`
				p.canonicalizeConditional(graph, thisBlock) // recur to swap true and false branches for the unary expr `!ok`
			}
		}
	}
}

// collectChildren establishes the links between the range / switch statement nodes and their child
// nodes. This is specifically designed for our preprocess function: when we rewrite the CFG to
// re-insert the lost information, we need to know if a block in CFG belongs to a certain range
// statement or switch statement AST node for retrieving lost information.
func collectChildren(funcDecl *ast.FuncDecl) (map[ast.Node]*ast.RangeStmt, map[ast.Node]*ast.SwitchStmt) {
	rangeChildren, switchChildren := make(map[ast.Node]*ast.RangeStmt), make(map[ast.Node]*ast.SwitchStmt)

	ast.Inspect(funcDecl, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.RangeStmt:
			if n.Key != nil {
				rangeChildren[n.Key] = n
			}
			if n.Value != nil {
				rangeChildren[n.Value] = n
			}
			rangeChildren[n.X] = n
			rangeChildren[n.Body] = n
		case *ast.SwitchStmt:
			if n.Init != nil {
				switchChildren[n.Init] = n
			}
			if n.Tag != nil {
				switchChildren[n.Tag] = n
			}
			switchChildren[n.Body] = n
		}
		return true
	})

	return rangeChildren, switchChildren
}

// markRangeStatements rewrites a cfg to reflect ranging loops - the assignments in a `for... range y {}`
// loop are by default erased in the CFG pass, so we match on the structure of all blocks in the CFG
// and their AST nodes to rediscover and reinsert these assignments or, in the case of a `for range`
// loop with no assignments - we insert a fresh *ast.UnaryExpr simply indicating that this is a range
func markRangeStatements(graph *cfg.CFG, rangeChildren map[ast.Node]*ast.RangeStmt) {
	for _, block := range graph.Blocks {
		n := len(block.Nodes)
		if n < 1 {
			continue
		}

		rangeStmt := rangeChildren[block.Nodes[n-1]]
		if rangeStmt == nil {
			continue
		}

		// we have a `range` statement! now time to figure out which one
		rawRangeExpr := &ast.UnaryExpr{
			OpPos: rangeStmt.For,
			Op:    token.RANGE,
			X:     rangeStmt.X,
		}

		singleRangeExpr := func(expr ast.Expr) *ast.AssignStmt {
			return &ast.AssignStmt{
				Lhs:    []ast.Expr{expr},
				TokPos: rangeStmt.TokPos,
				Tok:    rangeStmt.Tok,
				Rhs:    []ast.Expr{rawRangeExpr},
			}
		}

		doubleRangeExpr := func(expr1 ast.Expr, expr2 ast.Expr) *ast.AssignStmt {
			return &ast.AssignStmt{
				Lhs:    []ast.Expr{expr1, expr2},
				TokPos: rangeStmt.TokPos,
				Tok:    rangeStmt.Tok,
				Rhs:    []ast.Expr{rawRangeExpr},
			}
		}

		if rangeStmt.Key == nil {
			// we have a `for range expr {}` loop
			if rangeStmt.X == block.Nodes[n-1] {
				block.Nodes = append(block.Nodes[:n-1], rawRangeExpr)
			}
		} else if rangeStmt.Value == nil {
			// we have a `for x := range expr {}` loop
			if rangeStmt.Key == block.Nodes[n-1] &&
				rangeStmt.X == block.Nodes[n-2] {
				block.Nodes = append(block.Nodes[:n-2], singleRangeExpr(rangeStmt.Key))
			}
		} else {
			// we have a `for x, y := range expr {}` loop
			if rangeStmt.Value == block.Nodes[n-1] &&
				rangeStmt.Key == block.Nodes[n-2] &&
				rangeStmt.X == block.Nodes[n-3] {
				block.Nodes = append(block.Nodes[:n-3], doubleRangeExpr(rangeStmt.Key, rangeStmt.Value))
			}
		}
	}
}

// markSwitchStatements restructures a cfg to reflect switch statements.
//
// In particular, `switch x { case y0 : e0 case y1 : e1 ... }` will be parsed by the CFG into:
//
// Block0: Nodes: x, y0, Succs: Block1, Block2
// Block1: e0
// Block2: Nodes: y1, Succs: Block3, Block4
// Block3: e1
// Block4: Nodes: y2, Succs: Block5, Block6
//
// Which we transform into:
//
// Block0: Nodes: x == y0, Succs: Block1, Block2
// Block1: e0
// Block2: Nodes: x == y1, Succs: Block3, Block4
// Block3: e1
// Block4: Nodes: x == y2, Succs: Block5, Block6
//
// This will allow the existing logic for reading conditionals from the CFG to handle `switch` statements,
// which simply checks that the block ends with a Binary check like `x == y` and has two successors.
//
// invariant - consecutive cases of a switch statement have block numbers whose ordering
// reflects the syntactic ordering of the cases - if a case were to have a lower block number
// than its initial switch statement this would be broken
func markSwitchStatements(graph *cfg.CFG, switchChildren map[ast.Node]*ast.SwitchStmt) {
	knownCaseBlockIdxs := make(map[int32]bool)

	for i, block := range graph.Blocks {
		if knownCaseBlockIdxs[int32(i)] {
			continue
		}

		n := len(block.Nodes)
		if n < 2 {
			continue
		}

		switchExpr, ok := block.Nodes[n-2].(ast.Expr)
		if !ok {
			continue
		}
		if switchChildren[switchExpr] == nil {
			continue
		}

		// we've found a switch statement!
		caseExpr := block.Nodes[n-1].(ast.Expr)
		block.Nodes = append(block.Nodes[:n-2], &ast.BinaryExpr{
			X:     switchExpr,
			OpPos: caseExpr.Pos(), // use the position of the case expression
			Op:    token.EQL,
			Y:     caseExpr,
		})

		if len(block.Succs) != 2 {
			panic(fmt.Sprintf("Inspection of switch statement failed - "+
				"assumption of two successors for first case block violated: "+
				"found %d", len(block.Succs)))
		}

		knownCaseBlockIdxs[block.Index] = true
		caseBlockIdx := block.Succs[1].Index
		for len(graph.Blocks[caseBlockIdx].Succs) == 2 {
			knownCaseBlockIdxs[caseBlockIdx] = true
			caseBlock := graph.Blocks[caseBlockIdx]

			if len(caseBlock.Nodes) != 1 {
				panic(fmt.Sprintf("Inspection of switch statement failed "+
					"- assumption of single node in non-first case block "+
					"violated: found %d", len(caseBlock.Nodes)))
			}

			caseBlock.Nodes = []ast.Node{
				&ast.BinaryExpr{
					X:     switchExpr,
					OpPos: caseBlock.Nodes[0].Pos(), // use the position of the case expression
					Op:    token.EQL,
					Y:     caseBlock.Nodes[0].(ast.Expr),
				},
			}

			if blockSuccs := graph.Blocks[caseBlockIdx].Succs; blockSuccs != nil {
				caseBlockIdx = blockSuccs[1].Index
			}
		}
	}
}
