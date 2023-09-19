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
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

// GetDeclaringPath finds the path of nested AST nodes beginning with the passed interval `[start, end]`
func GetDeclaringPath(pass *analysis.Pass, start, end token.Pos) ([]ast.Node, bool) {
	astFile := lookupAstFromFile(pass, pass.Fset.File(start))
	if astFile == nil {
		astFile = lookupAstFromFilename(pass, pass.Fset.Position(start).Filename)
	}
	if astFile != nil {
		path, _ := astutil.PathEnclosingInterval(astFile, start, end)
		return path, true
	}
	return nil, false
}

// each of the following deepNilabilityOf... functions serves to inspect an object for possible sites
// that could grant it a deep nilability annotation. Every case defaults to just introspecting the
// type itself - as named types can be declared with annotations; this is the call to `DeepNilabilityAsNamedType`
// as the final return of each function. Other sites to check are the parameters and results of functions
// global variables, and struct fields

// this combines each of the special-cased deep nilability introspectors to give a method that
// determines a deep nilability trigger for an arbitrary assertion node
func deepNilabilityTriggerOf(node AssertionNode) annotation.ProducingAnnotationTrigger {
	if node == nil {
		panic("deepNilabilityTriggerOf should not be called on nil node")
	}

	switch node := node.(type) {
	case *varAssertionNode:
		if node.Root() == nil {
			panic("deepNilabilityTriggerOf should only be called on nodes in a valid assertion tree")
		}
		return annotation.DeepNilabilityOfVar(node.Root().FuncObj(), node.decl)
	case *fldAssertionNode:
		return annotation.DeepNilabilityOfFld(node.decl)
	case *indexAssertionNode:
		return annotation.DeepNilabilityAsNamedType(node.valType)
	case *RootAssertionNode:
		panic("deepNilabilityTriggerOf should NOT be called not the root node - as this would" +
			" imply an indexNode is a child of the root node")
	case *funcAssertionNode:
		if util.FuncNumResults(node.decl) != 1 {
			panic("multiply returning function entered into assertion tree - " +
				"this should never happen")
		}
		return annotation.DeepNilabilityOfFuncRet(node.decl, 0)
	default:
		panic(fmt.Sprintf("unrecognized node type %T: add case", node))
	}
}

// TODO:
//   add another assertion node to track pointer loads: i.e. to allow:
//   if *a != nil { *(*a) }
//   safely

// TrackableExpr represents an expression that we track - i.e. observe non-local nilability properties
// of. If `e = nil; e.f` throws an error regardless of annotations, then `e` is trackable, for example.
// This notion exactly aligns with lists of `AssertionNode`s
type TrackableExpr []AssertionNode

// MinimalString for a TrackableExpr returns a sequence of minimal string representations of its
// contained nodes
func (t TrackableExpr) MinimalString() string {
	if len(t) == 0 {
		return "<empty>"
	}
	out := t[0].MinimalString()
	for _, subexpr := range t[1:] {
		out += fmt.Sprintf(".%s", subexpr.MinimalString())
	}
	return out
}

func detachFromParent(node AssertionNode, whichChild int) {
	if node.Parent() == nil {
		panic("passed assertion node has no parent - cannot detach")
	}
	if len(node.Parent().Children()) <= whichChild {
		panic(fmt.Sprintf("passed assertion node only has %d children - "+
			"cannot remove child %d", len(node.Parent().Children()), whichChild))
	}

	node.Parent().SetChildren(append(
		node.Parent().Children()[:whichChild],
		node.Parent().Children()[whichChild+1:]...))
	node.SetParent(nil)
}

func converseToken(t token.Token) token.Token {
	switch t {
	case token.EQL:
		return token.EQL
	case token.NEQ:
		return token.NEQ
	case token.LSS:
		return token.GTR
	case token.GTR:
		return token.LSS
	case token.LEQ:
		return token.GEQ
	case token.GEQ:
		return token.LEQ
	}
	panic(fmt.Sprintf("unrecognized token %s has no known converse", t))
}

func inverseToken(t token.Token) token.Token {
	switch t {
	case token.EQL:
		return token.NEQ
	case token.NEQ:
		return token.EQL
	case token.LSS:
		return token.GEQ
	case token.GTR:
		return token.LEQ
	case token.LEQ:
		return token.GTR
	case token.GEQ:
		return token.LSS
	}
	panic(fmt.Sprintf("unrecognized token %s has no known inverse", t))
}

// AddNilCheck takes the knowledge that an expression `expr` was evaluated as part of a conditional
// and incorporates it into the assertion tree by producing non-nil or nil at expr, if expr is trackable
//
// this function does not have to handle boolean operators or short circuiting because that is done
// as a restructuring pass on the CFG itself (see restructureBlocks)
//
// Notably, it is "curried" - the expression and branch identifier are passed first, followed by
// the assertion node being modified. This is so that the processing for it can be done at most once
//
// It returns two functions, the first: `trueCheck`, can be called on a *RootAssertionNode to
// incorporate the knowledge that `expr` was evaluated to true, and the second: `falseCheck` can be
// called on a *RootAssertionNode to incorporate the knowledge that `expr` was evaluated to false
//
// For better performance by the caller, it also returns a boolean flag `isNoop` indicating whether
// the returned function is a no-op
func AddNilCheck(pass *analysis.Pass, expr ast.Expr) (trueCheck, falseCheck RootFunc, isNoop bool) {
	noop := func(node *RootAssertionNode) {}

	binExpr, ok := util.StripParens(expr).(*ast.BinaryExpr)

	if !ok {
		return noop, noop, true // is not a binary expression - do no work
	}

	asLenCall := func(expr ast.Expr) (ast.Expr, bool) {
		if call, ok := expr.(*ast.CallExpr); ok {
			if fun, ok := call.Fun.(*ast.Ident); ok {
				if fun.Name == "len" && len(call.Args) == 1 {
					return call.Args[0], true
				}
			}
		}
		return nil, false
	}

	isLiteralZeroInt := func(expr ast.Expr) bool {
		if lit, ok := expr.(*ast.BasicLit); ok {
			if lit.Kind == token.INT && lit.Value == "0" {
				return true
			}
		}
		return false
	}

	isLiteralPositiveInt := func(expr ast.Expr) bool {
		if lit, ok := expr.(*ast.BasicLit); ok {
			if lit.Kind == token.INT && lit.Value != "0" && lit.Value[0] != '-' {
				return true
			}
		}
		return false
	}

	isLiteralInt := func(expr ast.Expr) bool {
		// this handles negative int literals by stripping their `-` sign before the real check
		if unExpr, ok := expr.(*ast.UnaryExpr); ok {
			if unExpr.Op == token.SUB {
				expr = unExpr.X
			}
		}
		if lit, ok := expr.(*ast.BasicLit); ok {
			return lit.Kind == token.INT
		}
		return false
	}

	// We optimistically assume that non-literal integer typed expressions in length checks
	// are positive - see the uses of this function below to admit nonliteral ints everywhere
	// positive ints are matched on
	// TODO - evaluate the unsoundness of this assumption in practice more completely
	isNonLiteralInt := func(expr ast.Expr) bool {
		if isLiteralInt(expr) {
			return false
		}
		if t, ok := pass.TypesInfo.Types[expr].Type.(*types.Basic); ok {
			return t.Info()&types.IsInteger != 0
		}
		return false
	}

	produceNegativeNilCheck := func(expr ast.Expr) RootFunc {
		return produceExprByTrigger(expr, &annotation.NegativeNilCheck{ProduceTriggerNever: &annotation.ProduceTriggerNever{}})
	}

	// An exprCheck is a pattern that we match on that, if successful, will give us a pair
	// of functions updating a root node in the true and false branches of a conditional
	// to indicate the information that can be gained from that conditional

	type exprCheck struct {
		// op is the operation of the binary expression being parsed that this exprCheck expects
		op token.Token
		// matcher is a function that examines the two operands of the binary expression
		// and can potentially trigger a return from the enclosing call to `AddNillCheck`
		// if it finds a match
		matcher func(ast.Expr, ast.Expr) (RootFunc, RootFunc, bool)
	}

	// this is the list of checkers that we currently recognize
	checkers := []exprCheck{
		{ // this exprCheck matches on expressions like `nil == a`
			op: token.EQL,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if util.IsLiteral(x, "nil") && !util.IsLiteral(y, "nil") {
					return noop, produceNegativeNilCheck(y), false
				}
				return noop, noop, true
			},
		},
		{ // this exprCheck matches on expressions like `len(a) == 0`
			op: token.EQL,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if lenArg, isLen := asLenCall(x); isLen && isLiteralZeroInt(y) {
					return noop, produceNegativeNilCheck(lenArg), false
				}
				return noop, noop, true
			},
		},
		{ // this exprCheck matches on expressions like `len(a) == len(b)`
			// we interpret these as generating non-nil for both `a` and `b`, which is technically
			// unsound, but in practice is used sufficiently frequently that we seem to need to admit
			// it
			// TODO - evaluate the impact of this unsound assumption, and maybe switch to treating
			// it as a contract that only generates non-nil for one side when the other is checked
			op: token.EQL,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				xLenArg, xIsLen := asLenCall(x)
				yLenArg, yIsLen := asLenCall(y)

				if xIsLen && yIsLen {
					return composeRootFuncs(
						produceNegativeNilCheck(xLenArg),
						produceNegativeNilCheck(yLenArg),
					), noop, false
				}
				return noop, noop, true
			},
		},
		{ // this exprCheck matches on expressions like `len(a) == 37` or `len(a) == b`
			op: token.EQL,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if lenArg, isLen := asLenCall(x); isLen && (isLiteralPositiveInt(y) || isNonLiteralInt(y)) {
					return produceNegativeNilCheck(lenArg), noop, false
				}
				return noop, noop, true
			},
		},
		{ // this exprCheck matches on expressions like `len(a) > 0` or `len(a) > 9`
			op: token.GTR,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if lenArg, isLen := asLenCall(x); isLen && (isLiteralZeroInt(y) || isLiteralPositiveInt(y) || isNonLiteralInt(y)) {
					return produceNegativeNilCheck(lenArg), noop, false
				}
				return noop, noop, true
			},
		},
		{ // this exprCheck matches on expressions like `len(a) >= 19`
			op: token.GEQ,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if lenArg, isLen := asLenCall(x); isLen && (isLiteralPositiveInt(y) || isNonLiteralInt(y)) {
					return produceNegativeNilCheck(lenArg), noop, false
				}
				return noop, noop, true
			},
		},
	}

	// this applies each of the checkers to see if we can use it to trigger a return from this function
	// the converse, inverse, and contrapositive of each exprCheck is checked as well
	for _, check := range checkers {
		if binExpr.Op == check.op {
			trueCheck, falseCheck, isNoop = check.matcher(binExpr.X, binExpr.Y)
			if !isNoop {
				return
			}
		}
		if binExpr.Op == converseToken(check.op) {
			trueCheck, falseCheck, isNoop = check.matcher(binExpr.Y, binExpr.X)
			if !isNoop {
				return
			}
		}
		if binExpr.Op == inverseToken(check.op) {
			falseCheck, trueCheck, isNoop = check.matcher(binExpr.X, binExpr.Y)
			if !isNoop {
				return
			}
		}
		if binExpr.Op == converseToken(inverseToken(check.op)) {
			falseCheck, trueCheck, isNoop = check.matcher(binExpr.Y, binExpr.X)
			if !isNoop {
				return
			}
		}
	}
	return noop, noop, true
}

func produceExprByTrigger(expr ast.Expr, trigger annotation.ProducingAnnotationTrigger) RootFunc {
	return func(self *RootAssertionNode) {
		self.AddProduction(&annotation.ProduceTrigger{
			Annotation: trigger,
			Expr:       expr,
		})
	}
}

// CopyNode computes a deep code of an AssertionNode
// precondition: node is not nil
func CopyNode(node AssertionNode) AssertionNode {
	var fresh AssertionNode
	switch node := node.(type) {
	case *RootAssertionNode:
		fresh = &RootAssertionNode{
			triggers:        append(make([]annotation.FullTrigger, 0), node.triggers...),
			exprNonceMap:    node.exprNonceMap,
			functionContext: node.functionContext,
		}
	case *varAssertionNode:
		fresh = &varAssertionNode{decl: node.decl}
	case *fldAssertionNode:
		fresh = &fldAssertionNode{decl: node.decl, functionContext: node.functionContext}
	case *funcAssertionNode:
		fresh = &funcAssertionNode{decl: node.decl, args: node.args}
	case *indexAssertionNode:
		fresh = &indexAssertionNode{
			index:    node.index,
			valType:  node.valType,
			recvType: node.recvType}
	default:
		panic("unrecognized node type")
	}

	fresh.SetChildren(make([]AssertionNode, 0, len(node.Children())))
	for _, child := range node.Children() {
		freshChild := CopyNode(child)
		freshChild.SetParent(fresh)
		fresh.SetChildren(append(fresh.Children(), freshChild))
	}

	fresh.SetConsumeTriggers(append(make([]*annotation.ConsumeTrigger, 0, len(node.ConsumeTriggers())), node.ConsumeTriggers()...))

	return fresh
}

// lookupAstFromFile attempts to find the file ast for a given file (token.File)
// nilable(result 0)
func lookupAstFromFile(pass *analysis.Pass, file *token.File) *ast.File {
	for _, astFile := range pass.Files {
		if file != nil && int(astFile.Pos()) >= file.Base() && int(astFile.Pos()) <= file.Base()+file.Size() {
			return astFile
		}
	}
	return nil
}

// lookupAstFromFilename attempts to find the file ast for a given file (token.File)
// nilable(result 0)
func lookupAstFromFilename(pass *analysis.Pass, filename string) *ast.File {
	for _, astFile := range pass.Files {
		if astFile.Name.Name == filename {
			return astFile
		}
	}
	return nil
}

// ProducerNilability is a type to denote the nilability status of the producer.
type ProducerNilability uint8

const (
	// ProducerNilabilityUnknown is the default value when a producer's nilability is not guaranteed to be nil or nonnil --> TriggerIfNilable and TriggerifDeepNilable
	ProducerNilabilityUnknown ProducerNilability = iota
	// ProducerIsNil is when the producer is guranteed to produce a nil value --> ProduceTriggerTautology
	ProducerIsNil
	// ProducerIsNonNil is when the producer is guranteed to produce a non-nil value --> ProduceTriggerNever
	ProducerIsNonNil
)

// FilterTriggersForErrorReturn analyzes return expression triggers of error returning functions to filter out redundant
// triggers based on the error contract that were earlier conservatively added in `handleErrorReturns`. The function operates in two steps:
// (1) infer nilability status of the error return expression based on the producers for its corresponding trigger, and
// (2) remove redundant triggers based on the inferred nilability of error return, and appropriately update consumers of the remaining triggers
//
// FilterTriggersForErrorReturn takes two arguments:
// (1) set of triggers that need to be filtered. These are function-level triggers for intra-procedural analysis,
// and package-level for inter-procedural analysis
// (2) a computeProducerNilability that defines how to compute the nilability status of a given producer. Particularly,
// we are interested in knowing the nilability of the producer of the error return consumer (`UseAsErrorRetWithNilabilityUnknown`).
// This argument is needed since the computation differs from intra-procedural to inter-procedural analysis,
// as well as between different modes of inference.
//
// FilterTriggersForErrorReturn produces two outputs:
// (1) final set of triggers that is filtered and refined by replacing consumers
// (2) raw set of deleted triggers
func FilterTriggersForErrorReturn(
	triggers []annotation.FullTrigger,
	computeProducerNilability func(p *annotation.ProduceTrigger) ProducerNilability,
) (filteredTriggers []annotation.FullTrigger, deletedTriggers []annotation.FullTrigger) {
	if len(triggers) == 0 {
		return nil, nil
	}

	// used to assign the nilability status of the error return expression
	const (
		// unknown						// 0b00
		_errRetNil    uint8 = 1 << iota // 0b01
		_errRetNonnil                   // 0b10
		// mixed (nil+nonnil)			// 0b11
	)

	type info struct {
		nilability     uint8
		errTriggers    []int
		nonErrTriggers []int
	}

	// Step 1: collection phase for return statements in the function for:
	// (1) inferred nilability of the error return expression (stored in `info.nilability`)
	// (2) trigger indices for return expressions (stored in `info.errTriggers` and `info.nonErrTriggers`)
	retTriggers := make(map[*ast.ReturnStmt]info)
	for i, t := range triggers {
		switch c := t.Consumer.Annotation.(type) {
		case *annotation.UseAsErrorRetWithNilabilityUnknown:
			v := retTriggers[c.RetStmt]
			v.errTriggers = append(v.errTriggers, i)

			// "|" helps to encode if both nil and non-nil values are found through different paths
			prodNilability := computeProducerNilability(t.Producer)
			if prodNilability == ProducerIsNonNil {
				v.nilability |= _errRetNonnil
			} else if prodNilability == ProducerIsNil {
				v.nilability |= _errRetNil
			}
			retTriggers[c.RetStmt] = v

		case *annotation.UseAsNonErrorRetDependentOnErrorRetNilability:
			v := retTriggers[c.RetStmt]
			v.nonErrTriggers = append(v.nonErrTriggers, i)
			retTriggers[c.RetStmt] = v
		}
	}

	// Step 2: remove redundant triggers based on the nilability status collected in Step 1. There are three cases we need to consider:
	// (1) error return = nil: remove error trigger, update consumers of non-error triggers
	// (2) error return = non-nil: remove non-error triggers, update consumer of error trigger
	// (3) error return = mixed: remove error trigger, but don't update consumers of non-error triggers in the interest of printing the appropriate error message
	// (4) error return = unknown: noop
	allDelIndices := make(map[int]bool)

	for _, v := range retTriggers {
		switch v.nilability {
		case _errRetNil:
			// mark error return trigger to be removed
			for _, i := range v.errTriggers {
				allDelIndices[i] = true
			}

			// update the placeholder non-error returns consumer `UseAsNonErrorRetDependentOnErrorRetNilability` with `UseAsReturn`
			for _, i := range v.nonErrTriggers {
				if old, ok := triggers[i].Consumer.Annotation.(*annotation.UseAsNonErrorRetDependentOnErrorRetNilability); ok {
					if oldAnn, ok := old.Ann.(*annotation.RetAnnotationKey); ok {
						triggers[i].Consumer = &annotation.ConsumeTrigger{
							Annotation: &annotation.UseAsReturn{
								TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: oldAnn},
								IsNamedReturn:   old.IsNamedReturn,
								RetStmt:         old.RetStmt,
							},
							Expr:   triggers[i].Consumer.Expr,
							Guards: triggers[i].Consumer.Guards,
						}
					}
				}
			}

		case _errRetNonnil:
			// mark non-error return triggers to be removed
			for _, i := range v.nonErrTriggers {
				allDelIndices[i] = true
			}

			// update the placeholder error return consumer `UseAsErrorRetWithNilabilityUnknown` with `UseAsErrorResult`
			for _, i := range v.errTriggers {
				if old, ok := triggers[i].Consumer.Annotation.(*annotation.UseAsErrorRetWithNilabilityUnknown); ok {
					if oldAnn, ok := old.Ann.(*annotation.RetAnnotationKey); ok {
						triggers[i].Consumer = &annotation.ConsumeTrigger{
							Annotation: &annotation.UseAsErrorResult{
								TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: oldAnn},
								IsNamedReturn:   old.IsNamedReturn,
								RetStmt:         old.RetStmt,
							},
							Expr:   triggers[i].Consumer.Expr,
							Guards: triggers[i].Consumer.Guards,
						}
					}
				}
			}

		case _errRetNil | _errRetNonnil:
			// this is the case of mixed nilability, i.e., we know that through at least one path, error return was nil,
			// so non-error returns should be tracked for checking nilability. Therefore, mark error return trigger to be
			// removed so that it doesn't get reevaluated later, but don't replace `UseAsNonErrorRetDependentOnErrorRetNilability`
			// with `UseAsReturn` for the sake of printing the appropriate error message
			for _, i := range v.errTriggers {
				allDelIndices[i] = true
			}

		default:
			// Noop: case of unknown. Don't remove or update any triggers since the nilability of error return expression is not guaranteed
		}
	}

	// delete all marked indices
	if len(allDelIndices) == 0 {
		return triggers, nil
	}

	for i, t := range triggers {
		if !allDelIndices[i] {
			filteredTriggers = append(filteredTriggers, t)
		} else {
			deletedTriggers = append(deletedTriggers, t)
		}
	}
	return
}
