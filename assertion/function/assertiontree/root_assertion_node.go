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
	"go/constant"
	"go/token"
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// RootAssertionNode is the object that will be directly handled by the propagation algorithm,
// their only children should be VarAssertionNodes and FuncAssertionNodes
//
// the triggers field keeps track of productions and consumptions that have been directly matched
// its consumeTriggers field should be kept empty
//
// //nilable(funcObj)
type RootAssertionNode struct {
	assertionNodeCommon
	triggers []annotation.FullTrigger

	// funcObj does not have to be set. When set, it indicates the object corresponding to this function
	funcObj *types.Func

	// exprNonceMap maps expressions to nonces created to track their contracts
	exprNonceMap util.ExprNonceMap

	// functionContext holds the context of the function during backpropagation. The state includes
	// map objects that are created at initialization, and configurations that are passed through function analyzer.
	functionContext FunctionContext
}

// LocationOf returns the location of the given expression.
func (r *RootAssertionNode) LocationOf(expr ast.Expr) token.Position {
	return util.PosToLocation(expr.Pos(), r.Pass())
}

// HasContract returns if the given function has any contracts.
func (r *RootAssertionNode) HasContract(funcObj *types.Func) bool {
	_, ok := r.functionContext.funcContracts[funcObj]
	return ok
}

// MinimalString for a RootAssertionNode returns a minimal string representation of that root node
func (r *RootAssertionNode) MinimalString() string {
	return fmt.Sprintf("root<func: %s>", r.functionContext.funcDecl.Name)
}

// AddNewTriggers adds the given new triggers to the existing set of triggers of this node
func (r *RootAssertionNode) AddNewTriggers(newTrigger ...annotation.FullTrigger) {
	r.triggers = annotation.MergeFullTriggers(r.triggers, newTrigger...)
}

// FuncDecl returns the underlying function declaration of this node
func (r *RootAssertionNode) FuncDecl() *ast.FuncDecl {
	return r.functionContext.funcDecl
}

// Pass the overarching analysis pass
func (r *RootAssertionNode) Pass() *analysis.Pass {
	return r.functionContext.pass
}

// FuncNameIdent returns the function name identifier node
func (r *RootAssertionNode) FuncNameIdent() *ast.Ident {
	return r.functionContext.funcDecl.Name
}

// DefaultTrigger is not well defined for root nodes
func (r *RootAssertionNode) DefaultTrigger() annotation.ProducingAnnotationTrigger {
	panic("DefaultTrigger() not defined for RootAssertionNodes")
}

// BuildExpr is not well defined for root nodes
func (r *RootAssertionNode) BuildExpr(*analysis.Pass, ast.Expr) ast.Expr {
	panic("BuildExpr() not defined for RootAssertionNodes")
}

// Root for a RootAssertionNode is the identity function
func (r *RootAssertionNode) Root() *RootAssertionNode {
	return r
}

// Size for a RootAssertionNode also includes the full triggers
func (r *RootAssertionNode) Size() int {
	size := 1 + len(r.ConsumeTriggers()) + len(r.triggers)
	for _, child := range r.Children() {
		size += child.Size()
	}
	return size
}

// FuncObj returns the underlying function declaration of this node as a types.Func
func (r *RootAssertionNode) FuncObj() *types.Func {
	if r.funcObj == nil {
		r.funcObj = r.ObjectOf(r.FuncNameIdent()).(*types.Func)
	}
	return r.funcObj
}

// GetNonce returns the nonce associated with the passed expression, if one exists. the boolean
// return indicates whether a nonce was found
func (r *RootAssertionNode) GetNonce(expr ast.Expr) (util.GuardNonce, bool) {
	guard, ok := r.exprNonceMap[expr]
	return guard, ok
}

// GetTriggers returns the full triggers accumulated at this root node
func (r *RootAssertionNode) GetTriggers() []annotation.FullTrigger {
	return r.triggers
}

// GetDeclaringIdent finds the identifier that serves as the declaration of the passed object
func (r *RootAssertionNode) GetDeclaringIdent(obj types.Object) *ast.Ident {

	if path, ok := GetDeclaringPath(r.Pass(), obj.Pos(), obj.Pos()); ok && len(path) > 0 {
		if ident, ok := path[0].(*ast.Ident); ok && ident.Name == obj.Name() {
			return ident
		}
		// In case the declaration is package.ident
		if sel, ok := path[1].(*ast.SelectorExpr); ok {
			if sel.Sel.Name == obj.Name() {
				return sel.Sel
			}
		}
	}

	// create a fake object just to allow lookups
	fakeIdent := &ast.Ident{
		NamePos: obj.Pos(),
		Name:    obj.Name(),
		Obj:     nil,
	}

	r.functionContext.AddFakeIdent(fakeIdent, obj)
	return fakeIdent
}

// ObjectOf is the same as [types.Info.ObjectOf], but if an identifier cannot be looked up (e.g.,
// it is an artificial identifier we created to aid the analysis), we look up the internal backup
// map instead. ObjectOf returns nil if and only if both attempts fail.
func (r *RootAssertionNode) ObjectOf(ident *ast.Ident) types.Object {
	obj := r.Pass().TypesInfo.ObjectOf(ident)
	if obj != nil {
		return obj
	}
	return r.functionContext.findFakeIdent(ident)
}

// funcArgsFromCallExpr returns the set of arguments that are passed to the method at the call site. If the method
// is an anonymous function, it expands the argument set with the closure variables collected for that function
func (r *RootAssertionNode) funcArgsFromCallExpr(expr *ast.CallExpr) []ast.Expr {
	fun := expr.Fun

	if ident, ok := fun.(*ast.Ident); ok {
		// if the declaration of the ident points to a function literal node,
		// then update fun with the function literal node
		if funcLit := getFuncLitFromAssignment(ident); funcLit != nil {
			fun = funcLit
		}
	}

	switch fun := fun.(type) {
	case *ast.SelectorExpr:
		if r.isType(fun.X) {
			return expr.Args[1:]
		}
	case *ast.FuncLit:
		args := expr.Args
		if info, ok := r.functionContext.funcLitMap[fun]; ok {
			for _, closure := range info.ClosureVars {
				args = append(args, closure.Ident)
			}
			return args
		}
	}

	return expr.Args
}

// Equal returns true iff a is the same path as b
// nilable(a, b)
func (r *RootAssertionNode) Equal(a, b TrackableExpr) bool {
	return len(a) == len(b) && r.IsPrefix(a, b)
}

// IsPrefix returns true iff a is a prefix of b
func (r *RootAssertionNode) IsPrefix(a, b TrackableExpr) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(b) < len(a) {
		return false
	}
	for i := range a {
		if !r.shallowEqNodes(a[i], b[i]) {
			return false
		}
	}
	return true
}

// IsStrictPrefix returns true iff a is a prefix of b and a does not equal b
func (r *RootAssertionNode) IsStrictPrefix(a, b TrackableExpr) bool {
	return len(b) > len(a) && r.IsPrefix(a, b)
}

func newRootAssertionNode(exprNonceMap util.ExprNonceMap, functionContext FunctionContext) *RootAssertionNode {
	return &RootAssertionNode{
		exprNonceMap:    exprNonceMap,
		functionContext: functionContext,
	}
}

// using information from self (pass and funcDecl only) - turn a path into a new assertion tree starting
// at a new root. Except for that new root, all nodes are preserved so they can still be accessed as before
// the call. The new root is returned
func (r *RootAssertionNode) linkPath(path TrackableExpr) *RootAssertionNode {
	root := newRootAssertionNode(make(util.ExprNonceMap), r.functionContext)
	var currNode AssertionNode = root // use this currNode to build a linear tree to merge into r
	for _, node := range path {
		currNode.SetChildren([]AssertionNode{node})
		node.SetParent(currNode)
		currNode = node
	}
	return root
}

// AddConsumption takes the knowledge that consumer.expr will be consumed at a site characterized by the trigger
// consumer.annotation, and incorporate it into the assertion tree self
func (r *RootAssertionNode) AddConsumption(consumer *annotation.ConsumeTrigger) {

	// we check if the type of the expression `expr` prevents it from ever being nil in the first place
	if util.ExprBarsNilness(r.Pass(), consumer.Expr) {
		return // expr cannot be nil, so do nothing
	}

	path, producers := r.ParseExprAsProducer(consumer.Expr, false)
	if path == nil { // expr is not trackable
		if producers == nil {
			return // expr is not trackable, but cannot be nil, so do nothing
		}
		if len(producers) != 1 {
			panic("multiply-returning function call was passed to AddConsumption")
		}
		// expr can be nil - complete the trigger and add to root
		r.AddNewTriggers(annotation.FullTrigger{
			// we are consuming the expression directly - so only its shallow nilability counts
			Producer: producers[0].GetShallow(),
			Consumer: consumer,
		})
	} else {
		// we're adding a fresh node to the assertion tree to represent this consumption!
		newRoot := r.linkPath(path)
		path[len(path)-1].SetConsumeTriggers([]*annotation.ConsumeTrigger{consumer})
		// merge it in - increasing the set of consumeTrigges as far as the path already exists
		// in the assertion tree and extending the tree beyond that
		// TODO - possibly avoid merging in a whole new path
		// ^^^^ But I suspect gains would be marginal or non-existant - same logic either way
		r.mergeInto(r, newRoot)
	}
}

// This function takes an expression, represented as a path of AssertionNodes returned from ParseExprAsProducer,
// and searches for it in the assertion tree self
//
// if nodePtr != nil, it was found, and whichChild indicates which child it is of its parent.
// if nodePtr == nil, the expression was not trackable, or it was trackable but not present
//
// nilable(path, nodePtr)
func (r *RootAssertionNode) lookupPath(path TrackableExpr) (nodePtr AssertionNode, whichChild int) {
	if path == nil {
		// expr is not trackable - return nil
		return nil, 0
	}
	// lookup that path in r
	nodePtr = r    // this tracks our lookup
	whichChild = 0 // this tracks which child number we took to reach that lookup - useful for removing
lookup:
	for _, node := range path {
		for i, child := range nodePtr.Children() {
			if r.shallowEqNodes(node, child) {
				nodePtr = child
				whichChild = i
				continue lookup
			}
		}
		// path does not exist in r, so even though the expr is trackable no assertions we're tracking care about it
		return nil, 0
	}
	return nodePtr, whichChild

}

// AddProduction takes the knowledge that producer.expr will have a value produced by the trigger producer.annotation,
// and incorporates it into the assertion tree rootNode
func (r *RootAssertionNode) AddProduction(producer *annotation.ProduceTrigger, deeperProducer ...*annotation.ProduceTrigger) {
	path, _ := r.ParseExprAsProducer(producer.Expr, false)
	currNode, whichChild := r.lookupPath(path)
	if currNode == nil {
		return // we don't care if this expression has a value produced because it's not tracked
	}
	// If we've reached here, that means currNode points to a subtree of r matching producer.expr
	// since a value has now been produced for producer.expr, we can remove it from the assertion tree.
	// Note that it's safe to remove the entire subtree under current node, since productions to paths accessible
	// from the current expression and happening before the current line will have no effect on those paths'
	// values going forward.
	// e.g. in `x.f = nonNilVal(); x = foo(); x.f.g()` the production at `x = foo()` also has the effect of
	// invalidating the previous assignment to `x.f`.

	r.triggerProductions(currNode, producer, deeperProducer...)

	detachFromParent(currNode, whichChild)
}

// triggerProductions takes a node (assumed to be attached to its parent) and matches any of its
// consumeTriggers with the given produceTrigger, as well as matching any more deeply found consumeTriggers
// with the default non-tracked produceTriggers of their consuming expressions. Direct children of the
// node being produced also have the option to be matches with a single optionally passed `deeperProducer`,
// used for assignments by values with known deep nilness properties.
func (r *RootAssertionNode) triggerProductions(node AssertionNode, producer *annotation.ProduceTrigger, deeperProducer ...*annotation.ProduceTrigger) {

	// first we check if we were passed a deeper producer. If so, we use it to produce any \
	// indexAssertionNode children of the currNode
	if len(deeperProducer) != 0 {
		if len(deeperProducer) != 1 {
			// TODO: consider allowing multiple levels of deeper producers to be passed -
			//   but very incompatible with current annotations approach so not yet
			panic("for now - only one level of deeper producer is supported, don't pass more")
		}
		for _, child := range node.Children() {
			if child, ok := child.(*indexAssertionNode); ok {
				r.triggerProductions(child, deeperProducer[0])
			}
		}
	}
	matchConsumeTriggers := func(
		node AssertionNode,
		producer *annotation.ProduceTrigger) {
		for _, consumer := range node.ConsumeTriggers() {
			r.AddNewTriggers(annotation.FullTrigger{
				Producer: producer,
				Consumer: consumer,
			})
		}

		node.SetConsumeTriggers(nil)
	}

	// for any consumeTriggers as the indexed expr, directly match them with this produceTrigger
	matchConsumeTriggers(node, producer)

	// 	now we search for any deeper consumeTriggers in our indexed subtree, trying to match them
	// with default producers as we go. These default producers are constructed using the methods
	// BuildExpr and DefaultTrigger of assertion nodes. The former allows us to build up an expression
	// to use to symbolize the production, and the latter allows us to point out the particular
	// annotation that will yield the production of the found consumeTrigger

	var processChildren func(ast.Expr, AssertionNode)
	processChildren = func(producingSubexpr ast.Expr, node AssertionNode) {
		for _, child := range node.Children() {
			producingExpr := child.BuildExpr(r.Pass(), producingSubexpr)

			matchConsumeTriggers(child, &annotation.ProduceTrigger{
				Annotation: child.DefaultTrigger(),
				Expr:       producingExpr,
			})
			processChildren(producingExpr, child)
		}
	}

	processChildren(producer.Expr, node)
}

// GuardMatchBehavior as a type represents the set of possible effects of obtaining a guard match.
type GuardMatchBehavior = int

const (
	// ContinueTracking is a GuardMatchBehavior indicating that the field
	// GuardMatched should be set to true and the ConsumeTrigger that was matched should
	// otherwise be left in the assertion tree to flow through the function
	ContinueTracking GuardMatchBehavior = iota

	// ProduceAsNonnil is a GuardMatchBehavior indicating that the ConsumeTrigger
	// that was matched should be treated as nonnil-produced at this point, using the
	// trigger OkReadReflCheck
	ProduceAsNonnil
)

// AddGuardMatch takes an expression, and sees if that expression is mapped to a nonce
// indicating a RichCheckEffect that has been propagated from the concrete site of a check to the
// earlier site whose nilability semantics depend on that check.
// If it is mapped to a nonce, it sees if that expression is also present in the assertion
// tree with a consume trigger guarded by that nonce. This indicates that the flow we were
// looking for - for example, from `v` in `v, ok := m[k]` to `if ok {needsNonnil(v)}` - exists.
// The function takes a `GuardMatchBehavior` indicating what to do if the guard is found,
// for now, either continue tracking its expression or produce it as nonnil.
//
// To elaborate further, here is a complete rundown of the guarding mechanism.
//
// During preprocessing (preprocess_blocks.go) some statements are identified as producing a `RichCheckEffect`
// - a contract indicating that certain conditionals later in the program should have an effect on the
// semantics of that earlier statement. As an example, if `v, ok := m[k]` is encountered, then regardless
// of the deep nilability of `m`, `v` will be nilable. However, if `ok` is checked later in the program,
// it will be exactly as nilable as `m` is deeply nilable. This non-local reliance is propagated in the
// form of a RichCheckEffect that takes a GuardNonce uniquely generated corresponding to the AST node `v`
// at that site, and indicates that any time the expression `ok` is checked to be true, `v` should have
// that GuardNonce added to the set `Guards` of all of its `ConsumeTrigger`s. This indicates that those
// consumptions occur in a context "guarded" by that check. These `Guards` sets are intersected at control
// flow points (see `MergeConsumeTriggerSlices`), to ensure that the presence of a guard on a consumer
// really does indicate that it only occurs in a context in which the appropriate check has been made.
//
// This intersecting guard propagation then ensures that by the time any `ConsumeTrigger`s reach the
// statement that was dependent on the associated nonce, they will contain the information of whether
// they are properly guarded by that nonce. For example, in the below code snippet, line 1 will
// associate a nonce with `v`, to be applied when `ok` is checked. That contract will be propagated
// to the check on line 3 by a RichCheckEffect, so when backpropagation occurs across that positive branch
// of the check, it will see that `v` has two ConsumeTriggers, one generated by line 4 and one generated
// by line 7, and apply the nonce guard to both. However, on unifying the two branches, it will see that
// the ConsumeTrigger generated on line 7 is present on both sides, so it will intersect the Guards sets
// on each side and erase the nonce. Two ConsumeTriggers will then reach line 1, one from line 4 and
// one from line 7, but only the one from line 4 will have the appropriate nonce in its Guards set.
//
// ```
//
//	1 v, ok := m[k]
//	2
//	3 if ok {
//	4    consume(v)
//	5 }
//	6
//	7 consume(v)
//
// ```
//
// The role of this function, AddGuardMatch, is to look at an expression, take all the ConsumeTriggers
// for that expression in the current assertion tree, and set GuardMatched to true for them if they
// have the appropriate nonce in their Guards set. In the above example, this function would be called
// when backpropagating across line 1 with `v` for expr. The appropriate nonce would be found, and this
// function would see that it is present for 4's ConsumeTrigger but not 7's. Thus 4's would get
// GuardMatched set to true and 7's would not. If both of these ConsumeTriggers flowed to the beginning
// of the program, then they would get matched with a default ProduceTrigger as a deep read of `m`, which
// `checkGuardOnFullTrigger` would invalidate unless paired with a ConsumeTrigger with GuardMatched = true.
// GuardMatched for a ConsumeTrigger takes a conjunction over all paths from that production site to
// that ConsumeTrigger, so it is true iff the trigger has had every guard in its Guards set required
// every time it has passed through a contract-generating statement on any path.
//
// This description characterizes the `ContinueTracking` behavior. A simpler alternative, `ProduceAsNonnil`,
// indicates that if the appropriate nonce is found in a ConsumeTrigger's Guards set, the ConsumeTrigger
// should be matched immediately with a ProduceTrigger indicating nonnil production. This behavior is
// appropriate, for example, for the map itself in a read `v, ok := m[k]` - where consumptions of `m`
// guarded by a check `ok == true` are guaranteed to be produced as nonnil
func (r *RootAssertionNode) AddGuardMatch(expr ast.Expr, behavior GuardMatchBehavior) {
	guard, ok := r.GetNonce(expr)

	if !ok {
		return
	}

	exprPath, _ := r.ParseExprAsProducer(expr, false)
	currNode, _ := r.lookupPath(exprPath)
	if currNode == nil {
		return // we don't care if this expression could become guarded because it's not tracked
	}
	consumers := currNode.ConsumeTriggers()
	switch behavior {
	case ContinueTracking:
		for i, consumer := range consumers {
			if consumer.Guards.Contains(guard) && !consumer.GuardMatched {
				consumers[i] = &annotation.ConsumeTrigger{
					Annotation:   consumer.Annotation,
					Expr:         consumer.Expr,
					Guards:       consumer.Guards,
					GuardMatched: true,
				}
			}
		}
	case ProduceAsNonnil:
		var newConsumers []*annotation.ConsumeTrigger
		for _, consumer := range consumers {
			if consumer.Guards.Contains(guard) {
				r.AddNewTriggers(annotation.FullTrigger{
					Producer: &annotation.ProduceTrigger{
						Annotation: annotation.OkReadReflCheck{},
						Expr:       expr,
					},
					Consumer: consumer,
				},
				)
			} else {
				newConsumers = append(newConsumers, consumer)
			}
		}
		consumers = newConsumers
	}

	currNode.SetConsumeTriggers(consumers)
}

func (r *RootAssertionNode) consumeIndexExpr(expr ast.Expr) {
	t := r.Pass().TypesInfo.Types[expr].Type
	if util.TypeIsDeeplySlice(t) {
		r.AddConsumption(&annotation.ConsumeTrigger{
			Annotation: annotation.SliceAccess{},
			Expr:       expr,
			Guards:     util.NoGuards(),
		})
	}

	// reads of nilable maps should not necessarily produce errors - the flag config.ErrorOnNilableMapRead
	// encodes this optionality and is currently set to false
	if config.ErrorOnNilableMapRead && util.TypeIsDeeplyMap(t) {
		r.AddConsumption(&annotation.ConsumeTrigger{
			Annotation: annotation.MapAccess{},
			Expr:       expr,
			Guards:     util.NoGuards(),
		})
	}
	// there are some weird types that can show up here (*internal/abi.IntArgRegBitmap for example)
	// so don't error out if we don't recognize the type just no-op
}

// AddComputation takes the knowledge that the expression expr has to be computed to generate any necessary assertions to
// ensure that the access is safe. This will take the form of nested calls to AddConsumption
//
// basic semantics: any ast node with an ast.Expr field recurs into that field
func (r *RootAssertionNode) AddComputation(expr ast.Expr) {
	switch expr := expr.(type) {
	// we seek to recur through the AST to look for any sites at which an expression
	// must be non-nil we ignore any expressions that provide types not values since
	// assignments and branching can't happen within expressions in Go, the order in
	// which we recur doesn't matter
	case *ast.BinaryExpr:
		// process the binary expression `X op Y` in reverse, i.e., add consumers for Y first and then X
		r.AddComputation(expr.Y)

		// if the binary expr is a short-circuiting `&&`, check if the `X` part of the binary expression is a negative nil check
		// If true, add a producer right away to match with any consumer that may have appeared in the `Y` part
		// e.g., in `return x != nil && x.f == 1`, consumer trigger for the dereference `x.f` is marked safe and matched with the produce trigger created below for `x != nil`
		if expr.Op == token.LAND {
			if retExpr, retType := asNilCheckExpr(expr.X); retType == _negativeNilCheck {
				r.AddProduction(&annotation.ProduceTrigger{
					Annotation: annotation.NegativeNilCheck{},
					Expr:       retExpr,
				})
				return
			}
		}

		r.AddComputation(expr.X)
	case *ast.CallExpr:
		r.AddComputation(expr.Fun)
		exprArgs := r.funcArgsFromCallExpr(expr)
		var consumeArg func(int, ast.Expr)
		consumeArgNoop := func(int, ast.Expr) {}
		consumeArgTrigger := func(fdecl *types.Func) func(int, ast.Expr) {
			// this returns a function that adds a consume trigger for the i-th argument to
			// an annotated function call. One case we handle specially is that of a
			// multiply-returning function passed directly to a multiple param function, say
			// `foo(bar())`. In that case, we eagerly generate full triggers matching the
			// producer for the i-th result of `bar()` to a consumer for the i-th parameter
			// of `foo()`. In that case, since adding the consumer is already handled by the
			// call to `consumeArgTrigger` itself, the returned `func(i, expr)` becomes a
			// no-op. In all other cases, the function returned by `consumeArgTrigger` will
			// add a consumption on the annotation of the i-th parameter of `fdecl` and the
			// expression `expr` to the root node.
			if len(exprArgs) == 1 {
				if argFunc, ok := exprArgs[0].(*ast.CallExpr); ok {
					handleArgFuncIdent := func(argFuncIdent *ast.Ident) bool {
						if r.isFunc(argFuncIdent) {
							funcObj := r.ObjectOf(argFuncIdent).(*types.Func)
							if n := util.FuncNumResults(funcObj); n > 1 {
								// is a pass of a multiply returning function to another function
								_, producers := r.ParseExprAsProducer(argFunc, true)
								if len(producers) != n {
									panic("function number of returns differed on alternate inspections")
								}
								for i, producer := range producers {
									r.AddNewTriggers(annotation.FullTrigger{
										// the argument is consumed directly - it's deep nilability
										// doesn't matter (but it would if we were checking correct
										// variance of nilability types)
										Producer: producer.GetShallow(),
										Consumer: &annotation.ConsumeTrigger{
											Annotation: annotation.ArgPass{
												TriggerIfNonNil: annotation.TriggerIfNonNil{
													Ann: annotation.ParamKeyFromArgNum(fdecl, i),
												}},
											Expr:   argFunc,
											Guards: util.NoGuards(),
										},
									})
								}
								// we have already handled
								return true
							}
							// is a pass of a function to another function, but not multiply returning
							return false
						}
						// in this case - the identifier for the argument function did not have
						// a declaration available, so don't try to consume it
						return true
					}
					switch argFunc := argFunc.Fun.(type) {
					case *ast.Ident:
						if handleArgFuncIdent(argFunc) {
							return consumeArgNoop
						}
					case *ast.SelectorExpr:
						if handleArgFuncIdent(argFunc.Sel) {
							return consumeArgNoop
						}
					default:
						// application is anonymous - no annotations
						// TODO implement
						// unfortunately, we can't compute the appropriate consumption here
						return consumeArgNoop
					}
				}
			}
			return func(i int, arg ast.Expr) {
				if expr.Ellipsis != token.NoPos && i == len(expr.Args)-1 {
					// this is an unpacking of a variadic argument: i.e. the call `foo(_, _, a...)`
					r.AddNewTriggers(annotation.FullTrigger{
						Producer: &annotation.ProduceTrigger{
							Annotation: exprAsDeepProducer(r, arg),
							Expr:       arg,
						},
						Consumer: &annotation.ConsumeTrigger{
							Annotation: annotation.ArgPass{
								TriggerIfNonNil: annotation.TriggerIfNonNil{
									Ann: annotation.ParamKeyFromArgNum(fdecl, i),
								}},
							Expr:   arg,
							Guards: util.NoGuards(),
						},
					})
				} else {
					var paramKey annotation.Key
					if r.HasContract(fdecl) {
						// Creates a new param site with location information at every call site
						// for a function with contracts. The param site is unique at every call
						// site, even with the same function called.
						paramKey = annotation.NewCallSiteParamKey(fdecl, i, r.LocationOf(arg))
					} else {
						paramKey = annotation.ParamKeyFromArgNum(fdecl, i)
					}
					consumer := annotation.ConsumeTrigger{
						Annotation: annotation.ArgPass{
							TriggerIfNonNil: annotation.TriggerIfNonNil{
								Ann: paramKey,
							}},
						Expr:   arg,
						Guards: util.NoGuards(),
					}
					r.AddConsumption(&consumer)
				}
			}
		}

		if fun := getFuncIdent(expr, &r.functionContext); fun != nil && r.isFunc(fun) {
			// here we have found a call to a function whose declaration we have access to,
			// so we can mark its arguments as consumed
			consumeArg = consumeArgTrigger(r.ObjectOf(fun).(*types.Func))

			if r.functionContext.isDepthOneFieldCheck() {
				// Add Productions for struct field params
				r.addProductionForFuncCallArgAndReceiverFields(expr, fun)

				// Add Consumptions for struct field params
				r.addConsumptionsForArgAndReceiverFields(expr, fun)
			}
		} else {
			// here we have found either a builtin function like make or new,
			// or a typecast like int(x) - in either case (at least for now), do nothing to try
			// to consume the arguments
			consumeArg = consumeArgNoop
		}

		// when we reach this point, consumeArg will be set to a no-op exactly if we don't know
		// how to process consumption of this function's arguments (e.g. anonymous funcs) or if
		// we already have, namely through the multiple consumption case above

		for i, arg := range exprArgs {
			consumeArg(i, arg) // if arguments are to a known-annotated function, consume with its annotations
			r.AddComputation(arg)
		}
	case *ast.CompositeLit:
		for _, elt := range expr.Elts {
			r.AddComputation(elt)
		}
	case *ast.IndexExpr:
		r.consumeIndexExpr(expr.X)
		r.AddComputation(expr.X)
		r.AddComputation(expr.Index)
	case *ast.KeyValueExpr:
		r.AddComputation(expr.Key)
		r.AddComputation(expr.Value)
	case *ast.ParenExpr:
		r.AddComputation(expr.X)
	case *ast.SelectorExpr:
		// check if this is just qualifying a package:
		if id, ok := expr.X.(*ast.Ident); ok {
			if r.isPkgName(id) {
				return
			}
		}

		// A selector expression (`X.Sel`, where X is an expression and Sel is a selector) can be handled in the following two ways:
		// - (1) Allow the expression X to be nilable by creating a TriggerIfNonNil consumer for it. This is a special case,
		//       with so far the only known case being of method invocations for supporting nilable receivers. Our support
		//       is currently limited to enabling this analysis only if the below criteria is satisfied.
		//       - Check 1: selector expression is a method invocation (e.g., `s.foo()`)
		//       - Check 2: the invoked method is in scope
		//       - Check 3: the invoking expression (caller) is of struct type. (We are restricting support only for structs
		//         due to the challenges documented in .)
		//
		// - (2) Don't allow the expression X to be nilable by creating a FldAccess (ConsumeTriggerTautology) consumer for it.
		//       This is default behavior which gets triggered if the above special case is not satisfied.

		allowNilable := false
		if funcObj, ok := r.ObjectOf(expr.Sel).(*types.Func); ok { // Check 1:  selector expression is a method invocation
			conf := r.Pass().ResultOf[config.Analyzer].(*config.Config)
			if conf.IsPkgInScope(funcObj.Pkg()) { // Check 2: invoked method is in scope
				t := util.TypeOf(r.Pass(), expr.X)
				// Here, `t` can only be of type struct or interface, of which we only support for structs (see .
				if util.TypeAsDeeplyStruct(t) != nil { // Check 3: invoking expression (caller) is of struct type
					allowNilable = true
					// We are in the special case of supporting nilable receivers! Can be nilable depending on declaration annotation/inferred nilability.
					r.AddConsumption(&annotation.ConsumeTrigger{
						Annotation: annotation.RecvPass{
							TriggerIfNonNil: annotation.TriggerIfNonNil{
								Ann: annotation.RecvAnnotationKey{
									FuncDecl: funcObj,
								},
							}},
						Expr:   expr.X,
						Guards: util.NoGuards(),
					})
				}
			}
		}
		if !allowNilable {
			// We are in the default case -- it's a field/method access! Must be non-nil.
			r.AddConsumption(&annotation.ConsumeTrigger{
				Annotation: annotation.FldAccess{Sel: r.ObjectOf(expr.Sel)},
				Expr:       expr.X,
				Guards:     util.NoGuards(),
			})
		}
		r.AddComputation(expr.X)
	case *ast.SliceExpr:
		// similar to index case

		// zero slicing contains b[:0] b[0:0] b[0:] b[:] b[0:0:0], which are safe even when b is
		// nil, so we do not create consumer triggers for those slicing.
		if !r.isZeroSlicing(expr) {
			// For all the other slicing, the slice must be nonnil, so we create a consumer
			// trigger.
			r.AddConsumption(&annotation.ConsumeTrigger{
				Annotation: annotation.SliceAccess{},
				Expr:       expr.X,
				Guards:     util.NoGuards(),
			})
		}

		r.AddComputation(expr.X)
		r.AddComputation(expr.Low)
		r.AddComputation(expr.High)
		r.AddComputation(expr.Max)
	case *ast.StarExpr:
		// pointer load! definitely must be non-nil
		r.AddConsumption(&annotation.ConsumeTrigger{
			Annotation: annotation.PtrLoad{},
			Expr:       expr.X,
			Guards:     util.NoGuards(),
		})
		r.AddComputation(expr.X)
	case *ast.TypeAssertExpr:
		// doesn't need to be non-nil, but really should be
		r.AddComputation(expr.X)
	case *ast.UnaryExpr:
		// channel receive case
		if expr.Op == token.ARROW {
			// added this consumer since receiving over a nil channel can cause panic
			r.AddConsumption(&annotation.ConsumeTrigger{
				Annotation: annotation.ChanAccess{},
				Expr:       expr.X,
				Guards:     util.NoGuards(),
			})
		}
		r.AddComputation(expr.X)
	case *ast.FuncLit:
		// TODO: analyze the bodies of anonymous functions
	default:
		// TODO - once debugger is working - fill in cases here
		// if we don't recognize the node - do nothing
	}
}

// getFuncIdent returns the function identified from a call expression. If the function
// is an anonymous function, it will return the fake function declaration created in the
// function analyzer
func getFuncIdent(expr *ast.CallExpr, fc *FunctionContext) *ast.Ident {
	ident := util.FuncIdentFromCallExpr(expr)

	var funcLit *ast.FuncLit
	// if ident is nil, check if the expr represents a FuncLit node
	if ident == nil {
		funcLit, _ = expr.Fun.(*ast.FuncLit)
	} else {
		// check if the declaration the ident points to a function literal node
		funcLit = getFuncLitFromAssignment(ident)
	}

	if funcLit != nil {
		if info, ok := fc.funcLitMap[funcLit]; ok {
			return info.FakeFuncDecl.Name
		}
	}

	return ident
}

// getFuncLitFromAssignment if the declaration of the ident is an assignment
// statement and Rhs of the assignment is a call expression which represents an
// anonymous function, returns the ident of the fake function declaration created
// for that. Otherwise, return nil.
func getFuncLitFromAssignment(ident *ast.Ident) *ast.FuncLit {
	if ident.Obj == nil || ident.Obj.Decl == nil {
		return nil
	}

	if assign, ok := ident.Obj.Decl.(*ast.AssignStmt); ok {
		// TODO get the correct ident for many to one assignments
		if len(assign.Lhs) != len(assign.Rhs) {
			return nil
		}

		for i := range assign.Lhs {
			if assign.Lhs[i].(*ast.Ident).Obj != ident.Obj {
				continue
			}
			if rhs, ok := assign.Rhs[i].(*ast.FuncLit); ok {
				return rhs
			}
		}
	}

	return nil
}

// LiftFromPath takes a `path` of assertion nodes, and searches for it in the assertion tree rooted
// at `rootNode`. If found, it removes that tree and returns its root as `node`, with `ok` = true.
// If not found, it returns `node`, `ok` = nil, false
//
// This is used as the first half of an assignment between trackable expressions. The two halves are
// kept separate to allow them to be separated into two parallel phases in the case of multiple
// assignments, but for illustrative purposes, here is how a self-contained single assignment method
// would look:
//
// ```
//
//	func (rootNode *RootAssertionNode) AddAssignment(dstpath, srcpath TrackableExpr) {
//		node, ok := rootNode.LiftFromPath(dstpath)
//		if ok {
//			rootNode.LandAtPath(srcpath, node)
//		}
//	}
//
// ```
//
// nilable(path, result 0)
func (r *RootAssertionNode) LiftFromPath(path TrackableExpr) (AssertionNode, bool) {
	if path != nil {
		node, whichChild := r.lookupPath(path)
		if node != nil {
			detachFromParent(node, whichChild)
			return node, true
		}
	}
	return nil, false
}

// LandAtPath takes a `path` of assertion nodes, and another target `node`, and places that target
// into the assertion tree rooted at `rootNode` at the location specified by `path`. It fails only
// if `path` is nil.
//
// This is used as the second half of an assignment between trackable expressions. For information on
// why this is done, and an example of how to complete an entire assignment, see `LiftFromPath`'s
// documentation.
func (r *RootAssertionNode) LandAtPath(path TrackableExpr, node AssertionNode) {
	if path != nil {
		newRoot := r.linkPath(path)
		newNode := path[len(path)-1]
		newNode.SetConsumeTriggers(node.ConsumeTriggers())
		newNode.SetChildren(node.Children())

		r.mergeInto(r, newRoot)
	}
}

// RootFunc is a function type taking a RootAssertionNode pointer as a parameter
type RootFunc = func(*RootAssertionNode)

// ProcessEntry is called when an assertion tree is known to have reached the entry to its function
// It takes any remaining assertions (consumeTriggers) and conclusively resolves them
// (see for len(self.Children()) > 0) condition by:
// - producing all parameters to the function from their appropriate annotations (paramAnnotationKey)
// - producing all non-parameter variables as definitely nil (noVarAssign)
// - producing all remaining function assertions according to their annotation (retAnnotationKey)
func (r *RootAssertionNode) ProcessEntry() {
	for len(r.Children()) > 0 {
		child := r.Children()[0]
		builtExpr := child.BuildExpr(r.Pass(), nil)

		if r.functionContext.isDepthOneFieldCheck() {
			// process field Assertion nodes of function parameters
			r.addProductionsForParamFields(child, builtExpr)
		}

		r.AddProduction(&annotation.ProduceTrigger{
			Annotation: child.DefaultTrigger(),
			Expr:       builtExpr,
		})
	}

	// filter triggers for error return handling -- intra-procedural
	if util.FuncIsErrReturning(r.FuncObj()) {
		r.triggers, _ = FilterTriggersForErrorReturn(
			r.triggers,
			func(p *annotation.ProduceTrigger) ProducerNilability {
				kind := p.Annotation.Kind()
				switch kind {
				case annotation.Always:
					return ProducerIsNil
				case annotation.Never:
					return ProducerIsNonNil
				default:
					return ProducerNilabilityUnknown
				}
			},
		)
	}

	for i := range r.triggers {
		r.triggers[i] = CheckGuardOnFullTrigger(r.triggers[i])
	}
}

// performs a shallow comparison of two nodes - doesn't recur into their subtrees and doesn't look at triggers
// invariant on AssertionNodes is that this can never hold between any two of their distinct children
func (r *RootAssertionNode) shallowEqNodes(left, right AssertionNode) bool {
	switch left := left.(type) {
	case *RootAssertionNode:
		right, ok := right.(*RootAssertionNode)
		if !ok {
			return false
		}
		if left.FuncDecl() != right.FuncDecl() {
			return false
		}
	case *varAssertionNode:
		right, ok := right.(*varAssertionNode)
		if !ok {
			return false
		}
		if left.decl != right.decl {
			return false
		}
	case *fldAssertionNode:
		right, ok := right.(*fldAssertionNode)
		if !ok {
			return false
		}
		if left.decl != right.decl {
			return false
		}
	case *funcAssertionNode:
		right, ok := right.(*funcAssertionNode)
		if !ok {
			return false
		}
		if left.decl != right.decl {
			return false
		}
		if len(left.args) != len(right.args) {
			return false
		}
		for i := range left.args {
			if right.args == nil {
				// TODO: remove this when  is implemented and we can replace it with a real suppression
				return false
			}
			if !r.eqStable(left.args[i], right.args[i]) {
				return false
			}
		}
	case *indexAssertionNode:
		right, ok := right.(*indexAssertionNode)
		if !ok {
			return false
		}
		if !r.eqStable(left.index, right.index) {
			return false
		}
	default:
		panic("unrecognized node type")
	}
	return true
}

// compares full equality, used as fixed point condition for iteration
func (r *RootAssertionNode) eqNodes(left, right AssertionNode) bool {
	if !r.shallowEqNodes(left, right) ||
		!annotation.ConsumeTriggerSlicesEq(left.ConsumeTriggers(), right.ConsumeTriggers()) ||
		len(left.Children()) != len(right.Children()) {
		return false
	}
	if lroot, ok := left.(*RootAssertionNode); ok {
		if rroot, ok := right.(*RootAssertionNode); ok {
			if !annotation.FullTriggerSlicesEq(lroot.triggers, rroot.triggers) {
				return false
			}
		} else {
			// nodes have different types!
			return false
		}
	}
lsearch:
	for _, lchild := range left.Children() {
		for _, rchild := range right.Children() {
			if r.eqNodes(lchild, rchild) {
				continue lsearch
			}
		}
		return false
	}
	return true
}

// checks if a builtin - e.g. "new" or "make"
func (r *RootAssertionNode) isBuiltIn(ident *ast.Ident) bool {
	_, ok := r.ObjectOf(ident).(*types.Builtin)
	return ok
}

// checks if a constant - e.g. "true"
func (r *RootAssertionNode) isConst(ident *ast.Ident) bool {
	_, ok := r.ObjectOf(ident).(*types.Const)
	return ok
}

// checks if the literal value nil
func (r *RootAssertionNode) isNil(ident *ast.Ident) bool {
	// sometimes we have to insert freshly created nil literal ast nodes, so we add this check to make
	// sure they're handled
	// it's sound because nil is a reserved name, so if an object is named nil it really has to be nil
	// the case handled below takes care of known compile time aliases of nil
	if ident.Name == "nil" {
		return true
	}

	_, ok := r.ObjectOf(ident).(*types.Nil)
	return ok
}

// checks if this expression is an instance of types.Func
// this condition holds only for functions defined in the source - not builtins
func (r *RootAssertionNode) isFunc(ident *ast.Ident) bool {
	_, ok := r.ObjectOf(ident).(*types.Func)
	return ok
}

// checks if this is a package name
func (r *RootAssertionNode) isPkgName(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		_, ok := r.ObjectOf(ident).(*types.PkgName)
		return ok
	}
	return false
}

// checks if this is a type name
func (r *RootAssertionNode) isTypeName(expr ast.Expr) bool {
	if ident, ok := expr.(*ast.Ident); ok {
		_, ok := r.ObjectOf(ident).(*types.TypeName)
		return ok
	}
	return false
}

// checks if an expression is a type
func (r *RootAssertionNode) isType(expr ast.Expr) bool {
	return r.Pass().TypesInfo.Types[expr].IsType()
}

// isZeroSlicing returns if the given slice expression is a special case that will not cause panic
// even when the slice itself is nil, i.e, one of [:0] [0:0] [0:] [:] [0:0:0]
func (r *RootAssertionNode) isZeroSlicing(expr *ast.SliceExpr) bool {
	lo, hi, max := expr.Low, expr.High, expr.Max
	return ((lo == nil || r.isIntZero(lo)) && r.isIntZero(hi) && max == nil) || // [:0] [0:0]
		((lo == nil || r.isIntZero(lo)) && hi == nil && max == nil) || // [0:] [:]
		r.isIntZero(lo) && r.isIntZero(hi) && r.isIntZero(max) // [0:0:0]
}

// isIntZero returns if the given expression is evaluated to integer zero at compile time. For
// example, zero literal, zero const or binary expression that evaluates to zero, e.g., 1 - 1
// should all return true. Note the function will return false for zero string `"0"`.
func (r *RootAssertionNode) isIntZero(expr ast.Expr) bool {
	tv, ok := r.Pass().TypesInfo.Types[expr]
	if !ok {
		return false
	}
	intValue, ok := constant.Val(tv.Value).(int64)
	return ok && intValue == 0
}

// This function defines whether an expression is `stable` - i.e. whether we assume it constant
// across multiple syntactic accesses. This obviously includes literal expressions closed under
// builtin logical and arithmetic expressions, but, by assumption, includes function calls and
// indexes by other `stable` expressions
func (r *RootAssertionNode) isStable(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.BasicLit:
		return true
	case *ast.BinaryExpr:
		return r.isStable(expr.X) && r.isStable(expr.Y)
	case *ast.UnaryExpr:
		return r.isStable(expr.X)
	case *ast.ParenExpr:
		return r.isStable(expr.X)
	case *ast.CallExpr:
		for _, arg := range expr.Args {
			if !r.isStable(arg) {
				return false
			}
		}
		return r.isStable(expr.Fun)
	case *ast.Ident:
		// There are three cases in which we admit an identifier is a stable:
		// if it is a builtin name, if it is a function name, or if it is const.
		// Package is considered a special case of ident to suppport selector expressions used to access stable
		// expressions, such as constants declared in another package (e.g., pkg.Const)
		if r.isBuiltIn(expr) || r.isConst(expr) || r.isNil(expr) || r.isPkgName(expr) {
			return true
		}

		// TODO: check for function names
		return false
	case *ast.SelectorExpr:
		return r.isStable(expr.Sel) && r.isStable(expr.X)
	default:
		return false
	}
}

// Between two stable expressions, check if we expect them to produce the same value
// precondition: isStable(left) && isStable(right), then checks if left and right are equal
func (r *RootAssertionNode) eqStable(left, right ast.Expr) bool {
	right = util.StripParens(right).(ast.Expr)
	switch left := util.StripParens(left).(type) {
	case *ast.BasicLit:
		if right, ok := right.(*ast.BasicLit); ok {
			return left.Value == right.Value
		}
		return false
	case *ast.BinaryExpr:
		if right, ok := right.(*ast.BinaryExpr); ok {
			return left.Op == right.Op &&
				r.eqStable(left.X, right.X) && r.eqStable(left.Y, right.Y)
		}
		return false
	case *ast.UnaryExpr:
		if right, ok := right.(*ast.UnaryExpr); ok {
			return left.Op == right.Op && r.eqStable(left.X, right.X)
		}
		return false
	case *ast.CallExpr:
		if right, ok := right.(*ast.CallExpr); ok {
			if len(left.Args) != len(right.Args) {
				return false
			}
			for i := range left.Args {
				if !r.eqStable(left.Args[i], right.Args[i]) {
					return false
				}
			}
			return r.eqStable(left.Fun, right.Fun)
		}
		return false
	case *ast.Ident:
		if right, ok := right.(*ast.Ident); ok {
			// if the two identifiers are special values, just check them for string equality
			if (r.isNil(left) && r.isNil(right)) ||
				(r.isBuiltIn(left) && r.isBuiltIn(right)) ||
				(r.isConst(left) && (r.isConst(right))) ||
				(r.isPkgName(left) && r.isPkgName(right)) {
				return left.Name == right.Name
			}
			rightVarObj, rightOk := r.ObjectOf(right).(*types.Var)
			leftVarObj, leftOk := r.ObjectOf(left).(*types.Var)

			if !rightOk || !leftOk {
				return false // here, we have eliminated all of the cases in which
				// non-variable identifiers can be equal, so if either side is a
				// non-variable then the sides are not equal
			}
			// if they are variables, check them for declaration equality
			return leftVarObj == rightVarObj
		}
		return false
	case *ast.SelectorExpr:
		if right, ok := right.(*ast.SelectorExpr); ok {
			if !r.eqStable(left.Sel, right.Sel) {
				return false
			}
			return r.eqStable(left.X, right.X)
		}
		return false
	default:
		return false
	}
}

// precondition: shallowEqNodes(left, right), then copies remaining data from RIGHT INTO LEFT
func (r *RootAssertionNode) mergeInto(left, right AssertionNode) {
	if !r.shallowEqNodes(left, right) {
		panic("mergeInto is meaningless and erroneous for non-shallow-eq nodes")
	}
	// merge in consumers
	left.SetConsumeTriggers(
		annotation.MergeConsumeTriggerSlices(
			left.ConsumeTriggers(),
			right.ConsumeTriggers()))

	if left, lok := left.(*RootAssertionNode); lok {
		right := right.(*RootAssertionNode)
		left.triggers = annotation.MergeFullTriggers(left.triggers, right.triggers...)
	}

	// merge in children
rchildloop:
	for _, rchild := range right.Children() {
		for _, lchild := range left.Children() {
			if r.shallowEqNodes(lchild, rchild) {
				r.mergeInto(lchild, rchild)
				continue rchildloop
			}
		}
		// no existing matching child found, so add one
		freshrchild := CopyNode(rchild)
		freshrchild.SetParent(left)
		left.SetChildren(append(left.Children(), freshrchild))
	}
}
