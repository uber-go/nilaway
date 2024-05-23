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
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util"
	"go.uber.org/nilaway/util/asthelper"
	"golang.org/x/tools/go/cfg"
)

// A RichCheckEffect is the fact that a certain check is associated with an effect that can
// be triggered by a conditional, for example the `ok` in `v, ok := m[k]`
//
// the functions `effectIfTrue` and `effectIfFalse` are analogous to the respective returns from
// `AddNilCheck` - functions that are marked as preprocessing at the beginning of successor blocks
// to a conditional that matches the trigger. In this case, an expression in a conditional matching
// the trigger is determined by the interface function `isTriggeredBy`. There are certain statements
// that, if encountered between the establishment of the RichCheckEffect and the trigger, invalidate its
// effect. For example, for the `ok` in `v, ok := m[k]`, an assignment to either `v` or `ok` invalidates
// the effect. Whether an expression invalidates this effect is determined by the interface function
// `isInvalidatedBy`.
type RichCheckEffect interface {
	// isTriggeredBy indicates whether a given expression in a conditional is sufficient to trigger
	// this `RichCheckEffect`
	isTriggeredBy(expr ast.Expr) bool

	// isInvalidatedBy indicates whether a given expression invalidates this effect
	isInvalidatedBy(node ast.Node) bool

	// effectIfTrue is the effect to insert as preprocessing in the true branch of a triggering conditional
	effectIfTrue(node *RootAssertionNode)

	// effectIfFalse is the effect to insert as preprocessing in the false branch of a triggering condition
	effectIfFalse(node *RootAssertionNode)

	// isNoop returns whether this effect is a noop (i.e. placeholder value)
	isNoop() bool

	// equals returns true iff this effect should be considered equal to another
	// correctness of these `equals` functions is vital to correctness (and termination) of the propagation
	// in `propagateRichChecks`.
	equals(RichCheckEffect) bool
}

// A FuncErrRet is a RichCheckEffect for the `err` in `r0, r1, r2, ..., err := f()`, where the
// function `f` has a final result of type `error` - and until this is checked all other results are
// assumed nilable
//
// For proper invalidation, each stored return of a function is treated as a separate effect
type FuncErrRet struct {
	root  *RootAssertionNode // an associated root node
	err   TrackableExpr      // the `error`-typed return of the function
	ret   TrackableExpr      // the return value of the function
	guard util.GuardNonce    // the guard to be applied on a matching check
}

func (f *FuncErrRet) isTriggeredBy(expr ast.Expr) bool {
	return exprIsPositiveNilCheck(f.root, expr, f.err)
}

func (f *FuncErrRet) isInvalidatedBy(node ast.Node) bool {
	return nodeAssignsOneWithoutOther(f.root, node, f.err, f.ret)
}

func (f *FuncErrRet) effectIfTrue(node *RootAssertionNode) {
	guardExpr(node, f.ret, f.guard)
}

func (f *FuncErrRet) effectIfFalse(*RootAssertionNode) {
	// no-nop
}

func (f *FuncErrRet) isNoop() bool { return false }

func (f *FuncErrRet) equals(effect RichCheckEffect) bool {
	otherFuncErrRet, ok := effect.(*FuncErrRet)
	if !ok {
		return false
	}
	return f.root.Equal(f.err, otherFuncErrRet.err) &&
		f.root.Equal(f.ret, otherFuncErrRet.ret) &&
		f.guard == otherFuncErrRet.guard
}

// okRead provides a general implementation for the special return form: `v1, v2, ..., ok := expr`.
// Concrete examples of patterns supported are:
// - map ok read: `v, ok := m[k]`
// - channel ok receive: `v, ok := <-ch`
// - function ok return: `r0, r1, r2, ..., ok := f()`
type okRead struct {
	root  *RootAssertionNode // an associated root node
	value TrackableExpr      // `value` could be a value for read from a map or channel, or the return value of a function
	ok    TrackableExpr      // `ok` is boolean "ok" for read from a map or channel, or return from a function
	guard util.GuardNonce    // the guard to be applied on a matching check
}

func (r *okRead) isTriggeredBy(expr ast.Expr) bool {
	return exprMatchesTrackableExpr(r.root, expr, r.ok)
}

func (r *okRead) isInvalidatedBy(node ast.Node) bool {
	return nodeAssignsOneWithoutOther(r.root, node, r.ok, r.value)
}

func (r *okRead) effectIfTrue(node *RootAssertionNode) {
	guardExpr(node, r.value, r.guard)
}

func (r *okRead) effectIfFalse(*RootAssertionNode) {
	// no-op
}

func (*okRead) isNoop() bool { return false }

func (r *okRead) equals(effect RichCheckEffect) bool {
	other, ok := effect.(*okRead)
	if !ok {
		return false
	}
	return r.root.Equal(r.value, other.value) && r.root.Equal(r.ok, other.ok) && r.guard == other.guard
}

// A MapOkRead is a RichCheckEffect for the `ok` in `v, ok := m[k]` assignment. To match such an assignment,
// both the `v` and the `ok` must be identifiers, and to have the intended effect, an `if ok { }` must
// be encountered before an assignment to either `v` or `ok`.
//
// Possible future extensions to the robustness of this effect would be to track the flow of `v` and `ok`
// instead of just giving up when flow (i.e. assignment) occurs, and to expand the allowed language of
// `v` and `ok` from identifiers to trackable expressions.
type MapOkRead struct {
	okRead
}

// A MapOkReadRefl indicates that a map was read in a `v, ok := m[k]` assignment, and now
// if `ok` is checked it should produce non-nil for `m` because it cannot be nil if `ok` is true.
type MapOkReadRefl struct {
	okRead
}

// A ChannelOkRecv is a RichCheckEffect for the `ok` in `v, ok := <-chan` assignment. To match such an assignment,
// both the `v` and the `ok` must be identifiers, and to have the intended effect, an `if ok { }` must
// be encountered before an assignment to either `v` or `ok`.
//
// Possible future extensions to the robustness of this effect would be to track the flow of `v` and `ok`
// instead of just giving up when flow (i.e. assignment) occurs, and to expand the allowed language of
// `v` and `ok` from identifiers to trackable expressions.
type ChannelOkRecv struct {
	okRead
}

// A ChannelOkRecvRefl indicates that a channel receive was encountered with a `v, ok := <-chan` assignment, and now
// if `ok` is checked it should produce non-nil for `chan` because it cannot be nil if `ok` is true.
type ChannelOkRecvRefl struct {
	okRead
}

// A FuncOkReturn is a RichCheckEffect for the `ok` in `r0, r1, r2, ..., ok := f()`, where the
// function `f` has a final result of type `bool` - and until this is checked all other results are
// assumed nilable. For proper invalidation, each stored return of a function is treated as a separate effect
type FuncOkReturn struct {
	okRead
}

// A RichCheckNoop is a placeholder instance of RichCheckEffect that functions as a total noop.
// It is used to allow in place modification of collections of RichCheckEffects.
type RichCheckNoop struct{}

func (RichCheckNoop) isTriggeredBy(ast.Expr) bool { return false }

func (RichCheckNoop) isInvalidatedBy(ast.Node) bool { return false }

func (RichCheckNoop) effectIfTrue(*RootAssertionNode) {}

func (RichCheckNoop) effectIfFalse(*RootAssertionNode) {}

func (RichCheckNoop) isNoop() bool { return true }

func (RichCheckNoop) equals(effect RichCheckEffect) bool {
	_, isNoop := effect.(RichCheckNoop)
	return isNoop
}

// RichCheckFromNode analyzes the passed `ast.Node` to see if it generates a rich check effect.
// If it does, that effect is returned along with the boolean true
// If it does not, then `nil, false` is returned.
func RichCheckFromNode(rootNode *RootAssertionNode, nonceGenerator *util.GuardNonceGenerator, node ast.Node) ([]RichCheckEffect, bool) {
	var effects []RichCheckEffect
	someEffects := false
	if okReadEffects, ok := NodeTriggersOkRead(rootNode, nonceGenerator, node); ok {
		effects, someEffects = append(effects, okReadEffects...), true
	}
	if funcEffects, ok := NodeTriggersFuncErrRet(rootNode, nonceGenerator, node); ok {
		effects, someEffects = append(effects, funcEffects...), true
	}
	return effects, someEffects
}

// parseExpr wraps a call to ParseExprAsProducer with two additional bits of useful handling:
//  1. check for the empty expression and return nil when passed it
//  2. if parsing fails with a panic, return nil (This can happen because handling for the sake of contracts
//     is less refined than handling in the more general propagation. For example, unlike other code paths,
//     here we don't check for library identifiers which cannot be found in the set of sources for this
//     analysis pass before we call ParseExprAsProducer below)
func parseExpr(rootNode *RootAssertionNode, expr ast.Expr) TrackableExpr {
	defer func() {
		// This handles unexpected panics during parsing.
		// TODO: consider removing this hack.
		_ = recover()
	}()
	// this handles being passed the empty expression
	if util.IsEmptyExpr(expr) {
		return nil
	}
	parsed, _ := rootNode.ParseExprAsProducer(expr, false)
	return parsed
}

// NodeTriggersOkRead is a case of a node creating a rich bool effect for map reads, channel receives, and user-defined
// functions in the "ok" form. Specifically, it matches on `AssignStmt`s of the form
// - `v, ok := mp[k]`
// - `v, ok := <-ch`
// - `r0, r1, r2, ..., ok := f()`
func NodeTriggersOkRead(rootNode *RootAssertionNode, nonceGenerator *util.GuardNonceGenerator, node ast.Node) ([]RichCheckEffect, bool) {
	lhs, rhs := asthelper.ExtractLHSRHS(node)
	if len(lhs) < 2 || len(rhs) != 1 {
		return nil, false
	}

	okExpr := lhs[len(lhs)-1]
	lhsOkParsed := parseExpr(rootNode, okExpr)
	if lhsOkParsed == nil {
		// here, the lhs `ok` operand is not trackable so there are no rich effects
		return nil, false
	}

	var effects []RichCheckEffect

	switch rhs := rhs[0].(type) {
	case *ast.IndexExpr:
		// this is the case of `v, ok := mp[k]`. Early return if the lhs is not a map read of the expected format
		if len(lhs) != 2 {
			return nil, false
		}

		rhsXType := rootNode.Pass().TypesInfo.Types[rhs.X].Type
		if util.TypeIsDeeplyMap(rhsXType) {
			// Create a rich check effect for `v` part of the map read in `v, ok := mp[k]`
			if lhsValueParsed := parseExpr(rootNode, lhs[0]); lhsValueParsed != nil {
				// Here, the lhs `value` operand is trackable
				effects = append(effects, &MapOkRead{
					okRead{
						root:  rootNode,
						value: lhsValueParsed,
						ok:    lhsOkParsed,
						guard: nonceGenerator.Next(lhs[0]),
					}})
			}

			// Create a rich check effect for the map read `mp[k]` part of `v, ok := mp[k]`. This is important
			// to support cases when consequent map reads are used instead of creating a local variable `v`. For example,
			// ```
			// if _, ok := mp[k]; ok {
			//	  return *mp[k]
			// }
			// ```
			if rhsParsed := parseExpr(rootNode, rhs); rhsParsed != nil {
				// Here, the rhs `map read` itself is trackable
				effects = append(effects, &MapOkRead{
					okRead{
						root:  rootNode,
						value: rhsParsed,
						ok:    lhsOkParsed,
						guard: nonceGenerator.Next(rhs),
					}})
			}

			// Create a rich check effect for the map itself, `mp`, in `v, ok := mp[k]`
			if rhsMapParsed := parseExpr(rootNode, rhs.X); rhsMapParsed != nil {
				// Here, the rhs `map` operand is trackable
				effects = append(effects, &MapOkReadRefl{
					okRead{
						root:  rootNode,
						value: rhsMapParsed,
						ok:    lhsOkParsed,
						guard: nonceGenerator.Next(rhs.X),
					}})
			}
		}
	case *ast.UnaryExpr:
		// this is the case of `v, ok := <-ch`. Early return if the lhs is not a channel receive of the expected format
		if len(lhs) != 2 {
			return nil, false
		}

		rhsXType := rootNode.Pass().TypesInfo.Types[rhs.X].Type
		if rhs.Op == token.ARROW && util.TypeIsDeeplyChan(rhsXType) {
			lhsValueParsed := parseExpr(rootNode, lhs[0])
			if lhsValueParsed != nil {
				// here, the lhs `value` operand is trackable
				effects = append(effects, &ChannelOkRecv{
					okRead{
						root:  rootNode,
						value: lhsValueParsed,
						ok:    lhsOkParsed,
						guard: nonceGenerator.Next(lhs[0]),
					}})
			}

			if rhsChanParsed := parseExpr(rootNode, rhs.X); rhsChanParsed != nil {
				// here, the rhs `channel` operand is trackable
				effects = append(effects, &ChannelOkRecvRefl{
					okRead{
						root:  rootNode,
						value: rhsChanParsed,
						ok:    lhsOkParsed,
						guard: nonceGenerator.Next(rhs.X),
					}})
			}
		}
	case *ast.CallExpr:
		callIdent := util.FuncIdentFromCallExpr(rhs)
		if callIdent == nil {
			// this discards the case of an anonymous function
			// perhaps in the future we could change this
			return nil, false
		}

		rhsFuncDecl, ok := rootNode.ObjectOf(callIdent).(*types.Func)

		if !ok || !util.FuncIsOkReturning(rhsFuncDecl) {
			return nil, false
		}

		// we've found an assignment of vars to an "ok" form function!
		for i := 0; i < len(lhs)-1; i++ {
			lhsExpr := lhs[i]
			lhsValueParsed := parseExpr(rootNode, lhsExpr)
			if lhsValueParsed == nil || util.ExprBarsNilness(rootNode.Pass(), lhsExpr) {
				// ignore assignments to any variables whose type bars nilness, such as 'int'
				continue
			}
			// here, the lhs `value` operand is trackable
			effects = append(effects, &FuncOkReturn{
				okRead{
					root:  rootNode,
					value: lhsValueParsed,
					ok:    lhsOkParsed,
					guard: nonceGenerator.Next(lhs[i]),
				}})
		}
	}
	if len(effects) > 0 {
		return effects, true
	}
	return nil, false
}

// NodeTriggersFuncErrRet is a case of a node creating a rich check effect.
// it matches on calls to functions with error-returning types
func NodeTriggersFuncErrRet(rootNode *RootAssertionNode, nonceGenerator *util.GuardNonceGenerator, node ast.Node) ([]RichCheckEffect, bool) {
	lhs, rhs := asthelper.ExtractLHSRHS(node)

	if len(lhs) == 0 || len(rhs) != 1 {
		return nil, false
	}

	callExpr, ok := rhs[0].(*ast.CallExpr)

	if !ok {
		// rhs is not a function call
		return nil, false
	}

	callIdent := util.FuncIdentFromCallExpr(callExpr)

	if callIdent == nil {
		// this discards the case of an anonymous function
		// perhaps in the future we could change this
		return nil, false
	}

	rhsFuncDecl, ok := rootNode.Pass().TypesInfo.ObjectOf(callIdent).(*types.Func)

	if !ok || !util.FuncIsErrReturning(rhsFuncDecl) {
		return nil, false
	}

	// we've found an assignment of vars to an error-returning function!

	results := rhsFuncDecl.Type().(*types.Signature).Results()
	n := results.Len()
	if len(lhs) != n {
		panic(fmt.Sprintf("ERROR: AssignStmt found with %d operands on left, "+
			"and a %d-returning function on right", len(lhs), n))
	}

	errExpr := lhs[n-1]
	errExprParsed := parseExpr(rootNode, errExpr)

	if errExprParsed == nil {
		// here, unfortunately, the error return is not trackable so there are no RichCheckEffects
		return nil, false
	}

	var effects []RichCheckEffect
	someEffect := false

	for i := 0; i < n-1; i++ {
		lhsExpr := lhs[i]
		lhsExprParsed := parseExpr(rootNode, lhsExpr)

		if lhsExprParsed == nil || util.ExprBarsNilness(rootNode.Pass(), lhsExpr) {
			// for now, we ignore assignments into anything but local variables
			// we also ignore assignments to any variables whose type bars nilness, such as 'int'
			continue
		}

		// we've found a valid place that an error variable indicates the safety of
		// nilability annotations on a return variable, so instantiate a new RichCheckEffect!
		effects, someEffect = append(effects, &FuncErrRet{
			root:  rootNode,
			err:   errExprParsed,
			ret:   lhsExprParsed,
			guard: nonceGenerator.Next(lhsExpr),
		}), true
	}

	return effects, someEffect
}

// nodeIsAssignmentTo(pass, node, one, other) returns true if `node` is an assignment to the variable
// `one` but not an assignment to the variable `other`
func nodeAssignsOneWithoutOther(rootNode *RootAssertionNode, node ast.Node, one, other TrackableExpr) bool {
	var assignsOne, assignsOther bool
	if assignStmt, ok := node.(*ast.AssignStmt); ok {
		for _, assignedVal := range assignStmt.Lhs {
			parsedLHSExpr := parseExpr(rootNode, assignedVal)
			if parsedLHSExpr != nil {
				if !assignsOne && rootNode.Equal(parsedLHSExpr, one) {
					assignsOne = true
				}
				if !assignsOther && rootNode.Equal(parsedLHSExpr, other) {
					assignsOther = true
				}
			}
		}
	}
	return assignsOne && !assignsOther
}

// exprIsPositiveNilCheck checks if an expression `expr` is of the form `checksVar == nil` for some
// variable `checksVar`. Note that because of preprocessing done in `restructureBlock` from
// `preprocess_blocks.go`, this suffices to handle cases such as `nil != checksVar` as well.
func exprIsPositiveNilCheck(rootNode *RootAssertionNode, expr ast.Expr, checksExpr TrackableExpr) bool {
	if binExpr, ok := expr.(*ast.BinaryExpr); ok && binExpr.Op == token.EQL && util.IsLiteral(binExpr.Y, "nil") {
		return exprMatchesTrackableExpr(rootNode, binExpr.X, checksExpr)
	}
	return false
}

// exprMatchesTrackableExpr checks if an expression `expr` is equivalent to the passed TrackableExpr `checks`
func exprMatchesTrackableExpr(rootNode *RootAssertionNode, expr ast.Expr, checks TrackableExpr) bool {
	parsedExpr := parseExpr(rootNode, expr)
	return parsedExpr != nil && rootNode.Equal(parsedExpr, checks)
}

// guardExpr marks all the consume triggers in the var assertion node corresponding to the passed
// variable (if such a node exists) as guarded by the passed GuardNonce
func guardExpr(rootNode *RootAssertionNode, expr TrackableExpr, guard util.GuardNonce) {
	lookedUpNode, _ := rootNode.lookupPath(expr)
	if lookedUpNode != nil {
		// The passed expression is tracked, so mark its corresponding node as guarded
		lookedUpNode.SetConsumeTriggers(
			annotation.ConsumeTriggerSliceAsGuarded(
				lookedUpNode.ConsumeTriggers(), guard))
	}
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

		checkCFGFixedPointRuntime("RichCheckEffect Forwards Propagation", roundCount, n)
	}

	// this strips duplicates from the RichCheckEffect slices
	for i := range currBlocks {
		currBlocks[i] = mergeSlices(true, currBlocks[i])
	}

	return currBlocks
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
