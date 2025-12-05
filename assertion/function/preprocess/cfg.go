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
	"go/types"

	"go.uber.org/nilaway/hook"
	"go.uber.org/nilaway/util"
	"go.uber.org/nilaway/util/asthelper"
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
			p.restructureOnNoReturnCall(block)
		}
	}
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

	// Please check the docstring of the following call to see why this is needed.
	// TODO: remove this once anonymous function support handles it naturally.
	p.inlineTemplComponentFuncLit(graph, funcDecl)

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

func (p *Preprocessor) restructureOnNoReturnCall(block *cfg.Block) {
	if len(block.Nodes) == 0 || len(block.Succs) == 0 {
		return
	}

	for i, node := range block.Nodes {
		expr, ok := node.(*ast.ExprStmt)
		if !ok {
			continue
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			continue
		}

		if hook.IsNoReturnCall(p.pass, call) {
			block.Nodes = block.Nodes[:i] // The rest of the nodes are now unreachable.
			block.Succs = nil             // There will be no successor block.
			return
		}
	}
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

	var call *ast.CallExpr

	switch lastNode := block.Nodes[len(block.Nodes)-1].(type) {
	// Last node is a call expression for `if foo() { ... }` case.
	case *ast.CallExpr:
		call = lastNode
	// Otherwise, we check if it is `if ok := foo(); ok { ... }` case.
	// Note that this would fail for the following case:
	//
	// ok := foo()
	// if dummy {
	//   if ok {
	//     ...
	//   }
	// }
	//
	// (The example above is canonicalized -- `if dummy && ok {...}` is equivalent, and is
	// probably more common in practice).
	//
	// Here we will not find the declaration of `ok` in the block. Ideally we should really find
	// the declaration node of `ok` instead of simply checking the last node in the block (possibly
	// with the help of SSA).
	// TODO: implement that.
	case *ast.Ident:
		if len(block.Nodes) < 2 {
			break
		}
		assign, ok := block.Nodes[len(block.Nodes)-2].(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			break
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			break
		}
		if ident.Name != lastNode.Name {
			break
		}
		call, _ = assign.Rhs[0].(*ast.CallExpr)
	}

	if call == nil {
		return
	}

	// First, try replacing with a hook-based conditional transformation.
	if hookReplacement := hook.ReplaceConditional(p.pass, call); hookReplacement != nil {
		block.Nodes[len(block.Nodes)-1] = hookReplacement
		p.canonicalizeConditional(graph, block)
		return
	}

	// Second, try extracting and inlining a custom function's boolean expression.
	// Example:
	// ```
	// func isValid(x *int) bool {
	// 	return x != nil
	// }
	// func foo() {
	// 	var p *int
	// 	if isValid(p) {
	// 		print(*p)
	//	}
	// ```
	// This will transform the condition to: `if isValid(p) { ... }` to `if p != nil { ... }` by extracting the
	// boolean expression from isValid and substituting the parameter with the argument. This inlining allows NilAway to
	// correctly infer the nilability of the argument after the custom function call.
	//
	// Note: We append the conditional instead of replace here, as we want to leverage NilAway's trigger logic if
	// there is a potential nil panic in the custom function.
	// For example,
	// ```
	// L1. func isValid(x *int) bool {
	// L2. 	return x != nil || *x > 0 // nil panic!
	// L3. }
	// L4. func foo() {
	// L5. 	var p *int
	// L6. 	if isValid(p) {
	// L7. 		print(*p)
	// L8. 	}
	// L9. }
	// ```
	// In this case, the advantage of appending instead of replacing can be seen:
	// - If we "replace" the call with the adapted binary expression (`L6. if isValid(p)` with `L6. if p != nil || *p > 0`),
	// NilAway will report the error at the call site on L6: "unassigned variable `p` dereferenced". This would be confusing
	// for the users because the error should actually be reported at L2.
	// - If we "append" the adapted binary expression, leaving the original call intact, NilAway will correctly report the error at the call site on L2:
	// 		"L6: unassigned variable `p` passed as arg `x` to `isValid()`"
	// 		"L2: function parameter `x` dereferenced".
	// The downside of this however, is that NilAway will also report a duplicate error at L6 for the nil panic in the adapted binary expression
	// "L6: unassigned variable `p` dereferenced".
	// This is a conscious trade-off that we are making to keep the logic simpler and more efficient, while still reporting the correct errors,
	// and better user experience. Also, we don't expect binary expressions in custom functions to be nil panic-prone, so the downside is manageable.
	// TODO: ideally, we should expand the capability of the contract analyzer to handle the custom function logic to be more robust and accurate.
	//  We can remove this logic in pre-preprocessing once we do that.
	if customFuncReplacement := p.inlineBoolFunc(call); customFuncReplacement != nil {
		block.Nodes = append(block.Nodes, customFuncReplacement)
		p.canonicalizeConditional(graph, block)
	}
}

// inlineBoolFunc inlines a boolean expression from a custom function (i.e. nil-check function) into the call site.
// For example:
// ```
//
//	func isPtrNonnil(x *int) bool {
//		return x != nil && *x > 0
//	}
//	func foo(p *int) {
//		if isPtrNonnil(p) {
//			print(*p)
//		}
//	}
//
// ```
// This will transform the condition to: `if isPtrNonnil(p) { ... }` to `if p != nil && *p > 0 { ... }` by extracting the
// binary expression from isPtrNonnil and substituting the parameter with the argument. This inlining allows NilAway to
// correctly infer the nilability of the argument after the custom function call.
func (p *Preprocessor) inlineBoolFunc(call *ast.CallExpr) *ast.BinaryExpr {
	calledFuncDecl := p.findFuncDeclFromCallExpr(call)
	if calledFuncDecl == nil {
		return nil
	}

	if binaryExpr := p.extractBinaryExpressionFromFunc(calledFuncDecl); binaryExpr != nil {
		return p.adaptBinaryExpr(binaryExpr, calledFuncDecl, call)
	}
	return nil
}

// findFuncDeclFromCallExpr finds the function declaration from a call expression.
func (p *Preprocessor) findFuncDeclFromCallExpr(call *ast.CallExpr) *ast.FuncDecl {
	ident := util.FuncIdentFromCallExpr(call)

	if ident == nil || ident.Obj == nil || ident.Obj.Decl == nil {
		return nil
	}
	funcDecl, ok := ident.Obj.Decl.(*ast.FuncDecl)
	if !ok {
		return nil
	}
	return funcDecl
}

// extractBinaryExpressionFromFunc extracts a binary expression from a function if:
// 1. The function returns a boolean value
// 2. The function body contains only one statement, and that statement is a return of a binary expression.
// For example, `func isPtrNonnil(x *int) bool { return x != nil }` will return the binary expression `x != nil`.
func (p *Preprocessor) extractBinaryExpressionFromFunc(funcDecl *ast.FuncDecl) *ast.BinaryExpr {
	if funcDecl == nil || funcDecl.Body == nil {
		return nil
	}

	var funcObj *types.Func
	if obj := p.pass.Pass.TypesInfo.ObjectOf(funcDecl.Name); obj != nil {
		funcObj, _ = obj.(*types.Func)
	}
	if funcObj == nil {
		return nil
	}

	// Check if the function returns a boolean value.
	results := funcObj.Signature().Results()
	n := results.Len()
	if n != 1 || !types.Identical(results.At(n-1).Type(), util.BoolType) {
		return nil
	}

	// Check if the function only contains one return statement of a binary expression.
	if len(funcDecl.Body.List) != 1 {
		return nil
	}
	retStmt, ok := funcDecl.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(retStmt.Results) != 1 {
		return nil
	}
	binExpr, ok := retStmt.Results[0].(*ast.BinaryExpr)
	if !ok {
		return nil
	}
	return binExpr
}

// adaptBinaryExpr adapts identifiers in a binary expression inside funcDecl
// by replacing function parameter identifiers with the corresponding call arguments.
// It handles nested expressions (unary, binary, calls, selectors, indexing, etc.).
func (p *Preprocessor) adaptBinaryExpr(binaryExpr *ast.BinaryExpr, funcDecl *ast.FuncDecl, call *ast.CallExpr) *ast.BinaryExpr {
	if binaryExpr == nil || funcDecl == nil || call == nil || funcDecl.Type.Params == nil {
		return nil // early return on nil inputs
	}

	// Build a mapping from parameter name -> argument expression
	paramToArg := make(map[string]ast.Expr)
	var paramFlatList []*ast.Ident
	for _, field := range funcDecl.Type.Params.List {
		for _, name := range field.Names {
			paramFlatList = append(paramFlatList, name)
		}
	}
	if len(paramFlatList) != len(call.Args) {
		return nil // early return if mismatch
	}

	for i, param := range paramFlatList {
		paramToArg[param.Name] = call.Args[i]
	}

	// Rebuild the binary expression with fully adapted subtrees
	return &ast.BinaryExpr{
		X:     p.adaptExprDeep(binaryExpr.X, paramToArg),
		OpPos: binaryExpr.OpPos,
		Op:    binaryExpr.Op,
		Y:     p.adaptExprDeep(binaryExpr.Y, paramToArg),
	}
}

// adaptExprDeep recursively substitutes identifiers that match function parameters
// with their corresponding call-argument expressions throughout the expression tree.
// Note that adaptExprDeep always returns a new expression tree and does not modify
// the input expression in place.
func (p *Preprocessor) adaptExprDeep(expr ast.Expr, subst map[string]ast.Expr) ast.Expr {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *ast.Ident:
		if arg, ok := subst[e.Name]; ok && arg != nil {
			return arg
		}
		return e

	case *ast.ParenExpr:
		return &ast.ParenExpr{
			X:      p.adaptExprDeep(e.X, subst),
			Lparen: e.Lparen,
			Rparen: e.Rparen,
		}

	case *ast.UnaryExpr:
		return &ast.UnaryExpr{
			X:     p.adaptExprDeep(e.X, subst),
			Op:    e.Op,
			OpPos: e.OpPos,
		}

	case *ast.BinaryExpr:
		return &ast.BinaryExpr{
			X:     p.adaptExprDeep(e.X, subst),
			Y:     p.adaptExprDeep(e.Y, subst),
			Op:    e.Op,
			OpPos: e.OpPos,
		}

	case *ast.CallExpr:
		newArgs := make([]ast.Expr, len(e.Args))
		for i, arg := range e.Args {
			newArgs[i] = p.adaptExprDeep(arg, subst)
		}
		return &ast.CallExpr{
			Fun:      p.adaptExprDeep(e.Fun, subst),
			Args:     newArgs,
			Ellipsis: e.Ellipsis,
			Lparen:   e.Lparen,
			Rparen:   e.Rparen,
		}

	case *ast.SelectorExpr:
		return &ast.SelectorExpr{
			X:   p.adaptExprDeep(e.X, subst),
			Sel: e.Sel, // field/method name stays as-is
		}

	case *ast.IndexExpr:
		return &ast.IndexExpr{
			X:      p.adaptExprDeep(e.X, subst),
			Index:  p.adaptExprDeep(e.Index, subst),
			Lbrack: e.Lbrack,
			Rbrack: e.Rbrack,
		}

	case *ast.StarExpr:
		return &ast.StarExpr{
			X:    p.adaptExprDeep(e.X, subst),
			Star: e.Star,
		}

	case *ast.TypeAssertExpr:
		return &ast.TypeAssertExpr{
			X:      p.adaptExprDeep(e.X, subst),
			Type:   e.Type, // type stays as-is
			Lparen: e.Lparen,
			Rparen: e.Rparen,
		}

	case *ast.SliceExpr:
		return &ast.SliceExpr{
			X:      p.adaptExprDeep(e.X, subst),
			Low:    p.adaptExprDeep(e.Low, subst),
			High:   p.adaptExprDeep(e.High, subst),
			Max:    p.adaptExprDeep(e.Max, subst),
			Slice3: e.Slice3,
			Lbrack: e.Lbrack,
			Rbrack: e.Rbrack,
		}

	default:
		// Unhandled node kinds are returned as-is.
		return e
	}
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
		if asthelper.IsLiteral(x, "nil", "true", "false") {
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
			if asthelper.IsLiteral(y, "nil") {
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
			if asthelper.IsLiteral(y, "false") {
				replaceCond(x) // replaces `ok != false` with `ok`
			} else if asthelper.IsLiteral(y, "true") {
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
			if asthelper.IsLiteral(y, "true") {
				replaceCond(x) // replaces `ok == true` with `ok`
			} else if asthelper.IsLiteral(y, "false") {
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
