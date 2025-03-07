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

// Package assertiontree contains the node definitions for the assertion tree, as well as the main
// backpropagation algorithm.
package assertiontree

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion/function/preprocess"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"go.uber.org/nilaway/util/typeshelper"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/cfg"
)

// backpropAcrossBlock iterates over all nodes in the CFG block in _reverse_ order, writes logs,
// and delegates the handling of each node to backpropAcrossNode.
func backpropAcrossBlock(rootNode *RootAssertionNode, block *cfg.Block) error {
	// Iterate over all blocks in _reverse_ order
	for i := len(block.Nodes) - 1; i >= 0; i-- {
		node := block.Nodes[i]

		err := backpropAcrossNode(rootNode, node)
		if err != nil {
			pos := rootNode.Pass().Fset.Position(node.Pos())
			// If any error occurs when back-propagating a node, we wrap the error with more
			// information such as file name and positions for easier debugging.
			return fmt.Errorf(
				"backpropagation across node (%s:%d:%d) of type %T failed for reason: %w",
				pos.Filename, pos.Line, pos.Column, node, err,
			)
		}
	}

	return nil
}

// backpropAcrossNode is the main driver function for the backpropagation of each node of
// different types. For some complicated cases, it further delegates the handling to other
// finer-grained backpropX functions for better code clarity.
func backpropAcrossNode(rootNode *RootAssertionNode, node ast.Node) error {
	switch n := node.(type) {
	case *ast.ParenExpr:
		return backpropAcrossNode(rootNode, n.X)
	case *ast.ReturnStmt:
		return backpropAcrossReturn(rootNode, n)
	case *ast.AssignStmt:
		return backpropAcrossAssignment(rootNode, n.Lhs, n.Rhs)
	case *ast.ValueSpec:
		// These nodes represent declarations such as `var x, y : int = 4, 3`
		if len(n.Names) > 0 && len(n.Values) > 0 {
			err := backpropAcrossAssignment(rootNode, toExprSlice(n.Names), n.Values)
			if err != nil {
				return err
			}
		}
	case *ast.SendStmt:
		return backpropAcrossSend(rootNode, n)
	case *ast.ExprStmt:
		rootNode.AddComputation(n.X)
	case *ast.GoStmt:
		rootNode.AddComputation(n.Call)
	case *ast.IncDecStmt:
		rootNode.AddComputation(n.X)

	case *ast.SelectorExpr:
		rootNode.AddComputation(n)
	case *ast.BinaryExpr:
		rootNode.AddComputation(n)
	case *ast.CallExpr:
		rootNode.AddComputation(n)
	case *ast.UnaryExpr:
		rootNode.AddComputation(n)
	case *ast.StarExpr:
		rootNode.AddComputation(n)
	case *ast.IndexExpr:
		rootNode.AddComputation(n)
	case *ast.SliceExpr:
		rootNode.AddComputation(n)
	case *ast.TypeAssertExpr:
		rootNode.AddComputation(n)
	case *ast.CompositeLit:
		for _, expr := range n.Elts {
			rootNode.AddComputation(expr)
		}
	// The following cases are not interesting to our nilness analysis, or are currently
	// unsupported, so we do nothing for them.
	case *ast.BasicLit, *ast.Ident, *ast.EmptyStmt, *ast.DeferStmt:
		// TODO: figure out what source code generates these cases - it's not obvious
		// TODO: handle defers
	default:
		return fmt.Errorf("unrecognized AST node %T in CFG - add a case for it", n)
	}

	return nil
}

// backpropAcrossSend handles backpropagation for send statements. It is designed to be called from
// backpropAcrossNode as a special handler.
func backpropAcrossSend(rootNode *RootAssertionNode, node *ast.SendStmt) error {
	// Note that for channel sends, we have:
	// (1) A send to a nil channel blocks forever;
	// (2) A send to a closed channel panics.
	// (1) falls out of scope for NilAway and hence we do not create a consumer here for the
	// channel variable. For (2), since we do not track the state of the channels, we currently
	// cannot support it.
	// TODO: rethink our strategy of handling channels (#192).
	consumer, err := exprAsAssignmentConsumer(rootNode, node, nil)
	if err != nil {
		return err
	}
	if consumer != nil {
		rootNode.AddConsumption(&annotation.ConsumeTrigger{
			Annotation: consumer,
			Expr:       node.Value,
			Guards:     util.NoGuards(),
		})
	}

	rootNode.AddComputation(node.Chan)
	rootNode.AddComputation(node.Value)

	return nil
}

// backpropAcrossReturn handles backpropagation for return statements. It is designed to be called
// from backpropAcrossNode as a special handler.
func backpropAcrossReturn(rootNode *RootAssertionNode, node *ast.ReturnStmt) error {
	// we have to handle the case that a multiply-returning function is being returned, and split
	// the productions appropriate instead of just calling computeAndConsumeResults directly in that case

	if rootNode.functionContext.functionConfig.EnableStructInitCheck {
		rootNode.addConsumptionsForFieldsOfParams()
	}

	if len(node.Results) == 1 {
		if call, ok := node.Results[0].(*ast.CallExpr); ok {
			var fident *ast.Ident

			handleIdent := func(fun *ast.Ident) (hasFuncObj bool) {
				if rootNode.isBuiltIn(fun) || rootNode.isTypeName(fun) {
					return false // definitely not multiply returning here
				}
				if !rootNode.isFunc(fun) {
					return false // this is a call to a variable with function type -
					// we could try to search these for their annotations (and should) but not yet
				}
				fident = fun
				return true
			}

			switch fun := call.Fun.(type) {
			case *ast.Ident:
				if !handleIdent(fun) {
					return computeAndConsumeResults(rootNode, node)
				}
			case *ast.SelectorExpr:
				if !handleIdent(fun.Sel) {
					return computeAndConsumeResults(rootNode, node)
				}
			default:
				// In this case - an anonymous function is called and returned, for now I don't
				// know what to do here, so we just compute
				// TODO - handle this case (and similar case in ParseExprAsProducer)
				return computeAndConsumeResults(rootNode, node)
			}
			if fident == nil {
				// Since functions are assumed to be without side effects, we don't know that
				// `fident` is actually definitely, non-nil here, tracked as
				return errors.New("fident variable is nil")
			}
			funcObj := rootNode.ObjectOf(fident).(*types.Func)
			if util.FuncNumResults(funcObj) > 1 {
				// this is the case we were looking for!
				// we've identified that a multiply-returning function is being returned

				_, producers := rootNode.ParseExprAsProducer(call, true)
				for i := 0; i < util.FuncNumResults(funcObj); i++ {
					if producers == nil {
						// this nil check reflects programmer logic
						return errors.New("producers variable is nil")
					}
					// since we don't individually track the returns of a multiply returning function,
					// we form full triggers for each return whose type doesn't bar nilness
					if !util.TypeBarsNilness(funcObj.Type().(*types.Signature).Results().At(i).Type()) {
						isErrReturning := util.FuncIsErrReturning(funcObj)
						isOkReturning := util.FuncIsOkReturning(funcObj)

						trigger := annotation.FullTrigger{
							Producer: &annotation.ProduceTrigger{
								// since the value is being returned directly, only its shallow nilability
								// matters (but deep would matter if we were enforcing correct variance)
								Annotation: producers[i].GetShallow().Annotation,
								Expr:       call,
							},
							Consumer: &annotation.ConsumeTrigger{
								Annotation: &annotation.UseAsReturn{
									TriggerIfNonNil: &annotation.TriggerIfNonNil{
										Ann: annotation.RetKeyFromRetNum(
											rootNode.ObjectOf(rootNode.FuncNameIdent()).(*types.Func),
											i,
										)},
									RetStmt: node,
								},
								Expr:   call,
								Guards: util.NoGuards(),
								// if an error returning function returns directly as the result of
								// another error returning function, then its results can safely be
								// interpreted as guarded
								GuardMatched: isErrReturning || isOkReturning,
							},
						}

						// This is a duplicate trigger for tracking "always safe" paths. The analysis of these triggers
						// will be processed at the inference stage.
						triggerAlwaysSafe := annotation.FullTrigger{
							Producer: trigger.Producer,
							Consumer: &annotation.ConsumeTrigger{
								Annotation: &annotation.UseAsReturn{
									TriggerIfNonNil: &annotation.TriggerIfNonNil{
										Ann: annotation.RetKeyFromRetNum(
											rootNode.ObjectOf(rootNode.FuncNameIdent()).(*types.Func),
											i,
										)},
									RetStmt:              node,
									IsTrackingAlwaysSafe: true,
								},
								Expr:         trigger.Consumer.Expr,
								Guards:       trigger.Consumer.Guards,
								GuardMatched: trigger.Consumer.GuardMatched,
							},
						}

						rootNode.AddNewTriggers(trigger, triggerAlwaysSafe)
					}
				}
				rootNode.AddComputation(call)
				return nil
			}
			// is a function call but not a multiply returning one - just compute!
		}
	}

	return computeAndConsumeResults(rootNode, node)
}

// backpropAcrossAssignment handles backpropagation for assignments, including special cases such
// as type switches and range statements. Moreover, it also handles special contracts such as map
// reads, channel reads, and type assertions by adding appropriate guards to them.
// Specifically, there are three phases in our process here:
// Phase 1. move any assertions on trackable LHS expressions to the RHS;
// Phase 2. mark any assignments into fields as consuming the assigned value by that fields' annotation;
// Phase 3. mark all LHS and RHS as computed.
// nonnil(lhs, rhs)
func backpropAcrossAssignment(rootNode *RootAssertionNode, lhs, rhs []ast.Expr) error {
	// For phase 1 and 2, we will first handle a few special assignments (with early return), then
	// if none of the special cases are hit, which means it is a normal assignment, we further
	// delegate the process to other functions depending on if it is many-to-one or one-to-one
	// assignment for better code clarity.
	// In all cases, we should do phase 3 before returning, so here we defer a function for phase 3.
	defer func() {
		// Phase 3
		// Now that we've back-propagated across the assignment itself, make sure we can compute
		// all of the lhs and rhs.
		for _, rhsVal := range rhs {
			rootNode.AddComputation(rhsVal)
		}
		for _, lhsVal := range lhs {
			rootNode.AddComputation(lhsVal)
		}
	}()

	// Phase 1 and 2
	// First handle a few special cases, e.g., type switches, type assertions, range statements,
	// and some cases for "ok" contracts, all of which will have a rhs with length 1.
	if len(rhs) == 1 {
		// Here we first strip the parentheses of the rhs to reveal the underlying nodes.
		rhsNode := astutil.Unparen(rhs[0])

		// Type switch `x := y.(type)`, which needs special handling because TypesInfo.Defs
		// can't find an object for the lhs.
		// Note that the key distinction between a "type switch" and a "type assertion" in the
		// AST is whether the `Type` field of the AST node is nil.
		if r, ok := rhsNode.(*ast.TypeAssertExpr); ok && r.Type == nil {
			// lhs must have one element, which is *ast.Ident.
			if len(lhs) != 1 {
				return errors.New("lhs must have one element for type switches")
			}
			lhsIdent := lhs[0].(*ast.Ident)
			return backpropAcrossTypeSwitch(rootNode, lhsIdent, r.X)
		}

		// Range statement of the form `for x := range y`, which is not overly complex to
		// handle but does involve distinct semantics.
		if r, ok := rhsNode.(*ast.UnaryExpr); ok && r.Op == token.RANGE {
			return backpropAcrossRange(rootNode, lhs, r.X)
		}

		// Now we handle special cases for "ok" contracts, the lhs must have length of 2, the first
		// being the processed variable and second being the `ok` boolean. Specifically, we
		// currently handle the following cases in NilAway:
		// 1. Map read: `v, ok := m[k]`
		// 2. Channel receive: `v, ok := <-ch`
		// TODO: 3. Type assertion: `v, ok := y.(*type)`
		if len(lhs) == 2 {
			rootNode.AddGuardMatch(lhs[0], ContinueTracking)

			// Map read
			if r, ok := rhsNode.(*ast.IndexExpr); ok {
				rootNode.AddGuardMatch(r, ContinueTracking)
				rootNode.AddGuardMatch(r.X, ProduceAsNonnil)
				return backpropAcrossOneToOneAssignment(rootNode, lhs[0:1], rhs)
			}

			// Channel read
			// There is a slight difference in the handling of map reads and channel receives
			// that is driven by Go's behavior. Reading from a nil map returns `nil`, but
			// reading from a nil channel gives a deadlock error. Hence, for maps guarding is
			// essentially required if the map is determined to be nilable, however, for a
			// channel guarding logic is enforced only if it is in an `ok` form. That is why
			// the NeedsGuard of ChanRecv is set to false in all other cases, but this one,
			// where we know that it is an `ok` receive case.
			if r, ok := rhsNode.(*ast.UnaryExpr); ok && r.Op == token.ARROW {
				rootNode.AddGuardMatch(r.X, ProduceAsNonnil)
				// Add produce trigger for channel receive on the expression `v` here itself,
				// since we want to set guarding = true.
				if !util.IsEmptyExpr(lhs[0]) {
					producer := exprAsDeepProducer(rootNode, r.X)
					producer.SetNeedsGuard(true)

					rootNode.AddProduction(&annotation.ProduceTrigger{
						// set the guard on channel receive since it is an ok form
						Annotation: producer,
						Expr:       lhs[0],
					})
				}
				// We do not need to "backpropAcrossOneToOneAssignment" since we explicitly
				// added a produce trigger above.
				return nil
			}

			// Type assertion
			if r, ok := rhsNode.(*ast.TypeAssertExpr); ok && r.Type != nil {
				// TODO: properly handle type assertions' "OK" contract
				return backpropAcrossOneToOneAssignment(rootNode, lhs[0:1], rhs)
			}
		}
	}

	// If the above code did not catch any special cases, it means this assignment is a normal
	// assignment, and we further delegate the handling to other functions.
	if len(lhs) == len(rhs) {
		return backpropAcrossOneToOneAssignment(rootNode, lhs, rhs)
	}
	return backpropAcrossManyToOneAssignment(rootNode, lhs, rhs)
}

// backpropAcrossRange handles range expression (e.g., "for i, v := range lst"), it is designed to
// be called from backpropAcrossAssignment as a finer-grained handler for special assignment cases.
func backpropAcrossRange(rootNode *RootAssertionNode, lhs []ast.Expr, rhs ast.Expr) error {
	// produceAsIndex(i) marks the ith lhs expression as a range index, producing it as non-nil
	// because it necessarily has basic type (int or char)
	produceAsIndex := func(i int) {
		// if nonempty, produce the index as definitely non-nil
		if !util.IsEmptyExpr(lhs[i]) {
			rootNode.AddProduction(&annotation.ProduceTrigger{
				Annotation: &annotation.RangeIndexAssignment{ProduceTriggerNever: &annotation.ProduceTriggerNever{}},
				Expr:       lhs[i],
			})
		}
	}

	// produceAsDeepRHS(i) marks the ith lhs expression as flowing deeply from the rhs
	produceAsDeepRHS := func(i int) {
		// if nonempty, produce the ranging value from the deep nilability of the rhs
		// we can't track the rhs of ranges since we would need to discover non-nil assignments
		// to an unbounded number of indices to conclude anything other than the annotation-based
		// deep nilability of rhs
		if !util.IsEmptyExpr(lhs[i]) {
			producer := exprAsDeepProducer(rootNode, rhs)
			producer.SetNeedsGuard(false)

			rootNode.AddProduction(&annotation.ProduceTrigger{
				// we remove the guard on any deep types read from a range because reading
				// them through a range guarantees they exist, removing the need for an ok check
				Annotation: producer,
				Expr:       lhs[i],
			})
		}
	}

	// produceNonNil marks the ith lhs expression as nonnil due to limitations of NilAway.
	produceNonNil := func(i int) {
		if !util.IsEmptyExpr(lhs[i]) {
			rootNode.AddProduction(&annotation.ProduceTrigger{
				Annotation: &annotation.ProduceTriggerNever{},
				Expr:       lhs[i],
			})
		}
	}

	rhsType := types.Unalias(rootNode.Pass().TypesInfo.Types[rhs].Type)

	// Go 1.23 introduced [range-over-func] language feature, where the `range` statement can
	// now take the following types:
	//
	// 1. `func(func() bool)`
	// 2. `func(func(K) bool)`
	// 3. `func(func(K, V) bool)`
	//
	// We currently do not handle these types yet, so here we assume that they are deeply non-nil
	// (by adding nonnil producers to both K and V if given).
	//
	// Note that the `iter` package provides `iter.Seq` and `iter.Seq2` generic types for 2 and 3
	// specifically. Therefore, we need to `.Underlying()` on the rhsType to find the underlying
	// func type for simplicity.
	//
	// [range-over-func]: https://tip.golang.org/doc/go1.23
	// TODO: handle that (#287).
	if typeshelper.IsIterType(rhsType) {
		for i := range lhs {
			produceNonNil(i)
		}
		return nil
	}

	// This block breaks down the cases for the `range` statement being analyzed,
	// starting by switching on how many left-hand operands there are
	switch len(lhs) {
	case 2:
		produceAsIndex(0)          // If we have two left hand operands, the first is always int-valued
		if typeIsString(rhsType) { // This checks if we are ranging over a string
			produceAsIndex(1) // If we are ranging over a string, then the second lhs operand is also non-nil
		} else {
			produceAsDeepRHS(1) // If we are not ranging over a string, then we cannot assume basic type
		}
	case 1:
		if util.TypeIsDeeplyMap(rhsType) ||
			util.TypeIsDeeplySlice(rhsType) ||
			util.TypeIsDeeplyArray(rhsType) ||
			typeIsString(rhsType) {
			produceAsIndex(0) // If we are ranging over a map slice or string with only a single
			// lhs operand, then that operand will be int-valued
			return nil
		}
		if util.TypeIsDeeplyChan(rhsType) {
			produceAsDeepRHS(0) // iterating over a channel with only a single lhs operand will
			// still result in deeply produced lhs values
			return nil
		}

		// Here the range is over basic types, such as integers (e.g., "for i := range 10").
		// We do not need to do anything here, as the basic types are always presumed to be non-nil.
		if _, ok := rhsType.(*types.Basic); ok {
			return nil
		}

		if _, ok := rhsType.(*types.TypeParam); ok {
			// We could be ranging over a generic slice (where rhsType is a *types.TypeParam) but
			// we do not handle generics yet. Here we just assume generic slices are all deeply
			// nonnil, i.e., we return nil producers for the elements.
			// TODO: handle that.
			return nil
		}
		return fmt.Errorf("unrecognized type of rhs in range statement: %s", rhsType)
	default:
		return fmt.Errorf("unexpected LHS found in assignment to 'range' operator")
	}

	return nil
}

// backpropAcrossTypeSwitch handles type switches (e.g., "switch v := a.(*type)"), it is designed
// to be called from backpropAcrossAssignment as a finer-grained handler for special assignment
// cases. The main reason that this case has to be handled separately is that it introduces a
// "symbolic" variable to track the result of the type switch. This variable does not have a
// canonical instance of `types.Var` declaring it, in fact there is NO instance of `types.Var`
// associated with the declaration site as there usually would be through TypesInfo.Defs, and
// TypesInfo. Uses will give a fresh `types.Var` at every usage site. This is why we have to
// inspect the assertion tree for any variables that match the symbolic type switch variable
// without being able to compare the identity of `types.Var` instances as we usually do.
// nonnil(lhs, rhs)
func backpropAcrossTypeSwitch(rootNode *RootAssertionNode, lhs *ast.Ident, rhs ast.Expr) error {
	// First, make a copy of the children array to iterate over, as we will mutate it.
	children := slices.Clone(rootNode.Children())

	// For each variable in the assertion tree, check if it's equal to the symbolic variable
	// being instantiated by this type switch, and, if so, assign to it.
	// we (annonyingly) have to handle this as a separate case and continue after the first child
	// is found because there is no canonical instance of types.Object for symbolic type switch
	// variables
	for _, child := range children {
		if varChild, ok := child.(*varAssertionNode); ok {
			if varChild.decl.Pos() == lhs.Pos() &&
				varChild.decl.Name() == lhs.Name {
				// even though we can't do the matching through a types.Object instance, we conclude
				// that the *types.Var in the assertion nodes matches the lhs of this assignment

				liftedChild, _ := rootNode.LiftFromPath([]AssertionNode{varChild})
				if liftedChild == nil {
					// this nil check reflects programmer logic
					return errors.New("liftedChild variable is nil")
				}
				rhsPath, rhsProducers := rootNode.ParseExprAsProducer(rhs, false)
				if rhsPath != nil {
					// rhs is trackable, so move assertions as we would in the vanilla assignment case
					rootNode.LandAtPath(rhsPath, liftedChild)
					// assignment complete
				} else {
					liftedChild.SetParent(rootNode)
					switch len(rhsProducers) {
					case 0:
						// lhsVal expression will never be nil here because rhsVal will never be nil
						rootNode.triggerProductions(liftedChild, &annotation.ProduceTrigger{
							Annotation: &annotation.ProduceTriggerNever{},
							Expr:       lhs,
						})
					case 1:
						rootNode.triggerProductions(liftedChild, &annotation.ProduceTrigger{
							Annotation: rhsProducers[0].GetShallow().Annotation,
							Expr:       lhs,
						}, rhsProducers[0].GetDeepSlice()...)
					default:
						return errors.New("expression e in a e.(type) switch was multiply returning - " +
							"this should be a type error")
					}
				}
			}
		}
	}
	// no current assertion matches the type switch lhs variable here, so it's a no-op
	return nil
}

// backpropAcrossOneToOneAssignment handles normal one-to-one assignment (e.g, "var a *int = b", or
// "var a, b, c *int = d, e, f"), it is designed to be called from backpropAcrossAssignment as a
// finer-grained handler for one-to-one normal assignments.
// nonnil(lhs, rhs)
func backpropAcrossOneToOneAssignment(rootNode *RootAssertionNode, lhs, rhs []ast.Expr) error {
	n := len(lhs)

	// precompute the parses of all LHS expressions - we'll need them
	parsedLHS := make([][]AssertionNode, n)
	for i := range lhs {
		var seq []AssertionNode
		if !util.IsEmptyExpr(lhs[i]) {
			seq, _ = rootNode.ParseExprAsProducer(lhs[i], false)
		}
		parsedLHS[i] = seq
	}
	// Phase 1

	// We now have n expressions on the RHS and n on the LHS
	// For each of these n pairs, we have 3 cases:
	// A) LHS is not trackable - nothing to be done in Phase 1
	// B) LHS is trackable but RHS is not - mark LHS as produced by RHS value
	// C) LHS and RHS are both trackable - move assertions from LHS to RHS
	//
	// Notably, the two phases of 3 - remove assertions from LHS with LiftFromPath and
	// add assertions to RHS with LandAtPat - must be done in parallel so that swaps behave
	// correctly, e.g.: x, y = y, x (see the test multipleassignment.go)

	// Before we can even start though, we have to deal with another problem: shadowing.
	// If x.f is non-nil, then after either of the following assignments y.f will be non-nil:
	//
	// y.f, y = nil, x
	// y, y.f = x, nil
	//
	// This is because the assignment y.f = nil is always to the *old* y, whereas the other
	// assignment y = x makes sure that subsequently we touch the *new* y which is x
	//
	// To handle this, we have to do a preliminary pass to figure out which expressions will
	// be assigned to over the course of this entire multiple assignment, and not propagate
	// assertions for any member assignments that will be shadowed. The two ways we conclude
	// that a member assignment M to E will be shadowed are:
	// i) there exists another member assignment to a strict prefix expression of E
	// ii) there exists another member assignment to E to the right of M
	// We test for both cases, putting the results in the array `shadowMask` below -
	// which is true at index i iff member assignment i is shadowed.

	// TODO: compute this more efficiently using a tree
	shadowMask := make([]bool, n)
buildShadowMask:
	for i := range lhs {
		for j := 0; j < i; j++ {
			if rootNode.IsStrictPrefix(parsedLHS[j], parsedLHS[i]) {
				shadowMask[i] = true
				continue buildShadowMask
			}
		}
		for j := i + 1; j < n; j++ {
			if rootNode.IsPrefix(parsedLHS[j], parsedLHS[i]) {
				shadowMask[i] = true
				continue buildShadowMask
			}
		}
	}

	// This struct is declared to assist with deferring the second phase of the assignments
	type deferredLanding struct {
		lhsNode AssertionNode
		rhs     []AssertionNode
	}

	// To process all case C's in parallel, we accumulate a list of the first phases,
	// the "lifts", and where they will land once all accumulated
	landings := make([]deferredLanding, 0, n)

	for i := range lhs {
		if !shadowMask[i] {
			lhsVal, rhsVal := lhs[i], rhs[i]

			// Split cases A, B, C from above
			lpath := parsedLHS[i]
			if lpath != nil { // If lpath == nil we're in case A so we do nothing
				rpath, rproducers := rootNode.ParseExprAsProducer(rhsVal, false)
				if rpath != nil {
					// Both lhsVal and rhsVal are trackable! we're in case C

					if rootNode.functionContext.functionConfig.EnableStructInitCheck {
						// If rhs is a function call that is tracked then we just add field producers before detaching
						// the assertion nodes
						_, rproducers := rootNode.ParseExprAsProducer(rhsVal, true)

						if len(rproducers) != 0 {
							// Length of rproducers must be 1 since assignment is one-one
							fieldProducers := rproducers[0].GetFieldProducers()
							rootNode.addProductionsForAssignmentFields(fieldProducers, lhsVal)
						}
					}

					lhsNode, ok := rootNode.LiftFromPath(lpath)
					// TODO: below check for `lhsNode != nil` should not be needed when NilAway supports Ok form for
					//  used-defined functions (tracked issue #77)
					if ok && lhsNode != nil {
						// Add assignment entries to the consumers of lhsNode for informative printing of errors
						for _, c := range lhsNode.ConsumeTriggers() {
							err := addAssignmentToConsumer(lhsVal, rhsVal, rootNode.Pass(), c.Annotation)
							if err != nil {
								return err
							}
						}

						// If the lhsVal path is not only trackable but tracked, we add it as
						// a deferred landing
						landings = append(landings, deferredLanding{
							lhsNode: lhsNode,
							rhs:     rpath,
						})
					}

				} else {
					// We're in case B
					switch len(rproducers) {
					case 0:
						// lhsVal expression will never be nil here because rhsVal will never be nil
						rootNode.AddProduction(&annotation.ProduceTrigger{
							Annotation: &annotation.ProduceTriggerNever{},
							Expr:       lhsVal,
						})
					case 1:
						if rootNode.functionContext.functionConfig.EnableStructInitCheck {
							fieldProducers := rproducers[0].GetFieldProducers()
							rootNode.addProductionsForAssignmentFields(fieldProducers, lhsVal)
						}

						// beforeTriggersLastIndex is used to find the newly added triggers on the next line
						beforeTriggersLastIndex := len(rootNode.triggers)

						rootNode.AddProduction(&annotation.ProduceTrigger{
							Annotation: rproducers[0].GetShallow().Annotation,
							Expr:       lhsVal,
						}, rproducers[0].GetDeepSlice()...)

						// Update consumers of newly added triggers with assignment entries for informative printing of errors
						// TODO: the below check `len(rootNode.triggers) == 0` should not be needed, however, it is added to
						//  satisfy NilAway's analysis
						if len(rootNode.triggers) == 0 {
							continue
						}
						for _, t := range rootNode.triggers[beforeTriggersLastIndex:len(rootNode.triggers)] {
							err := addAssignmentToConsumer(lhsVal, rhsVal, rootNode.Pass(), t.Consumer.Annotation)
							if err != nil {
								return err
							}
						}
					default:
						return errors.New("rhs expression in a 1-1 assignment was multiply returning - " +
							"this certainly indicates an error in control flow")
					}
				}
			}
		}
	}

	// Now we actually land each of the nodes lifted above, this guarantees parallelism
	for _, landing := range landings {
		rootNode.LandAtPath(landing.rhs, landing.lhsNode)
	}

	// Phase 2
	for i := range rhs {
		lhsVal, rhsVal := lhs[i], rhs[i]
		// Check whether a consumption trigger needs to be added for a field assignment here
		consumeTrigger, err := exprAsAssignmentConsumer(rootNode, lhsVal, rhsVal)
		if err != nil {
			return err
		}
		if consumeTrigger != nil {
			rootNode.AddConsumption(&annotation.ConsumeTrigger{
				Annotation: consumeTrigger,
				Expr:       rhsVal,
				Guards:     util.NoGuards(),
			})
		}
		if consumer := exprAsConsumedByAssignment(rootNode, lhsVal); consumer != nil {
			rootNode.AddConsumption(consumer)
		}
	}

	return nil
}

// backpropAcrossManyToOneAssignment handles normal many-to-one assignment (e.g, "a, b := foo()"),
// it is designed to be called from backpropAcrossAssignment as a finer-grained handler for
// many-to-one normal assignments.
// nonnil(lhs, rhs)
func backpropAcrossManyToOneAssignment(rootNode *RootAssertionNode, lhs, rhs []ast.Expr) error {
	// Single rhsVal value assigned to multiple lhs values - the only option here is that it is
	// a function return.
	if len(rhs) != 1 {
		return errors.New("assumptions about assignment shape violated: lhs count does not equal " +
			"rhsVal count, but rhsVal count is also not 1")
	}

	rhsVal, ok := astutil.Unparen(rhs[0]).(*ast.CallExpr)
	if !ok {
		return errors.New("assumptions about assignment shape violated: lhs count does not equal " +
			"rhsVal count, but rhsVal is not a call expression")
	}
	_, producers := rootNode.ParseExprAsProducer(rhsVal, true)
	if len(producers) > 0 && len(lhs) != len(producers) {
		return errors.New("rhsVal function returned different number of results than expression " +
			"present on lhs of assignment")
	}
	for i := range producers {

		lhsVal := lhs[i]

		// Eliminates checking of the `_` instances in the lhs of a multiple assignment
		if util.IsEmptyExpr(lhsVal) {
			continue
		}

		// Phase 1
		if rootNode.functionContext.functionConfig.EnableStructInitCheck {
			fieldProducers := producers[i].GetFieldProducers()
			rootNode.addProductionsForAssignmentFields(fieldProducers, lhsVal)
		}

		// beforeTriggersLastIndex is used to find the newly added triggers on the next line
		beforeTriggersLastIndex := len(rootNode.triggers)

		rootNode.AddGuardMatch(lhsVal, ContinueTracking)
		rootNode.AddProduction(&annotation.ProduceTrigger{
			Annotation: producers[i].GetShallow().Annotation,
			Expr:       lhsVal,
		}, producers[i].GetDeepSlice()...)

		// Update consumers of newly added triggers with assignment entries for informative printing of errors
		if len(rootNode.triggers) > 0 {
			for _, t := range rootNode.triggers[beforeTriggersLastIndex:len(rootNode.triggers)] {
				err := addAssignmentToConsumer(lhsVal, rhsVal, rootNode.Pass(), t.Consumer.Annotation)
				if err != nil {
					return err
				}
			}
		}

		// Phase 2
		consumeTrigger, err := exprAsAssignmentConsumer(rootNode, lhsVal, rhsVal)
		if err != nil {
			return err
		}
		if consumeTrigger != nil {
			// Update consumeTrigger with assignment entries for informative printing of errors
			if err = addAssignmentToConsumer(lhsVal, rhsVal, rootNode.Pass(), consumeTrigger); err != nil {
				return err
			}

			// lhsVal is a field read, so this is a field assignment
			// since multiple return functions aren't trackable, this is a completed trigger
			// as long as the type of the expression being assigned doesn't bar nilness
			if !util.ExprBarsNilness(rootNode.Pass(), lhsVal) {
				rootNode.AddNewTriggers(annotation.FullTrigger{
					Producer: &annotation.ProduceTrigger{
						// We are assigning directly into the field, so we only care about shallow,
						// but we would have to check deep if we were checking dep nilability variance
						Annotation: producers[i].GetShallow().Annotation,
						Expr:       rhsVal,
					},
					Consumer: &annotation.ConsumeTrigger{
						Annotation: consumeTrigger,
						Expr:       rhsVal,
						Guards:     util.NoGuards(),
					},
				})
			}
		}

		if consumer := exprAsConsumedByAssignment(rootNode, lhsVal); consumer != nil {
			// Update consumeTrigger with assignment entries for informative printing of errors
			if err = addAssignmentToConsumer(lhsVal, rhsVal, rootNode.Pass(), consumer.Annotation); err != nil {
				return err
			}

			rootNode.AddConsumption(consumer)
		}
	}

	return nil
}

// computePostOrder computes the postorder of depth first search tree (DFST) of the live blocks of the CFG.
// The backpropagation algorithm converges faster if the CFG blocks are traversed in postorder, compared to the random
// order (Check [Kam, Ullman 76']).
func computePostOrder(blocks []*cfg.Block) []int {
	numBlocks := len(blocks)
	visited := make([]bool, numBlocks)
	postOrder := make([]int, 0, numBlocks)
	var visit func(curBlock int)
	visit = func(curBlock int) {
		visited[curBlock] = true
		for _, suc := range blocks[curBlock].Succs {
			sucIdx := int(suc.Index)
			if !visited[sucIdx] {
				visit(sucIdx)
			}
		}
		postOrder = append(postOrder, curBlock)
	}

	// Start traversal from entry block (index 0): https://pkg.go.dev/golang.org/x/tools/go/cfg#CFG
	visit(0)

	return postOrder
}

// BackpropAcrossFunc is the main driver of the backpropagation, it takes a function declaration
// with accompanying CFG, and back-propagates a tree of assertions across it to generate, at entry
// to the function, the set of assertions that must hold to avoid possible nil flow errors.
func BackpropAcrossFunc(
	ctx context.Context,
	pass *analysis.Pass,
	decl *ast.FuncDecl,
	functionContext FunctionContext,
	graph *cfg.CFG,
) ([]annotation.FullTrigger, int, int, error) {
	// We transform the CFG to have it reflect the implicit control flow that happens
	// inside short-circuiting boolean expressions.
	preprocessor := preprocess.New(pass)
	graph = preprocessor.CFG(graph, functionContext.funcDecl)

	// Generate rick check effects.
	richCheckBlocks, exprNonceMap := genInitialRichCheckEffects(graph, functionContext)
	richCheckBlocks = propagateRichChecks(graph, richCheckBlocks)
	blocks, preprocessing := blocksAndPreprocessingFromCFG(pass, graph, richCheckBlocks)

	// The assertion nodes for each block and an array of bools to indicate whether each block is
	// updated in this round or not.
	// DANGER: anytime a pointer is copied from currAssertions to nextAssertions, it MUST be
	// treated as immutable - if any modification (e.g. merging or backprop) is going to occur you
	// must perform a deep copy with CopyNode
	updatedLastRound, updatedThisRound := make([]bool, len(blocks)), make([]bool, len(blocks))
	currAssertions, nextAssertions := make([]*RootAssertionNode, len(blocks)), make([]*RootAssertionNode, len(blocks))

	// The assertion nodes for the entry block, we will use it as an indication for stabilization.
	// We consider the backpropagation stable if # of stable rounds > # of live blocks + tolerance.
	var currRootAssertionNode, nextRootAssertionNode *RootAssertionNode
	roundCount, stableRoundCount := 0, 0
	postOrder := computePostOrder(blocks)

	// Initialize the process by creating the assertion nodes for the return block.
	retBlock := len(blocks) - 1
	currAssertions[retBlock] = newRootAssertionNode(exprNonceMap, functionContext)
	updatedLastRound[retBlock] = true

	for slices.Contains(updatedLastRound, true) {
		roundCount++

		select {
		case <-ctx.Done():
			return nil, roundCount, stableRoundCount, fmt.Errorf("backprop early stop due to context: %w", ctx.Err())
		default:
		}

		for _, i := range postOrder {
			block := blocks[i]

			// There is no need to process non-live blocks; their assertions will simply be nil.
			if !block.Live {
				continue
			}

			if len(block.Succs) > 2 {
				return nil, roundCount, stableRoundCount, errors.New("assumptions about CFG shape violated - a block has >2 successors")
			}

			// No need to re-process the assertion node for the current block if it does not have
			// successors, or they were not updated in current or last round.
			hasUpdated := slices.ContainsFunc(block.Succs, func(succ *cfg.Block) bool {
				return updatedThisRound[succ.Index] || updatedLastRound[succ.Index]
			})
			if len(block.Succs) == 0 || !hasUpdated {
				// Normally we should copy the assertion node, but here it is ok to simply pass it
				// to the next round since we are not modifying it.
				nextAssertions[i] = currAssertions[i]
				continue
			}

			// Before doing actual back propagation for the current block, we need to first prepare
			// the assertion node for its successors: (1) deep copy for modifications, and (2)
			// apply preprocessor (insert nil checks) corresponding to the branch condition
			// (if it is a branch block). In the meantime, we filter out any successors if there
			// are no assertion nodes associated with it.
			succs := make([]*RootAssertionNode, 0, len(block.Succs))
			for branchIndex, succ := range block.Succs {
				var succNode *RootAssertionNode
				if nextAssertions[succ.Index] == nil && currAssertions[succ.Index] == nil {
					// No need to preprocess if there is no assertion node for the successor.
					continue
				} else if nextAssertions[succ.Index] != nil {
					// If the successor was updated this round
					// deep copy the node for modifications.
					succNode = CopyNode(nextAssertions[succ.Index]).(*RootAssertionNode)
				} else {
					// If the successor was updated last round
					// deep copy the node for modifications.
					succNode = CopyNode(currAssertions[succ.Index]).(*RootAssertionNode)
				}

				// Apply preprocessor (for branches) if there is any.
				if preprocessing[i] != nil {
					if branchIndex == 0 {
						preprocessing[i].trueBranchFunc(succNode)
					} else {
						preprocessing[i].falseBranchFunc(succNode)
					}
				}

				succs = append(succs, succNode)
			}

			// No assertion nodes attached with any successors, this should never happen since we
			// will only reach here if any of the successors were updated in the current or last round.
			if len(succs) == 0 {
				return nil, roundCount, stableRoundCount, fmt.Errorf("no assertion nodes for successors of block %d", block.Index)
			}

			// Merge the branch successors if they are both available.
			if len(succs) == 2 {
				succs[0].mergeInto(succs[0], succs[1])
			}

			// Now, the final processed node is in succs[0], we can back-propagate across it.
			nextAssertions[i] = succs[0]
			err := backpropAcrossBlock(nextAssertions[i], blocks[i])
			if err != nil {
				return nil, roundCount, stableRoundCount, err
			}

			// Monotonize updates updatedThisRound to reflect whether the assertions changed at a given index.
			if currAssertions[i] != nil {
				if !nextAssertions[i].eqNodes(nextAssertions[i], currAssertions[i]) {
					currAssertions[i].mergeInto(nextAssertions[i], currAssertions[i])
					updatedThisRound[i] = true
				}
			} else {
				updatedThisRound[i] = true
			}
		}

		// ProcessEntry is expensive, and ideally we would only call it once after the fixed point
		// of the backprop has terminated. We call it every time because waiting for all of the assertion
		// trees (i.e. the assertion tree for each block) to stabilize sometimes never occurs. As an alternative,
		// we wait for only the entry to stabilize because that's the one we care about anyways, provided its been
		// stable for at least as many rounds as there are blocks because that means the information
		// from each block has had a chance to reach entry. But waiting for the entire tree at entry block
		// to stabilize because sometimes consume triggers generated in a loop that generates them
		// on slightly different trackable expressions every time are paired with produce triggers
		// similarly generated in a loop, (see infiniteAssertions test in loopflow.go), so the assertion
		// tree will continue to grow unboundedly even though this growth produced no new full triggers
		// i.e. no new errors. To get around this, we generate the full triggers every round and
		// track stabilization of those not stabilization of the root node.
		// TODO: implement that
		if nextAssertions[0] != nil {
			nextRootAssertionNode = CopyNode(nextAssertions[0]).(*RootAssertionNode)
			nextRootAssertionNode.ProcessEntry()
		}

		if nextRootAssertionNode == nil && currRootAssertionNode == nil ||
			(nextRootAssertionNode != nil && currRootAssertionNode != nil &&
				annotation.FullTriggerSlicesEq(nextRootAssertionNode.triggers, currRootAssertionNode.triggers)) {
			stableRoundCount++
		} else {
			stableRoundCount = 0
		}

		if stableRoundCount >= config.StableRoundLimit {
			break
		}

		checkCFGFixedPointRuntime(
			fmt.Sprintf("BackpropAcrossFunc(%s) Forwards Propagation", decl.Name.Name),
			roundCount, len(blocks),
		)

		// Move variables from this round to last round and create new ones for next round.
		// For best performance, we reuse the slices by simply swapping them and clearing the
		// slices for next rounds.
		currAssertions, nextAssertions = nextAssertions, currAssertions
		clear(nextAssertions)
		updatedLastRound, updatedThisRound = updatedThisRound, updatedLastRound
		clear(updatedThisRound)
		currRootAssertionNode, nextRootAssertionNode = nextRootAssertionNode, nil
	}

	// Return the generated full triggers at the entry block; we're done!
	if currRootAssertionNode == nil {
		return nil, roundCount, stableRoundCount, nil
	}
	return currRootAssertionNode.triggers, roundCount, stableRoundCount, nil
}
