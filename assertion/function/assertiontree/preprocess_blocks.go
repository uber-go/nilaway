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
	"fmt"
	"go/ast"
	"go/token"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/cfg"
)

// preprocess performs several passes on the CFG of utility to our analysis.
// Notably, it also generates a slice of RichCheckEffects for each block and
// returns it as a separate object, but does all other modification in place.
//
// The returned RichCheckEffect slices represent the RichCheckEffects present at
// the _end_ of each block
func preprocess(graph *cfg.CFG, fc FunctionContext) (*cfg.CFG, [][]RichCheckEffect, util.ExprNonceMap) {
	// The ASTs and CFGs are shared across all analyzers in the nogo framework, so we should never
	// modify them directly. Here, we make a copy of the graph (and all blocks in it) and modify
	// the copied graph instead.
	graph = copyGraph(graph)
	restructureBlocks(graph, fc.pass)
	richCheckBlocks, exprNonceMap := genInitialRichCheckEffects(graph, fc)
	richCheckBlocks = propagateRichChecks(graph, richCheckBlocks)

	// Next, we need to re-insert information that is lost during CFG build for *ast.RangeStmt
	// and *ast.SwitchStmt by iterating through all blocks. This requires knowing the links between
	// the nodes contained within a block to their parents (*ast.RangeStmt or *ast.SwitchStmt nodes).
	// So, here establish the link and then do the work.
	rangeChildren, switchChildren := collectChildren(fc.funcDecl)
	markRangeStatements(graph, rangeChildren)
	markSwitchStatements(graph, switchChildren)

	return graph, richCheckBlocks, exprNonceMap
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

// stripNoops returns a copy of the passed slice `effects`, minus any no-ops
func stripNoops(effects []RichCheckEffect) []RichCheckEffect {
	var strippedEffects []RichCheckEffect

	for _, effect := range effects {
		if !effect.isNoop() {
			strippedEffects = append(strippedEffects, effect)
		}
	}

	return strippedEffects
}

// genInitialRichCheckEffects computes an initial array of RichCheckEffect slices for each block,
// not doing any propagation over the CFG except for within each block to track nodes
// that create RichCheckEffects (such as `v, ok := mp[k]`) and make sure it isn't invalidated
// (such as by `ok = true`) before the end of the block.
//
// The returned RichCheckEffect slices represent the RichCheckEffects present at
// the _end_ of each block.
//
// Important: do not duplicate any pointers: each returned RichCheckEffect should be a unique object
func genInitialRichCheckEffects(graph *cfg.CFG, functionContext FunctionContext) (
	[][]RichCheckEffect, util.ExprNonceMap) {
	richCheckBlocks := make([][]RichCheckEffect, len(graph.Blocks))
	nonceGenerator := util.NewGuardNonceGenerator()

	// There is no canonical instance of RootAssertionNode until backpropAcrossFunc returns.
	// We use a temporary root here as a means to pass contextual information like the function
	// declaration and analysis pass.
	rootNode := newRootAssertionNode(nonceGenerator.GetExprNonceMap(), functionContext)
	for i, block := range graph.Blocks {
		var richCheckEffects []RichCheckEffect
		for _, node := range block.Nodes {

			// invalidate any richCheckEffects that this node invalidates
			for j, effect := range richCheckEffects {
				if effect.isInvalidatedBy(node) {
					richCheckEffects[j] = RichCheckNoop{}
				}
			}

			// check if this node produces a new richCheckEffect
			if effects, ok := RichCheckFromNode(rootNode, nonceGenerator, node); ok {
				richCheckEffects = append(richCheckEffects, effects...)
			}
		}
		// richCheckEffects is now fully populated

		// strip out noops and write into richCheckBlocks
		richCheckBlocks[i] = stripNoops(richCheckEffects)
	}
	return richCheckBlocks, nonceGenerator.GetExprNonceMap()
}

// This function restructures a cfg to reflect short-circuiting and other interesting semantics:
//
// It performs the following short-circuiting:
// - replace if !cond {T} {F} with if cond {F} {T} (in CFG, swap successors)
// - replace if cond1 && cond2 {T} {F} with if cond1 {if cond2 {T} else {F}}{F}
// - replace if cond1 || cond2 {T} {F} with if cond1 {T} else {if cond2 {T} else {F}}
//
// It also performs the following useful transformation:
// - replace if x != nil {T} {F} with if x == nil {F} {T} (i.e. swap successors)
// - replace nil == x {T} {F} with if x == nil {T} {F} (i.e. swap comparison order)
//
// In addition, it also performs the following transformations to standardize explicit boolean comparisons:
// - replace if x == true {T} {F} with if x {T} {F}
// - replace if x == false {T} {F} with if !x {T} {F}
func restructureBlocks(graph *cfg.CFG, pass *analysis.Pass) {
	failureBlock := &cfg.Block{
		Nodes: nil,
		Succs: nil,
		Index: int32(len(graph.Blocks)),
		Live:  false,
	}
	graph.Blocks = append(graph.Blocks, failureBlock)

	// important: add all new blocks to the end, don't try to "move around" any existing blocks because they're all
	// referenced by index!
	for _, block := range graph.Blocks {
		if block.Live {
			splitBlockOnTrustedFuncs(graph, block, failureBlock, pass)
		}
	}
	for _, block := range graph.Blocks {
		if block.Live {
			restructureBlock(graph, block)
		}
	}
}

func splitBlockOnTrustedFuncs(graph *cfg.CFG, thisBlock, failureBlock *cfg.Block, pass *analysis.Pass) {
	var expr *ast.ExprStmt
	var call *ast.CallExpr
	var retExpr any
	var trustedCond ast.Expr
	var ok bool

	for i, node := range thisBlock.Nodes {
		if expr, ok = node.(*ast.ExprStmt); !ok {
			continue
		}
		if call, ok = expr.X.(*ast.CallExpr); !ok {
			continue
		}
		if retExpr, ok = AsTrustedFuncAction(call, pass); !ok {
			continue
		}
		if trustedCond, ok = retExpr.(ast.Expr); !ok {
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
		splitBlockOnTrustedFuncs(graph, newBlock, failureBlock, pass)
		return
	}
}

func restructureBlock(graph *cfg.CFG, thisBlock *cfg.Block) {
	// TODO: This check should not be needed since `getConditional != nil` implies
	//  `thisBlock.Succs != nil` due to the length check inside `getConditional`. However, due to a
	//  FP we have to add this redundant check. This should not be needed after  is fixed.
	if thisBlock.Succs == nil {
		return
	}
	cond := getConditional(thisBlock)
	if cond == nil {
		return
	}

	// places a new given node into the last position of this block
	replaceCond := func(node ast.Node) {
		thisBlock.Nodes[len(thisBlock.Nodes)-1] = node
	}

	trueBranch := thisBlock.Succs[0]  // type *cfg.Block
	falseBranch := thisBlock.Succs[1] // type *cfg.Block

	replaceTrueBranch := func(block *cfg.Block) {
		thisBlock.Succs[0] = block
	}
	replaceFalseBranch := func(block *cfg.Block) {
		thisBlock.Succs[1] = block
	}

	swapTrueFalseBranches := func() {
		replaceTrueBranch(falseBranch)
		replaceFalseBranch(trueBranch)
	}

	switch cond := cond.(type) {
	case *ast.ParenExpr:
		// if a parenexpr, strip and restart - this is done with recursion to account for ((((x)))) case
		replaceCond(cond.X)
		restructureBlock(graph, thisBlock) // recur within parens
	case *ast.UnaryExpr:
		if cond.Op == token.NOT {
			// swap successors - i.e. swap true and false branches
			swapTrueFalseBranches()
			replaceCond(cond.X)
			restructureBlock(graph, thisBlock) // recur within NOT
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
			restructureBlock(graph, thisBlock)
			restructureBlock(graph, newBlock)
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
				replaceCond(newCond)               // replaces `ok != true` with `!ok`
				restructureBlock(graph, thisBlock) // recur to swap true and false branches for the unary expr `!ok`
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
				replaceCond(newCond)               // replaces `ok == false` with `!ok`
				restructureBlock(graph, thisBlock) // recur to swap true and false branches for the unary expr `!ok`
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

func mergeSlices(useDeepEquality bool, left []RichCheckEffect, rights ...[]RichCheckEffect) []RichCheckEffect {
	var eq func(first, second RichCheckEffect) bool
	if useDeepEquality {
		eq = func(first, second RichCheckEffect) bool {
			return first.equals(second)
		}
	} else {
		eq = func(first, second RichCheckEffect) bool {
			return first == second
		}
	}
	var out []RichCheckEffect
	addToOut := func(effect RichCheckEffect) {
		for _, outEffect := range out {
			if eq(outEffect, effect) {
				return
			}
		}
		out = append(out, effect)
	}
	for _, l := range left {
		addToOut(l)
	}
	for _, right := range rights {
		for _, r := range right {
			addToOut(r)
		}
	}
	return out
}

func genPreds(graph *cfg.CFG) [][]int32 {
	out := make([][]int32, len(graph.Blocks))
	for _, block := range graph.Blocks {
		if block.Live {
			for _, succ := range block.Succs {
				out[succ.Index] = append(out[succ.Index], block.Index)
			}
		}
	}
	return out
}

// RichCheckEffectSlicesString returns a slice of RichCheckEffect slices as a string representation
func RichCheckEffectSlicesString(name string, richCheckBlocks [][]RichCheckEffect) string {
	out := fmt.Sprintf("%s len %d: {\n", name, len(richCheckBlocks))
	for i, richCheckEffects := range richCheckBlocks {
		repr := "nil"
		if richCheckEffects != nil {
			repr = "{"
			for _, richCheckEffect := range richCheckEffects {
				repr += fmt.Sprintf("%s; ", richCheckEffect.String())
			}
			repr += "}"
		}
		out += fmt.Sprintf("\t%s[%d]: %s\n", name, i, repr)
	}
	return out + "}"
}

// weakPropagateRichChecks performs a simple form of propagation of rich checks: for each effect, it
// figures out which blocks are reachable from the block it was declared in.
//
// The results are returned as a map from `RichCheckEffect`s to arrays of booleans, representing for
// each block whether it is reached by the block that effect is declared in
func weakPropagateRichChecks(graph *cfg.CFG, richCheckBlocks [][]RichCheckEffect) map[RichCheckEffect][]bool {
	reachability := make(map[RichCheckEffect][]bool)
	for blockNum := range richCheckBlocks {
		for _, check := range richCheckBlocks[blockNum] {
			newCheck := make([]bool, len(richCheckBlocks))
			newCheck[blockNum] = true // mark each check as reachable in its declaring block
			reachability[check] = newCheck
		}
	}
	done := false
	for !done {
		done = true
		for blockNum := range richCheckBlocks {
			for _, reachable := range reachability {
				if reachable[blockNum] {
					for _, nextBlock := range graph.Blocks[blockNum].Succs {
						if !reachable[nextBlock.Index] {
							reachable[nextBlock.Index] = true
							done = false
						}
					}
				}
			}
		}
	}
	return reachability
}

// propagateRichChecks takes an initial array richCheckBlocks and flows all of its contained checks
// forwards through the CFG as long as they are not invalidated. A check created by a node in block A
// is determined to flow to block B if every path from A to B does not invalidate the check. We capture
// this criterion by first calling the function weakPropagateRichChecks above to do reachability
// propagation without any knowledge of check invalidation. The real propagation done in this function
// then tempers its computation of checks at a given block via intersection at control flow points by
// including exactly those checks that are present in every predecessor of the block that is reachable
// from the originator block of the check.
func propagateRichChecks(graph *cfg.CFG, richCheckBlocks [][]RichCheckEffect) [][]RichCheckEffect {
	n := len(graph.Blocks)
	if len(richCheckBlocks) != n {
		panic(fmt.Sprintf("richCheckBlocks (len %d) and graph.blocks (len %d) out of "+
			"sync - fix generation pass in preprocess_blocks.go", len(richCheckBlocks), n))
	}

	effectReaches := weakPropagateRichChecks(graph, richCheckBlocks)

	currBlocks := richCheckBlocks
	nextBlocks := make([][]RichCheckEffect, n)

	preds := genPreds(graph)
	roundCount := 0

	done := false

	for !done {

		done = true

		for i := range preds {

			// predRichCheckEffects will be populated with all the rich bool effects that flow
			// into this block from one of its 0 or more predecessors
			var predRichCheckEffects []RichCheckEffect

			if len(preds[i]) >= 1 {
				reachingEffects := make(map[RichCheckEffect]bool)

				for _, predIndex := range preds[i] {
					for _, effect := range currBlocks[predIndex] {
						// for each effect in a predecessor, mark it as `true` in `reachingEffects`
						// - performing a merge
						reachingEffects[effect] = true
					}
				}

				for _, predIndex := range preds[i] {
					maskingEffects := make(map[RichCheckEffect]bool)
					for effect := range reachingEffects {
						if blocksEffectReaches, ok := effectReaches[effect]; ok &&
							blocksEffectReaches[predIndex] {
							maskingEffects[effect] = true
						}
					}
					for _, effect := range currBlocks[predIndex] {
						if maskingEffects[effect] {
							maskingEffects[effect] = false
						}
					}
					for effect, present := range maskingEffects {
						if present {
							reachingEffects[effect] = false
						}
					}
				}

				predRichCheckEffects = make([]RichCheckEffect, 0)

				for effect := range reachingEffects {
					if reachingEffects[effect] {
						predRichCheckEffects = append(predRichCheckEffects, effect)
					}
				}

				// This code performs a simple merge instead - but this is very unsound and NOT right
				// 		predRichCheckEffects =
				// 			append(make([]RichCheckEffect, 0, len(currBlocks[preds[i][0]])),
				// 				currBlocks[preds[i][0]]...)
				//
				// 		for _, predNum := range preds[i][1:] {
				// 			predRichCheckEffects = mergeSlices(false, predRichCheckEffects, currBlocks[predNum])
				// 		}

				for _, node := range graph.Blocks[i].Nodes {
					// invalidate any richCheckEffects that this node invalidates
					for j, effect := range predRichCheckEffects {
						if effect.isInvalidatedBy(node) {
							predRichCheckEffects[j] = RichCheckNoop{}
						}
					}
				}
			}

			nextBlocks[i] = mergeSlices(false, currBlocks[i], stripNoops(predRichCheckEffects))
			if len(nextBlocks[i]) > len(currBlocks[i]) {
				done = false
			}
		}

		currBlocks = nextBlocks
		nextBlocks = make([][]RichCheckEffect, n)

		roundCount++

		config.CheckCFGFixedPointRuntime(
			"RichCheckEffect Forwards Propagation", n, roundCount)
	}

	// this strips duplicates from the RichCheckEffect slices
	for i := range currBlocks {
		currBlocks[i] = mergeSlices(true, currBlocks[i])
	}

	return currBlocks
}
