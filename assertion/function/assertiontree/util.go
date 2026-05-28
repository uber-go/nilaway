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
	"slices"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/tokenhelper"
	"go.uber.org/nilaway/util/typeshelper"
	"golang.org/x/tools/go/ast/astutil"
)

// checkCFGFixedPointRuntime panics if a fixed point iteration loop runs beyond some upper
// bounded round number, determined by the number of blocks in the CFG of the analyzed function.
func checkCFGFixedPointRuntime(passName string, currRound, numBlocks int) {
	if maxRound := numBlocks * numBlocks * 2; currRound > maxRound {
		panic(fmt.Sprintf("propagation over %d-block CFG in %q ran for "+
			"%d rounds, when maximum allowed was %d rounds.",
			numBlocks, passName, currRound, maxRound),
		)
	}
}

// GetDeclaringPath finds the path of nested AST nodes beginning with the passed interval `[start, end]`
func GetDeclaringPath(pass *analysishelper.EnhancedPass, start, end token.Pos) ([]ast.Node, bool) {
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
		if typeshelper.FuncNumResults(node.decl) != 1 {
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
func AddNilCheck(pass *analysishelper.EnhancedPass, expr ast.Expr) (trueCheck, falseCheck RootFunc, isNoop bool) {
	noop := func(_ *RootAssertionNode) {}

	expr = ast.Unparen(expr)

	if e, ok := expr.(*ast.UnaryExpr); ok && e.Op == token.NOT {
		// Check if the unary expression is a negation of a binary expression.
		// If the unary expression encloses a nil check binary expression, then the below code interchanges the true
		// and false branches produced by the binary expression. For example, if`!(v != nil)`, then AddNilCheck on the inner
		// expression `(v != nil)` returns trueCheck: produceNegativeNilCheck, falseCheck: noop, isNoop = false, implying a
		// negative nil check for the true branch. But since it is preceded with a negation (!), the below code
		// interchanges the true and false branches, and returns trueCheck: noop, falseCheck: produceNegativeNilCheck,
		// implying a negative nil check for the false branch.
		trueNilCheck, falseNilCheck, isNoop := AddNilCheck(pass, e.X)
		return falseNilCheck, trueNilCheck, isNoop
	}
	binExpr, ok := expr.(*ast.BinaryExpr)
	if !ok {
		// `expr` is not a direct or indirect binary expression - do no work
		return noop, noop, true
	}

	produceNegativeNilChecks := func(exprs ...ast.Expr) RootFunc {
		return func(node *RootAssertionNode) {
			for _, e := range exprs {
				node.AddProduction(&annotation.ProduceTrigger{
					Annotation: &annotation.NegativeNilCheck{ProduceTriggerNever: &annotation.ProduceTriggerNever{}},
					Expr:       e,
				})
			}
		}
	}

	// The list of patterns that we match on that, if successful, will give us a pair
	// of functions updating a root node in the true and false branches of a conditional
	// to indicate the information that can be gained from that conditional.
	// We will apply each of the checkers to see if we can use it to trigger a return from this
	// function.
	//
	// The converse, inverse, and contrapositive of the binary expression is checked automatically
	// below. So each checker only has to present the base case.
	checkers := []struct {
		// op is the operation of the binary expression being parsed that this exprCheck expects
		op token.Token
		// matcher is a function that examines the two operands of the binary expression
		// and can potentially trigger a return from the enclosing call to `AddNilCheck`
		// if it finds a match
		matcher func(ast.Expr, ast.Expr) (RootFunc, RootFunc, bool)
	}{
		{
			// `a == nil`
			// Automatic cases:
			//   - `nil == a`
			//   - `a != nil`
			//   - `nil != a`
			op: token.EQL,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if !pass.IsNil(x) && pass.IsNil(y) {
					return noop, produceNegativeNilChecks(x), false
				}
				return noop, noop, true
			},
		},
		{
			// `len(a) == 0`
			// Automatic cases:
			//   - `0 == len(a)`
			//   - `len(a) != 0`
			//   - `0 != len(a)`
			op: token.EQL,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if lenArgs := extractLenArgs(x, false /* allowNested */); len(lenArgs) == 1 && pass.IsZero(y) {
					return noop, produceNegativeNilChecks(lenArgs[0]), false
				}
				return noop, noop, true
			},
		},
		{
			// `len(a) - len(c) == len(b) * len(d)`
			// We interpret all slices in the `len()` calls as non-nil slices, which is technically
			// unsound but happens frequently enough in practice that it is worth doing.
			// Automatic cases:
			//   - `len(b) * len(d) == len(a) - len(c)`
			//   - `len(a) - len(c) != len(b) * len(d)`
			//   - `len(b) * len(d) != len(a) - len(c)`
			op: token.EQL,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				xLenArgs, yLenArgs := extractLenArgs(x, true /* allowNested */), extractLenArgs(y, true /* allowNested */)
				if len(xLenArgs) != 0 && len(yLenArgs) != 0 {
					return produceNegativeNilChecks(slices.Concat(xLenArgs, yLenArgs)...), noop, false
				}
				return noop, noop, true
			},
		},
		{
			// `len(a) - 1 + b == [positive-int]`
			// Automatic cases:
			//   - `[positive-int] == len(a) - 1 + b`
			//   - `len(a) - 1 + b != [positive-int]`
			//   - `[positive-int] != len(a) - 1 + b`
			op: token.EQL,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if lenArgs := extractLenArgs(x, true /* allowNested */); len(lenArgs) == 1 && likelyPositiveInt(pass, y) {
					return produceNegativeNilChecks(lenArgs[0]), noop, false
				}
				return noop, noop, true
			},
		},
		{
			// `len(a) - 1 + b > [0 or positive-int]`
			// Automatic cases:
			//   - `[0 or positive-int] < len(a) - 1 + b`
			//   - `len(a) - 1 + b <= [0 or positive-int]`
			//   - `[0 or positive-int] >= len(a) - 1 + b`
			op: token.GTR,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if lenArgs := extractLenArgs(x, true /* allowNested */); len(lenArgs) == 1 && (pass.IsZero(y) || likelyPositiveInt(pass, y)) {
					return produceNegativeNilChecks(lenArgs[0]), noop, false
				}
				return noop, noop, true
			},
		},
		{
			// `len(a) - 1 + b >= [positive-int]`
			//
			// Automatic cases:
			//   - `[positive-int] <= len(a)`
			//   - `len(a) < [positive-int]`
			//   - `[positive-int] > len(a)`
			op: token.GEQ,
			matcher: func(x, y ast.Expr) (RootFunc, RootFunc, bool) {
				if lenArgs := extractLenArgs(x, true /* allowNested */); len(lenArgs) == 1 && likelyPositiveInt(pass, y) {
					return produceNegativeNilChecks(lenArgs[0]), noop, false
				}
				return noop, noop, true
			},
		},
	}

	// Apply the checkers.
	for _, check := range checkers {
		// Note that `op` might be equal to `converse(op)`, so we must check both cases.

		// `X op Y` and `X inverse(op) Y`.
		if binExpr.Op == check.op || binExpr.Op == tokenhelper.Inverse(check.op) {
			trueCheck, falseCheck, isNoop = check.matcher(binExpr.X, binExpr.Y)
			if binExpr.Op == tokenhelper.Inverse(check.op) {
				trueCheck, falseCheck = falseCheck, trueCheck
			}
			if !isNoop {
				return
			}
		}
		// `Y converse(op) X` and `Y inverse(converse(op)) X`.
		if binExpr.Op == tokenhelper.Converse(check.op) || binExpr.Op == tokenhelper.Converse(tokenhelper.Inverse(check.op)) {
			trueCheck, falseCheck, isNoop = check.matcher(binExpr.Y, binExpr.X)
			if binExpr.Op == tokenhelper.Converse(tokenhelper.Inverse(check.op)) {
				trueCheck, falseCheck = falseCheck, trueCheck
			}
			if !isNoop {
				return
			}
		}
	}
	return noop, noop, true
}

func extractLenArgs(expr ast.Expr, allowNested bool) []ast.Expr {
	var args []ast.Expr
	ast.Inspect(expr, func(n ast.Node) bool {
		e, ok := n.(ast.Expr)
		if !ok {
			return allowNested
		}
		if call, ok := e.(*ast.CallExpr); ok {
			if fun, ok := call.Fun.(*ast.Ident); ok && fun.Name == "len" && len(call.Args) == 1 {
				args = append(args, call.Args[0])
			}
		}
		return allowNested
	})
	return args
}

// likelyPositiveInt return true if the given expression is likely a positive integer. We
// optimistically assume that non-constant int-typed expressions to be positive integers, which is
// technically unsound but happens frequently enough in practice that it is worth doing.
func likelyPositiveInt(pass *analysishelper.EnhancedPass, expr ast.Expr) bool {
	if v, ok := pass.ConstInt(expr); ok {
		return v > 0
	}
	if t, ok := pass.TypesInfo.TypeOf(expr).Underlying().(*types.Basic); ok {
		return t.Info()&types.IsInteger != 0
	}
	return false
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

	copyConsumers := make([]*annotation.ConsumeTrigger, 0, len(node.ConsumeTriggers()))
	for _, c := range node.ConsumeTriggers() {
		copyConsumers = append(copyConsumers, c.Copy())
	}
	fresh.SetConsumeTriggers(copyConsumers)

	return fresh
}

// lookupAstFromFile attempts to find the file ast for a given file (token.File)
// nilable(result 0)
func lookupAstFromFile(pass *analysishelper.EnhancedPass, file *token.File) *ast.File {
	for _, astFile := range pass.Files {
		if file != nil && int(astFile.Pos()) >= file.Base() && int(astFile.Pos()) <= file.Base()+file.Size() {
			return astFile
		}
	}
	return nil
}

// lookupAstFromFilename attempts to find the file ast for a given file (token.File)
// nilable(result 0)
func lookupAstFromFilename(pass *analysishelper.EnhancedPass, filename string) *ast.File {
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
// (1) final set of triggers that is filtered and refined by replacing consumers;
// (2) raw set of deleted triggers (nil if there are no deleted triggers).
func FilterTriggersForErrorReturn(
	triggers []annotation.FullTrigger,
	computeProducerNilability func(p *annotation.ProduceTrigger) ProducerNilability,
) (filteredTriggers []annotation.FullTrigger, deletedTriggers map[annotation.FullTrigger]bool) {
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
			switch prodNilability {
			case ProducerIsNonNil:
				v.nilability |= _errRetNonnil
			case ProducerIsNil:
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

	// Fast return if there are no triggers to be deleted.
	if len(allDelIndices) == 0 {
		return triggers, nil
	}

	// Delete all marked indices.
	deletedTriggers = make(map[annotation.FullTrigger]bool, len(allDelIndices))
	for i, t := range triggers {
		if allDelIndices[i] {
			deletedTriggers[t] = true
			continue
		}
		filteredTriggers = append(filteredTriggers, t)
	}
	return filteredTriggers, deletedTriggers
}
