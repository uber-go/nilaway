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
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/cfg"
)

// If `block` is a conditional branch (e.g. an if statement), return the expression on which it
// branches, otherwise return nil.
// nilable(result 0)
func getConditional(block *cfg.Block) ast.Expr {
	// TODO: Nilness check for `block` is currently needed due to a FP, but should not be needed
	//  after  is implemented.
	if block == nil || len(block.Nodes) == 0 || len(block.Succs) != 2 {
		return nil
	}
	if expr, ok := block.Nodes[len(block.Nodes)-1].(ast.Expr); ok {
		return expr
	}

	return nil
}

// If `block` is the precursor to a range statement, return the expression being ranged over,
// otherwise return nil.
//
// This function matches on two cases:
//   - a block terminating with an `*ast.AssignStmt` whose singular rhs is an `*ast.UnaryExpr` with operation
//     `token.RANGE` - i.e. `[x | x, y] = range z`
//   - a block terminating with an `*ast.UnaryExpr` directly, whose operation is `token.RANGE`
//
// Both of these forms are inserted during the pass in assertion.markRangeStatements when that pass
// determines it has found the output of CFG-parsing an `*ast.RangeStmt`. These two functions can thus
// be seen as a direct input/output pair.
//
// nilable(result 0)
func getRangeExpr(block *cfg.Block) ast.Expr {
	if numNodes := len(block.Nodes); numNodes > 0 {
		lastNode := block.Nodes[numNodes-1]
		if assignStmt, ok := lastNode.(*ast.AssignStmt); ok && len(assignStmt.Rhs) == 1 {
			// if we are in the former case described above, strip the assignment and focus only on
			// its rhs
			lastNode = assignStmt.Rhs[0]
		}
		// check that the last node, or the rhs of the last node if it is an assignment, is a range expression
		if unaryExpr, ok := lastNode.(*ast.UnaryExpr); ok && unaryExpr.Op == token.RANGE {
			// we've matched the given block with one of the two desired cases, so we return
			// the expression being ranged over
			return unaryExpr.X
		}
	}
	return nil
}

// a preprocessPair bundles a function `trueBranchFunc` that modifies a *RootAssertionNode from the true
// branch of a conditional with a function `falseBranchFunc` that modifies the false branch
type preprocessPair struct {
	trueBranchFunc  RootFunc
	falseBranchFunc RootFunc
}

const knownNilableErrFunc = "sometimesErrs"

// exprCallsKnownNilableErrFunc checks if expression calls a function that we know to be nilable without
// needing to consult annotations.
//
// The best mechanism for this would be to somehow expose a fixed library function that serves this
// purpose, but for now, we simply check that it has the special name "sometimesErrs" set above through
// the constant `knownNilableErrFunc`
func exprCallsKnownNilableErrFunc(expr ast.Expr) bool {
	callExpr, ok := expr.(*ast.CallExpr)

	if !ok {
		return false
	}

	ident := util.FuncIdentFromCallExpr(callExpr)

	if ident == nil {
		// no ident - anonymous function
		return false
	}

	return ident.String() == knownNilableErrFunc
}

// For a return statement - make sure all returned results are computable by generating the
// appropriate assertions, and consume each as the respective return number of that function
// this indicates the "normal" case of backprop across return statements, and is called
// from backpropAcrossReturn when the interesting cases like a one-to-many return are eliminated.
// Much of the logic in this function deals with error-returning functions
// in particular, this function is responsible for splitting returns into the cases:
// 1: Normal Return - all results yield consume triggers eventually enforcing their annotated/inferred nilability
// 2: Error Return - consume triggers are created based on the error contract. i.e., based on the nilabiity status of the error return expression
func computeAndConsumeResults(rootNode *RootAssertionNode, node *ast.ReturnStmt) error {
	// no matter what case the consumption of these returns ends up as - each must be computed
	for i := range node.Results {
		rootNode.AddComputation(node.Results[i])
	}

	if len(node.Results) == 0 {
		// check if this is a named return case -- where an empty *ast.ReturnStmt shows up even in functions that have
		// a nonzero number of results
		funcSigResults := rootNode.FuncDecl().Type.Results
		if funcSigResults != nil && len(funcSigResults.List) > 0 {
			// flatten return variables in the signature
			results := make([]ast.Expr, 0)
			for _, field := range funcSigResults.List {
				for _, retVariable := range field.Names {
					results = append(results, retVariable)
				}
			}

			// if the function has named error return variable, then handle specially using the error handling logic
			if util.FuncIsErrReturning(rootNode.FuncObj()) {
				handleErrorReturns(rootNode, node, results, true /* isNamedReturn */)
				return nil
			}

			// below is the normal handling for named return variables
			for i, retVariable := range results {
				consumer := &annotation.ConsumeTrigger{
					Annotation: &annotation.UseAsReturn{
						TriggerIfNonNil: &annotation.TriggerIfNonNil{
							Ann: annotation.RetKeyFromRetNum(
								rootNode.ObjectOf(rootNode.FuncNameIdent()).(*types.Func),
								i,
							)},
						IsNamedReturn: true,
						RetStmt:       node,
					},
					Expr:   retVariable,
					Guards: util.NoGuards(),
				}

				// default handling if retVariable is not a blank identifier (e.g., i *int)
				if !util.IsEmptyExpr(retVariable) {
					rootNode.AddConsumption(consumer)

					if rootNode.functionContext.isDepthOneFieldCheck() {
						rootNode.addConsumptionsForFieldsOfReturns(results[i], i)
					}
				} else {
					// special handling if retVariable is a blank identifier (e.g., _ *int)
					if !util.ExprBarsNilness(rootNode.Pass(), retVariable) {
						producer := &annotation.ProduceTrigger{
							Annotation: &annotation.BlankVarReturn{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}},
							Expr:       retVariable,
						}
						fullTrigger := annotation.FullTrigger{
							Producer: producer,
							Consumer: consumer,
						}
						rootNode.AddNewTriggers(fullTrigger)
					}
				}
			}
		}
		return nil
	}

	if len(node.Results) == 1 {
		if tupleResult, ok := rootNode.Pass().TypesInfo.Types[node.Results[0]].Type.(*types.Tuple); ok && tupleResult.Len() > 1 {
			// We're returning a multiply returning function, but one that couldn't be parsed in
			// backpropAcrossReturn (likely due to being anonymous)
			// there is no consumption we can compute here, so abort
			return nil
		}
	}

	n := util.FuncNumResults(rootNode.FuncObj())
	if len(node.Results) != n {
		return fmt.Errorf(
			"ERROR: function %s returns %d results where signature indicates it should return %d",
			rootNode.FuncDecl().Name.Name, len(node.Results), n,
		)
	}

	if util.FuncIsErrReturning(rootNode.FuncObj()) {
		handleErrorReturns(rootNode, node, node.Results, false /* isNamedReturn */)
		return nil
	}

	// we've excluded all abnormal cases - here, just really consume each result as a return value
	for i := range node.Results {
		rootNode.AddConsumption(&annotation.ConsumeTrigger{
			Annotation: &annotation.UseAsReturn{
				TriggerIfNonNil: &annotation.TriggerIfNonNil{
					Ann: annotation.RetKeyFromRetNum(
						rootNode.ObjectOf(rootNode.FuncNameIdent()).(*types.Func),
						i,
					)},
				RetStmt: node},
			Expr:   node.Results[i],
			Guards: util.NoGuards(),
		})

		if rootNode.functionContext.isDepthOneFieldCheck() {
			rootNode.addConsumptionsForFieldsOfReturns(node.Results[i], i)
		}
	}

	return nil
}

// isErrorReturnNil returns true if the error return is guaranteed to be nil, false otherwise
func isErrorReturnNil(rootNode *RootAssertionNode, errRet ast.Expr) bool {
	if ident, ok := errRet.(*ast.Ident); ok && rootNode.isNil(ident) {
		// error return is the literal nil
		return true
	}

	// check for false cases where error return value may be nil
	if util.IsEmptyExpr(errRet) {
		// error result is a blank named return ("_ error"), so it's always nil
		return true
	}

	if exprCallsKnownNilableErrFunc(errRet) {
		// error value is the return of a known nilable function
		return true
	}
	return false
}

// isErrorReturnNonnil returns true if the error return is guaranteed to be nonnil, false otherwise
func isErrorReturnNonnil(rootNode *RootAssertionNode, errRet ast.Expr) bool {
	t := rootNode.Pass().TypesInfo.TypeOf(errRet)
	if _, ok := AsTrustedFuncAction(errRet, rootNode.Pass()); ok || util.TypeAsDeeplyStruct(t) != nil {
		return true
	}

	return false
}

// handleErrorReturns handles the special case for error returning functions (n-th result of type `error` which guards at least one of the first n-1 non-error results).
// It generates consumers by applying the error contract:
// (1) if error return value = nil, create consumers for the non-error returns
// (2) if error return value = non-nil, create consumer for error return
// (3) if error return value = unknown, create consumers for all returns (error and non-error), and defer applying of the error contract when the nilability status is known, such as at `ProcessEntry`
//
// Note that `results` should be explicitly passed since `retStmt` of a named return will contain no results
func handleErrorReturns(rootNode *RootAssertionNode, retStmt *ast.ReturnStmt, results []ast.Expr, isNamedReturn bool) {
	errRetIndex := len(results) - 1
	errRetExpr := results[errRetIndex]     // n-th expression
	nonErrRetExpr := results[:errRetIndex] // n-1 expressions

	// check if the error return is at all guarding any nilable returns, such as pointers, maps, and slices
	for _, r := range nonErrRetExpr {
		if util.ExprBarsNilness(rootNode.Pass(), r) {
			// no need to further analyze and create triggers
			return
		}
	}

	if isErrorReturnNil(rootNode, errRetExpr) {
		// if error is the only return expression in the statement, then create a consumer for it, else create consumers for the non-error return expressions
		if len(nonErrRetExpr) == 0 {
			createConsumerForErrorReturn(rootNode, errRetExpr, errRetIndex, retStmt, isNamedReturn)
		} else {
			// create general return consume triggers for all n-1 (non-error) return expressions
			createGeneralReturnConsumers(rootNode, nonErrRetExpr, retStmt, isNamedReturn)
		}

		// TODO: handle struct init in the context of error return in a better way in a follow up diff
		if rootNode.functionContext.isDepthOneFieldCheck() {
			for i := range results {
				rootNode.addConsumptionsForFieldsOfReturns(results[i], i)
			}
		}
	} else if isErrorReturnNonnil(rootNode, errRetExpr) {
		// create consume trigger for only the error return
		createConsumerForErrorReturn(rootNode, errRetExpr, errRetIndex, retStmt, isNamedReturn)
	} else {
		// the nilability of error return is unknown, hence create special consume triggers for all returns
		createSpecialConsumersForAllReturns(rootNode, nonErrRetExpr, errRetExpr, errRetIndex, retStmt, isNamedReturn)

		// TODO: handle struct init in the context of error return in a better way in a follow up diff
		if rootNode.functionContext.isDepthOneFieldCheck() {
			for i := range results {
				rootNode.addConsumptionsForFieldsOfReturns(results[i], i)
			}
		}
	}
}

// createConsumerForErrorReturn creates a consumer for the error return enforcing it to be non-nil
func createConsumerForErrorReturn(rootNode *RootAssertionNode, errRetExpr ast.Expr, errRetIndex int, retStmt *ast.ReturnStmt, isNamedReturn bool) {
	rootNode.AddConsumption(&annotation.ConsumeTrigger{
		Annotation: &annotation.UseAsErrorResult{
			TriggerIfNonNil: &annotation.TriggerIfNonNil{
				Ann: annotation.RetKeyFromRetNum(rootNode.FuncObj(), errRetIndex),
			},
			IsNamedReturn: isNamedReturn,
			RetStmt:       retStmt,
		},
		Expr:   errRetExpr,
		Guards: util.NoGuards(),
	})
}

// createGeneralReturnConsumers creates general return consumers for the non-return expressions in the return statement
func createGeneralReturnConsumers(rootNode *RootAssertionNode, results []ast.Expr, retStmt *ast.ReturnStmt, isNamedReturn bool) {
	for i := range results {
		// don't do anything if the expression is a blank identifier ("_")
		if util.IsEmptyExpr(results[i]) {
			continue
		}
		rootNode.AddConsumption(&annotation.ConsumeTrigger{
			Annotation: &annotation.UseAsReturn{
				TriggerIfNonNil: &annotation.TriggerIfNonNil{
					Ann: annotation.RetKeyFromRetNum(rootNode.FuncObj(), i)},
				IsNamedReturn: isNamedReturn,
				RetStmt:       retStmt},
			Expr:   results[i],
			Guards: util.NoGuards(),
		})
	}
}

// createSpecialConsumersForAllReturns conservatively creates specially designed consumers for all return expressions, error and non-error
func createSpecialConsumersForAllReturns(rootNode *RootAssertionNode, nonErrRetExpr []ast.Expr, errRetExpr ast.Expr, errRetIndex int, retStmt *ast.ReturnStmt, isNamedReturn bool) {
	for i := range nonErrRetExpr {
		// don't do anything if the expression is a blank identifier ("_")
		if util.IsEmptyExpr(nonErrRetExpr[i]) {
			continue
		}
		consumer := &annotation.ConsumeTrigger{
			Annotation: &annotation.UseAsNonErrorRetDependentOnErrorRetNilability{
				TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: annotation.RetKeyFromRetNum(rootNode.FuncObj(), i)},
				RetStmt:         retStmt,
				IsNamedReturn:   isNamedReturn,
			},
			Expr:   nonErrRetExpr[i],
			Guards: util.NoGuards(),
		}
		rootNode.AddConsumption(consumer)
	}

	rootNode.AddConsumption(&annotation.ConsumeTrigger{
		Annotation: &annotation.UseAsErrorRetWithNilabilityUnknown{
			TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: annotation.RetKeyFromRetNum(rootNode.FuncObj(), errRetIndex)},
			RetStmt:         retStmt,
			IsNamedReturn:   isNamedReturn,
		},
		Expr:   errRetExpr,
		Guards: util.NoGuards(),
	})
}

func typeIsString(t types.Type) bool {
	if t, ok := t.(*types.Basic); ok && t.Kind() == types.String {
		return true
	}
	return false
}

// some expressions consume their subexpressions specifically when assigned to - for now, we are
// aware only of map indices written to as having this behavior
// exprAsConsumedByAssignment recognizes these cases, and returns the corresponding consumeTrigger
// if one is found, otherwise returning `nil, false`
// nilable(result 0)
func exprAsConsumedByAssignment(rootNode *RootAssertionNode, expr ast.Node) *annotation.ConsumeTrigger {
	if exprType, ok := expr.(*ast.IndexExpr); ok {
		t := util.TypeOf(rootNode.Pass(), exprType.X)
		if util.TypeIsDeeplyMap(t) {
			return &annotation.ConsumeTrigger{
				Annotation: &annotation.MapWrittenTo{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}},
				Expr:       exprType.X,
				Guards:     util.NoGuards(),
			}
		}
	}
	return nil
}

// exprAsAssignmentConsumer is similar to parseExprAsProducer, but tries to parse the passed
// expression as a _consumer_ instead of as a _producer_. The simplest illustrative example of
// this is when a field read expression is passed as `expr` - meaning a field is being assigned
// to - such as `x.f = y`. This will result in an `annotation.FldAssign` being returned, which
// will serve to produce an error if that field is non-nil and a nilable value flows into it
// through the assignment triggering this call to exprAsAssignmentConsumer.
// other notable cases include passing a send expression here (which is why we take an `ast.Node`
// not `ast.Expr`, and various "deep" assignments such as to an index of an object
// nilable(result 0)
func exprAsAssignmentConsumer(rootNode *RootAssertionNode, expr ast.Node, exprRHS ast.Node) (annotation.ConsumingAnnotationTrigger, error) {
	if expr, ok := expr.(ast.Expr); ok && util.IsEmptyExpr(expr) {
		return nil, nil
	}

	handleAssignmentToIdent := func(ident *ast.Ident) annotation.ConsumingAnnotationTrigger {
		v := rootNode.ObjectOf(ident).(*types.Var)
		if annotation.VarIsGlobal(v) {
			// we've found an assignment to a global
			return &annotation.GlobalVarAssign{
				TriggerIfNonNil: &annotation.TriggerIfNonNil{
					Ann: &annotation.GlobalVarAnnotationKey{
						VarDecl: v,
					}}}
		}
		return nil
	}

	handleDeepAssignmentToIdent :=
		func(ident *ast.Ident) annotation.ConsumingAnnotationTrigger {
			funcObj := rootNode.FuncObj()
			varObj := rootNode.ObjectOf(ident).(*types.Var)
			if util.TypeIsDeep(varObj.Type()) {
				if annotation.VarIsParam(funcObj, varObj) {
					// we've found an assignment to a parameter with deep type - have to check its deep annotation!
					paramKey := annotation.ParamKeyFromName(funcObj, varObj)

					// but first - if it's a variadic parameter then its "deep" annotation is really just
					// its shallow annotation:
					if annotation.VarIsVariadicParam(funcObj, varObj) {
						return &annotation.VariadicParamAssignDeep{
							TriggerIfNonNil: &annotation.TriggerIfNonNil{
								Ann: paramKey}}
					}

					// we've concluded it's not a variadic parameter
					return &annotation.ParamAssignDeep{
						TriggerIfDeepNonNil: &annotation.TriggerIfDeepNonNil{
							Ann: paramKey}}
				}
				if annotation.VarIsGlobal(varObj) {
					// we've found an assignment to a global var with deep type - have to check its deep annotation!
					return &annotation.GlobalVarAssignDeep{
						TriggerIfDeepNonNil: &annotation.TriggerIfDeepNonNil{
							Ann: &annotation.GlobalVarAnnotationKey{
								VarDecl: varObj,
							}}}
				}
			}
			return nil
		}

	handleDeepAssignmentToExpr :=
		func(expr ast.Expr) (annotation.ConsumingAnnotationTrigger, error) {

			switch expr := expr.(type) {
			case *ast.Ident:
				if consumer := handleDeepAssignmentToIdent(expr); consumer != nil {
					return consumer, nil
				}
			case *ast.SelectorExpr:
				if rootNode.isPkgName(expr.X) {
					if consumer := handleDeepAssignmentToIdent(expr.Sel); consumer != nil {
						return consumer, nil
					}
				}

				// this is an assignment to an index of a field
				fldObj := rootNode.ObjectOf(expr.Sel).(*types.Var)
				if fldObj.IsField() && util.TypeIsDeep(fldObj.Type()) {
					return &annotation.FieldAssignDeep{
						TriggerIfDeepNonNil: &annotation.TriggerIfDeepNonNil{
							Ann: &annotation.FieldAnnotationKey{FieldDecl: fldObj},
						},
					}, nil
				}
			case *ast.CallExpr:
				// check if this is a call to a function by name
				if ident := util.FuncIdentFromCallExpr(expr); ident != nil {
					obj := rootNode.ObjectOf(ident).(*types.Func)
					if obj.Type().(*types.Signature).Results().Len() != 1 {
						return nil, errors.New("multiply returning function treated as assignment consumer")
					}
					return &annotation.FuncRetAssignDeep{
						TriggerIfDeepNonNil: &annotation.TriggerIfDeepNonNil{
							Ann: annotation.RetKeyFromRetNum(obj, 0),
						},
					}, nil
				}
			case *ast.IndexExpr:
				return exprAsAssignmentConsumer(rootNode, expr.X, exprRHS)
			}

			nameAsDeepTrigger := func(name *types.TypeName) *annotation.TriggerIfDeepNonNil {
				return &annotation.TriggerIfDeepNonNil{Ann: &annotation.TypeNameAnnotationKey{TypeDecl: name}}
			}

			exprType := rootNode.Pass().TypesInfo.Types[expr].Type

			if named, ok := exprType.(*types.Named); ok {
				// Calling Underlying on [types.Named] will always return the unnamed type, so we
				// do not have to recursively "unwrap" the [types.Named].
				// See [https://github.com/golang/example/tree/master/gotypes#named-types].
				switch named.Underlying().(type) {
				case *types.Slice:
					return &annotation.SliceAssign{TriggerIfDeepNonNil: nameAsDeepTrigger(named.Obj())}, nil
				case *types.Array:
					return &annotation.ArrayAssign{TriggerIfDeepNonNil: nameAsDeepTrigger(named.Obj())}, nil
				case *types.Map:
					return &annotation.MapAssign{TriggerIfDeepNonNil: nameAsDeepTrigger(named.Obj())}, nil
				case *types.Pointer:
					return &annotation.PtrAssign{TriggerIfDeepNonNil: nameAsDeepTrigger(named.Obj())}, nil
				case *types.Chan:
					return &annotation.ChanSend{TriggerIfDeepNonNil: nameAsDeepTrigger(named.Obj())}, nil
				}
			}

			// at this point - the value being deeply assigned to is of deep type but is not linked
			// to an annotation site, for example, local variables.
			// so we introspect on its type alone

			if !annotation.TypeIsDeepDefaultNilable(exprType) {
				if ident, ok := expr.(*ast.Ident); ok {
					varObj := rootNode.ObjectOf(ident).(*types.Var)
					return &annotation.LocalVarAssignDeep{
						ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{},
						LocalVar:                varObj,
					}, nil
				}
				return &annotation.DeepAssignPrimitive{ConsumeTriggerTautology: &annotation.ConsumeTriggerTautology{}}, nil
			}
			return nil, nil
		}

	switch expr := expr.(type) {
	case *ast.Ident:
		funcObj := rootNode.FuncObj()
		varObj := rootNode.ObjectOf(expr).(*types.Var)
		// This block checks if the rhs of the assignment is the builtin append function for slices
		if call, ok := exprRHS.(*ast.CallExpr); ok && util.TypeIsSlice(varObj.Type()) {
			if fun, ok := call.Fun.(*ast.Ident); ok && fun.Name == BuiltinAppend {
				if annotation.VarIsParam(funcObj, varObj) {
					// If there is a deep assignment to a slice using append method
					return handleDeepAssignmentToExpr(expr)
				}
			}
		}
		if consumer := handleAssignmentToIdent(expr); consumer != nil {
			return consumer, nil
		}

	case *ast.SelectorExpr:
		if rootNode.isPkgName(expr.X) {
			if consumer := handleAssignmentToIdent(expr.Sel); consumer != nil {
				return consumer, nil
			}
		}

		if rootNode.functionContext.isDepthOneFieldCheck() {
			if head := util.GetSelectorExprHeadIdent(expr); head != nil {
				if obj, ok := rootNode.ObjectOf(head).(*types.Var); ok {
					if !annotation.VarIsGlobal(obj) {
						// If field access for a variable that is not a global var we rely on default field nilability based on
						// escape analysis, and thus we do not create any triggers for field assignments.
						// For global variables we still maintain the previous behaviour. Thus do not return anything.
						// For a global variable g, `g.f = nil` would result in a const nil field assignment trigger.
						// However, for other type of variables `p.f = nil` would result into an escape trigger only if the
						// field escapes as per the definition of field escape in our analysis.
						return nil, nil
					}
				}
			}
		}

		return &annotation.FldAssign{
			TriggerIfNonNil: &annotation.TriggerIfNonNil{
				Ann: &annotation.FieldAnnotationKey{
					FieldDecl: rootNode.ObjectOf(expr.Sel).(*types.Var),
				},
			},
		}, nil
	case *ast.StarExpr:
		return handleDeepAssignmentToExpr(expr.X)
	case *ast.IndexExpr:
		return handleDeepAssignmentToExpr(expr.X)
	case *ast.SendStmt:
		return handleDeepAssignmentToExpr(expr.Chan)
	}

	// no recognized source of deep nilability consumption
	return nil, nil
}

func composeRootFuncs(f1, f2 RootFunc) RootFunc {
	return func(node *RootAssertionNode) {
		f1(node)
		f2(node)
	}
}

// This takes a cfg, and generates the information we need from it:
//  1. its set of blocks, but with a "return" block appended that's a successor of every block that returns
//     we need this as an index of where to start our backpropagation
//  2. for conditional branching blocks, add "pre-processing" to insert nil-checks corresponding to
//     their branch condition. If blocks[i] is a conditional, then preprocessing[i].trueBranchFunc will
//     be a function to insert the true result of the check and preprocessing[i].falseBranchFunc will
//     be a function to insert the false result
//
// The `richCheckBlocks` that it takes represents, for each block, which richCheckEffects hold at
// the end of that block
//
// postcondition - length of two return slices is equal
//
// nonnil(result 0, result 1)
func blocksAndPreprocessingFromCFG(
	pass *analysis.Pass, graph *cfg.CFG, richCheckBlocks [][]RichCheckEffect) (
	[]*cfg.Block, []*preprocessPair) {

	numBlocks := len(graph.Blocks)
	// add an empty "return" block
	blocks := append(graph.Blocks, &cfg.Block{
		Nodes: nil,
		Succs: nil,
		Index: int32(numBlocks),
		Live:  true,
	})
	// link all returning blocks to the "return" block
	for i := 0; i < numBlocks; i++ {
		if blocks[i].Return() != nil {
			blocks[i].Succs = append(blocks[i].Succs, blocks[numBlocks])
		}
	}

	// generate pre-processing
	preprocessing := make([]*preprocessPair, len(blocks))

	for i := range richCheckBlocks {
		if cond := getConditional(blocks[i]); cond != nil {
			// blocks[i] is a conditional

			// so add nil check productions to each successor
			// this is where the assumption that True Branch = Succs[0], False Branch = Succs[1] shows up
			trueNilCheck, falseNilCheck, isNoop := AddNilCheck(pass, cond)
			if !isNoop {
				// we've discovered that this is a nil check
				preprocessing[i] = &preprocessPair{
					trueBranchFunc:  trueNilCheck,
					falseBranchFunc: falseNilCheck,
				}
			}

			// now check for RichCheckEffects triggered by this conditional
			for _, effect := range richCheckBlocks[i] {
				if effect.isTriggeredBy(cond) {
					if preprocessing[i] == nil {
						preprocessing[i] = &preprocessPair{
							trueBranchFunc:  effect.effectIfTrue,
							falseBranchFunc: effect.effectIfFalse,
						}
					} else {
						preprocessing[i].trueBranchFunc = composeRootFuncs(
							preprocessing[i].trueBranchFunc, effect.effectIfTrue)
						preprocessing[i].falseBranchFunc = composeRootFuncs(
							preprocessing[i].falseBranchFunc, effect.effectIfFalse)
					}
				}
			}
		} else if rangeExpr := getRangeExpr(blocks[i]); rangeExpr != nil {
			blockSuccs := blocks[i].Succs

			// blocks[i] is a precursor to a range loop
			if blockSuccs == nil || len(blocks[i].Succs) != 1 {
				panic("expected shape of CFG violated: block that ends with range has " +
					"non-singular successors")
			}

			// this is the actual range loop node with two successors
			rangeLoop := blockSuccs[0]
			if len(rangeLoop.Nodes) != 0 {
				panic("expected shape of CFG violated: block presumed to be a range loop has " +
					"a nonzero number of nodes")
			}
			if len(rangeLoop.Succs) != 2 {
				panic("expected shape of CFG violated: block presumed to be a range loop has " +
					"a number of successors other than 2")
			}

			preprocessing[rangeLoop.Index] =
				&preprocessPair{
					trueBranchFunc: func(node *RootAssertionNode) { // producing ranging expression as nonnil
						node.AddProduction(&annotation.ProduceTrigger{
							Annotation: &annotation.RangeOver{ProduceTriggerNever: &annotation.ProduceTriggerNever{}},
							Expr:       rangeExpr,
						})
					},
					falseBranchFunc: func(*RootAssertionNode) {}, // no-op
				}
		}
	}

	return blocks, preprocessing
}

// nonnil(idents, result 0)
func toExprSlice(idents []*ast.Ident) []ast.Expr {
	exprs := make([]ast.Expr, len(idents))
	for i := range idents {
		exprs[i] = idents[i]
	}
	return exprs
}

func exprAsDeepProducer(rootNode *RootAssertionNode, expr ast.Expr) annotation.ProducingAnnotationTrigger {
	_, parsedExpr := rootNode.ParseExprAsProducer(expr, true)
	if len(parsedExpr) > 1 {
		panic("multiply returning function passed where a deep producer is expected - tuple types are not deep")
	}
	if len(parsedExpr) == 0 || !parsedExpr[0].IsDeep() || parsedExpr[0].GetDeep() == nil {
		// the expr is not deeply nilable
		return &annotation.ProduceTriggerNever{}
	}
	return parsedExpr[0].GetDeep().Annotation
}

// CheckGuardOnFullTrigger gives guarding its intended semantics:
// if a full trigger would be created with a guarded producer but
// not a guarded consumer, then the production as written in the
// trigger is ignored and replaced with an always-nilable-producing
// instance of annotation.GuardMissing
func CheckGuardOnFullTrigger(trigger annotation.FullTrigger) annotation.FullTrigger {
	if trigger.Producer.Annotation.NeedsGuardMatch() && !trigger.Consumer.GuardMatched {
		return annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{
				Annotation: &annotation.GuardMissing{
					ProduceTriggerTautology: &annotation.ProduceTriggerTautology{},
					OldAnnotation:           trigger.Producer.Annotation,
				},
				Expr: trigger.Producer.Expr,
			},
			Consumer: trigger.Consumer,
		}
	}
	return trigger
}
