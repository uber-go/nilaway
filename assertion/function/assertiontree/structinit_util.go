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
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion/structfield"
	"go.uber.org/nilaway/util"
)

// addProductionsForAssignmentFields adds production for each produce trigger in fieldProducers.
// fieldProducers contain all the field producers due to the rhs of the assignment.
// lhsVal is the assigned lhs expression
// If the assignment is lhsVal = f(), then the expression for the field producer is `lhsVal.field` for
// the corresponding field
func (r *RootAssertionNode) addProductionsForAssignmentFields(fieldProducers []*annotation.ProduceTrigger, lhsVal ast.Expr) {
	structType := r.Pass().TypesInfo.TypeOf(lhsVal)

	if structType := util.TypeAsDeeplyStruct(structType); structType != nil {
		for i, fieldProducer := range fieldProducers {
			if fieldProducer == nil {
				continue
			}

			selExpr := r.getSelectorExpr(structType.Field(i), lhsVal)

			r.AddProduction(&annotation.ProduceTrigger{
				Annotation: fieldProducer.Annotation,
				Expr:       selExpr,
			})

		}
	}

}

// addConsumptionsForFieldsOfReturns adds consumptions for each field of retNum-th return of a function.
// This creates consumptions in one of the following cases
// 1. if the return expression is chain of field accesses
// For example, an ident `x` is also a chain of selector expression, thus it will consume all `x.field` for fields
// that have nilable type
// 2. If the expression gives field producers, we create full triggers for those producers with the
// consumers of field return keys
func (r *RootAssertionNode) addConsumptionsForFieldsOfReturns(retExpr ast.Expr, retNum int) {
	fdecl := r.FuncObj()

	// fdecl Type() is always a *Signature
	res := fdecl.Type().(*types.Signature).Results().At(retNum)

	if resType := util.TypeAsDeeplyStruct(res.Type()); resType != nil {
		numFields := resType.NumFields()

		// For field selection chains we add the consumptions for fields by creating artificial selector expression
		if util.IsFieldSelectorChain(retExpr) {
			for fieldID := 0; fieldID < numFields; fieldID++ {
				fieldDecl := resType.Field(fieldID)

				if util.TypeBarsNilness(fieldDecl.Type()) {
					// We do not create field triggers for types that are not nilable
					continue
				}

				retKey := annotation.NewRetFldAnnKey(fdecl, retNum, fieldDecl)

				// create an artificial selector expression ast node
				selExpr := r.getSelectorExpr(fieldDecl, retExpr)

				consumer := annotation.GetRetFldConsumer(retKey, selExpr)
				r.AddConsumption(consumer)

				// Also add escape consumer
				escapeConsumer := annotation.GetEscapeFldConsumer(annotation.NewEscapeFldAnnKey(fieldDecl), selExpr)
				r.AddConsumption(escapeConsumer)
			}
			return
		}

		// For the expressions that return field producers we create full triggers
		_, producer := r.ParseExprAsProducer(retExpr, true)

		if producer == nil {
			return
		}

		for fieldIdx, fieldProducer := range producer[0].GetFieldProducers() {
			if fieldProducer == nil {
				// field producer is nil for fields that have non-nilable type
				continue
			}

			fieldDecl := resType.Field(fieldIdx)
			retKey := annotation.NewRetFldAnnKey(fdecl, retNum, fieldDecl)

			consumer := annotation.GetRetFldConsumer(retKey, retExpr)
			r.AddNewTriggers(annotation.FullTrigger{
				Producer: fieldProducer,
				Consumer: consumer,
			})

			// Also add escape consumer
			r.addEscapeFullTrigger(retExpr, resType, fieldIdx, fieldProducer)
		}
		return
	}
}

// getFieldProducersForFuncReturns creates and returns producers for nilable-typed fields of the struct at return index retNum.
// The method returns nil if the return at index retNum is not a struct. calledFuncDecl is the declaration of the function that
// is called.
func (r *RootAssertionNode) getFieldProducersForFuncReturns(calledFuncDecl *types.Func, retNum int) []*annotation.ProduceTrigger {
	// calledFuncDecl Type() is always *Signature
	result := calledFuncDecl.Type().(*types.Signature).Results().At(retNum)

	if resType := util.TypeAsDeeplyStruct(result.Type()); resType != nil {
		numFields := resType.NumFields()
		producers := make([]*annotation.ProduceTrigger, numFields)

		for fieldID := 0; fieldID < numFields; fieldID++ {
			fieldDecl := resType.Field(fieldID)

			if util.TypeBarsNilness(fieldDecl.Type()) {
				// We do not create field triggers for types that are not nilable
				return nil
			}

			retKey := annotation.NewRetFldAnnKey(calledFuncDecl, retNum, fieldDecl)

			fieldProducer := &annotation.ProduceTrigger{
				Annotation: &annotation.FldReturn{
					TriggerIfNilable: &annotation.TriggerIfNilable{Ann: retKey},
				},
			}

			producers[fieldID] = fieldProducer
		}
		return producers
	}
	return nil
}

// addProductionsForParamFields adds productions for fields of params and receivers. It is called while processing entry
// block during backprop
func (r *RootAssertionNode) addProductionsForParamFields(node AssertionNode, builtExpr ast.Expr) {
	nodes := make([]AssertionNode, len(node.Children()))
	// we need to make copy of the child nodes as they might get deleted when the productions are matched.
	// Each such child node represents a consumption on an argument-field pair s.f, and as we generate the
	// corresponding production in `addProductionsForParamFieldNode` (and call `r.AddProduction`
	// within that function), the two will get matched into a full trigger, which removes the consumption trigger
	// from its current position in the assertion tree.
	copy(nodes, node.Children())

	for _, node := range nodes {
		if fldNode, ok := node.(*fldAssertionNode); ok {
			if util.TypeBarsNilness(fldNode.decl.Type()) {
				// We do not add production for types that are not nilable
				continue
			}
			selExpr := r.getSelectorExpr(fldNode.decl, builtExpr)
			r.addProductionsForParamFieldNode(selExpr, fldNode)
		}

	}
}

// addProductionForVarFieldNode adds productions for fields of a struct defined as a local variable represented by varAssertionNode
func (r *RootAssertionNode) addProductionForVarFieldNode(varNode *varAssertionNode, varAstExpr ast.Expr) {
	for _, child := range varNode.Children() {
		if fldNode, ok := child.(*fldAssertionNode); ok {
			if util.TypeBarsNilness(fldNode.decl.Type()) {
				continue
			}
			selExpr := r.getSelectorExpr(fldNode.decl, varAstExpr)
			if varAstExpr == selExpr.X {
				r.AddProduction(
					&annotation.ProduceTrigger{
						Annotation: &annotation.UnassignedFld{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}},
						Expr:       selExpr,
					})
			}
		}
	}
}

// addProductionsForParamFieldNode adds productions for fields of params and receiver at the entry node for a specific
// fldAssertionNode
func (r *RootAssertionNode) addProductionsForParamFieldNode(selExpr *ast.SelectorExpr, node *fldAssertionNode) {
	funcDecl := r.functionContext.funcDecl
	funcObj := r.FuncObj()
	index := 0
	for _, paramSublist := range funcDecl.Type.Params.List {
		for _, paramName := range paramSublist.Names {
			if paramName == selExpr.X {
				r.AddProduction(
					&annotation.ProduceTrigger{
						Annotation: &annotation.ParamFldRead{
							TriggerIfNilable: &annotation.TriggerIfNilable{
								Ann: annotation.NewParamFldAnnKey(funcObj, index, node.decl)}},
						Expr: selExpr,
					})
			}
			index++
		}
	}

	if funcDecl.Recv == nil {
		return
	}

	// If funcDecl is a method then add production for fields of receivers
	for _, recvSublist := range funcDecl.Recv.List {
		for _, recvName := range recvSublist.Names {
			if recvName != selExpr.X {
				continue
			}
			r.AddProduction(
				&annotation.ProduceTrigger{
					Annotation: &annotation.ParamFldRead{
						TriggerIfNilable: &annotation.TriggerIfNilable{
							Ann: annotation.NewParamFldAnnKey(funcObj, annotation.ReceiverParamIndex, node.decl)}},
					Expr: selExpr,
				})
		}
	}
}

// addConsumptionsForArgAndReceiverFields adds consumptions for fields of receivers and arguments of a function call. It takes
// the call expression and adds consumptions for each param and receiver by calling addConsumptionsForArgFields and
// addConsumptionsForReceiverFields respectively
func (r *RootAssertionNode) addConsumptionsForArgAndReceiverFields(call *ast.CallExpr, funcIdent *ast.Ident) {
	result := r.Pass().ResultOf[structfield.Analyzer].(structfield.Result)

	r.addConsumptionsForArgFields(call, funcIdent, result.Context)

	// In case we are dealing with a method call or a function call in another package (and not dot-imported)
	r.addConsumptionsForReceiverFields(call, result.Context)
}

// addConsumptionsForReceiverFields adds consumptions for receiver fields at function call
func (r *RootAssertionNode) addConsumptionsForReceiverFields(call *ast.CallExpr, fieldContext *structfield.FieldContext) {
	if functionExpr, ok := call.Fun.(*ast.SelectorExpr); ok {
		if funcObj, ok := r.Pass().TypesInfo.ObjectOf(functionExpr.Sel).(*types.Func); ok {
			if funcObj.Type().(*types.Signature).Recv() != nil {
				// In case we are dealing with a method call add consumers for fields of receiver
				r.addConsumptionsForArgFieldsAtIndex(functionExpr.X, funcObj, annotation.ReceiverParamIndex, fieldContext)
			}
		}
	}
}

// addConsumptionsForReceiverFields adds consumptions for param fields at function call
func (r *RootAssertionNode) addConsumptionsForArgFields(call *ast.CallExpr, funcName *ast.Ident, fieldContext *structfield.FieldContext) {
	if funcObj, ok := r.Pass().TypesInfo.ObjectOf(funcName).(*types.Func); ok {
		for paramID, param := range call.Args {
			r.addConsumptionsForArgFieldsAtIndex(param, funcObj, paramID, fieldContext)
		}
	}
}

// addConsumptionsForArgFieldsAtIndex adds consumptions for fields of argument at index argIdx of a function call. Thus, it captures the
// nilability flow to the function through the fields of param. in case, argIdx is equal to ReceiverParamIndex then the consumptions
// are created for the receiver
func (r *RootAssertionNode) addConsumptionsForArgFieldsAtIndex(arg ast.Expr, funcObj *types.Func, argIdx int, fieldContext *structfield.FieldContext) {
	var param *types.Var
	if argIdx == annotation.ReceiverParamIndex {
		param = funcObj.Type().(*types.Signature).Recv()
	} else {
		param = util.GetParamObjFromIndex(funcObj, argIdx)
	}

	if paramType := util.TypeAsDeeplyStruct(param.Type()); paramType != nil {
		numFields := paramType.NumFields()

		// For field selection chains we add the consumptions for fields by creating artificial selector expression
		if util.IsFieldSelectorChain(arg) {
			for fieldIdx := 0; fieldIdx < numFields; fieldIdx++ {
				fieldDecl := paramType.Field(fieldIdx)

				if util.TypeBarsNilness(fieldDecl.Type()) {
					continue
				}

				selExpr := r.getSelectorExpr(fieldDecl, arg)

				// only create this trigger if the field was found to be accessed in the function given by `funcObj`
				if fieldContext.IsFieldUsedInFunc(funcObj, argIdx, fieldDecl.Name(), structfield.Accessed) {
					paramFieldKey := annotation.NewParamFldAnnKey(funcObj, argIdx, fieldDecl)
					r.AddConsumption(
						&annotation.ConsumeTrigger{
							Annotation: &annotation.ArgFldPass{
								TriggerIfNonNil: &annotation.TriggerIfNonNil{
									Ann: paramFieldKey,
								},
							},
							Guards: util.NoGuards(),
							Expr:   selExpr})
				}

				// Also add escape consumer
				escapeConsumer := annotation.GetEscapeFldConsumer(annotation.NewEscapeFldAnnKey(fieldDecl), selExpr)
				r.AddConsumption(escapeConsumer)
			}
			return
		}

		// For the argument expressions that produce field producers we create full triggers
		// e.g., f(&A{...}) or f(g())
		_, producers := r.ParseExprAsProducer(arg, true)

		if len(producers) == 0 {
			return
		}

		// TODO: We only handle first producer of the expressions

		for fieldIdx, fieldProducer := range producers[0].GetFieldProducers() {
			if fieldProducer == nil {
				// for fields that have non-nilable type we don't do anything
				continue
			}

			fieldDecl := paramType.Field(fieldIdx)
			paramFieldKey := annotation.NewParamFldAnnKey(funcObj, argIdx, fieldDecl)

			consumer := annotation.GetParamFldConsumer(paramFieldKey, arg)
			r.AddNewTriggers(annotation.FullTrigger{
				Producer: fieldProducer,
				Consumer: consumer,
			})

			// add escape trigger
			// TODO: Do not call this for list of special functions
			r.addEscapeFullTrigger(arg, paramType, fieldIdx, fieldProducer)
		}
	}
}

// addProductionForFuncCallArgAndReceiverFields is called while consuming a function call. Productions for fields of params
// and receivers are added to track the effect the function call can have on the fields.
func (r *RootAssertionNode) addProductionForFuncCallArgAndReceiverFields(call *ast.CallExpr, funcIdent *ast.Ident) {
	result := r.Pass().ResultOf[structfield.Analyzer].(structfield.Result)

	r.addProductionForFuncCallArgFields(funcIdent, call, result.Context)

	// In case we are dealing with a method call or call to other package
	r.addProductionForFuncCallReceiverFields(call, result.Context)
}

// addProductionForFuncCallReceiverFields adds productions for fields of receivers are added to track the
// effect the function call can have on the fields.
func (r *RootAssertionNode) addProductionForFuncCallReceiverFields(call *ast.CallExpr, fieldContext *structfield.FieldContext) {
	if funcName, ok := call.Fun.(*ast.SelectorExpr); ok {
		functionName := funcName.Sel

		if funcObj, ok := r.Pass().TypesInfo.ObjectOf(functionName).(*types.Func); ok {
			if funcObj.Type().(*types.Signature).Recv() != nil {
				receiver := funcName.X
				r.addProductionForFuncCallArgFieldsAtIndex(receiver, funcObj, annotation.ReceiverParamIndex, fieldContext)
			}
		}
	}
}

// addProductionForFuncCallArgFields adds productions for fields of params are added to track the
// effect the function call can have on the fields.
func (r *RootAssertionNode) addProductionForFuncCallArgFields(funcName *ast.Ident, call *ast.CallExpr, fieldContext *structfield.FieldContext) {
	if funcObj, ok := r.Pass().TypesInfo.ObjectOf(funcName).(*types.Func); ok {
		for paramIdx := range call.Args {
			param := call.Args[paramIdx]
			r.addProductionForFuncCallArgFieldsAtIndex(param, funcObj, paramIdx, fieldContext)
		}
	}
}

// addProductionForFuncCallArgFieldsAtIndex is called from addProductionForFuncCallArgFields for individual
// argument arg. Productions are added for fields of arguments of functions to track the side effect
// on the fields due to the function call.
func (r *RootAssertionNode) addProductionForFuncCallArgFieldsAtIndex(arg ast.Expr, methodType *types.Func, argIdx int, fieldContext *structfield.FieldContext) {
	var param *types.Var
	if argIdx == annotation.ReceiverParamIndex {
		param = methodType.Type().(*types.Signature).Recv()
	} else {
		param = util.GetParamObjFromIndex(methodType, argIdx)
	}

	if structType := util.TypeAsDeeplyStruct(param.Type()); structType != nil {
		numFields := structType.NumFields()

		for fieldID := 0; fieldID < numFields; fieldID++ {
			fieldDecl := structType.Field(fieldID)
			if !fieldContext.IsFieldUsedInFunc(methodType, argIdx, fieldDecl.Name(), structfield.Assigned) {
				continue
			}

			paramFieldKey, selExpr := r.getParamFieldKey(arg, methodType, argIdx, structType, fieldID)

			if paramFieldKey != nil {
				r.AddProduction(
					&annotation.ProduceTrigger{
						Annotation: &annotation.ParamFldRead{
							TriggerIfNilable: &annotation.TriggerIfNilable{
								Ann: paramFieldKey,
							},
						},
						Expr: selExpr})
			}
		}
	}
}

// addConsumptionsForFieldsOfParams is called at the return statement of the function during backprop. This consumption captures the
// possible side effect on the fields of params that the function call can have
func (r *RootAssertionNode) addConsumptionsForFieldsOfParams() {
	result := r.Pass().ResultOf[structfield.Analyzer].(structfield.Result)

	funcSig := r.FuncObj().Type().(*types.Signature)
	for i := 0; i < funcSig.Params().Len(); i++ {
		param := funcSig.Params().At(i)

		paramNode := util.GetFunctionParamNode(r.FuncDecl(), param)
		if paramNode != nil {
			r.addConsumptionsForFieldsOfParam(param, paramNode, i, result.Context)
		}
	}

	// `funcSig.Recv() != nil` actually implies `r.FuncDecl().Recv != nil` since they are referring
	// to the same function represented in two different systems (i.e., language objects and ASTs).
	// However, NilAway does not know this correlation, hence this redundant check.
	if funcSig.Recv() != nil && r.FuncDecl().Recv != nil {
		// The [language specs] require that the receiver must be a single non-variadic parameter.
		// However, they are represented as a regular parameter _list_ in the AST. Here, we can
		// safely use the first element of the list.
		// [language specs]: https://go.dev/ref/spec#Method_declarations
		receivers := r.FuncDecl().Recv.List[0].Names

		// The length of receivers can only be 0 (unnamed receiver) or 1 (named receiver).
		// We only need to handle the named case if it is not an empty (`_`) receiver.
		if len(receivers) != 0 && !util.IsEmptyExpr(receivers[0]) {
			r.addConsumptionsForFieldsOfParam(funcSig.Recv(), receivers[0], annotation.ReceiverParamIndex, result.Context)
		}
	}
}

// addConsumptionsForFieldsOfParam is called by addConsumptionsForFieldsOfParams for parameter at index paramIdx
func (r *RootAssertionNode) addConsumptionsForFieldsOfParam(param *types.Var, paramNode ast.Expr, paramIdx int, fieldContext *structfield.FieldContext) {
	if resType := util.TypeAsDeeplyStruct(param.Type()); resType != nil {
		numFields := resType.NumFields()
		for fieldIdx := 0; fieldIdx < numFields; fieldIdx++ {
			fieldDecl := resType.Field(fieldIdx)
			if !fieldContext.IsFieldUsedInFunc(r.FuncObj(), paramIdx, fieldDecl.Name(), structfield.Assigned) {
				continue
			}

			paramFieldKey, selExpr := r.getParamFieldKey(paramNode, r.FuncObj(), paramIdx, resType, fieldIdx)

			if paramFieldKey != nil {
				r.AddConsumption(&annotation.ConsumeTrigger{
					Annotation: &annotation.ArgFldPass{
						TriggerIfNonNil: &annotation.TriggerIfNonNil{
							Ann: paramFieldKey},
						IsPassed: true,
					},
					Expr:   selExpr,
					Guards: util.NoGuards(),
				})
			}
		}
	}
}

// getParamFieldKey returns param field annotation key and selector expression that selects the field of expression arg
func (r *RootAssertionNode) getParamFieldKey(arg ast.Expr, methodType *types.Func, argIdx int, structType *types.Struct, fieldID int) (annotation.Key, *ast.SelectorExpr) {
	fieldDecl := structType.Field(fieldID)

	if util.TypeBarsNilness(fieldDecl.Type()) {
		return nil, nil
	}
	selExpr := r.getSelectorExpr(fieldDecl, arg)

	paramFieldKey := &annotation.ParamFieldAnnotationKey{
		FuncDecl:             methodType,
		ParamNum:             argIdx,
		FieldDecl:            fieldDecl,
		IsTrackingSideEffect: true,
	}
	return paramFieldKey, selExpr
}

// addEscapeFullTrigger adds escape full trigger for the field with fieldIdx
func (r *RootAssertionNode) addEscapeFullTrigger(expr ast.Expr, structType *types.Struct, fieldIdx int, fieldProducer *annotation.ProduceTrigger) {

	escapeConsumer := annotation.GetEscapeFldConsumer(annotation.NewEscapeFldAnnKey(structType.Field(fieldIdx)), expr)
	r.AddNewTriggers(annotation.FullTrigger{
		Producer: fieldProducer,
		Consumer: escapeConsumer,
	})
}

// getSelectorExpr gets the declaring ident for field fieldDecl, and returns the selector expression
func (r *RootAssertionNode) getSelectorExpr(fieldDecl *types.Var, fieldOf ast.Expr) *ast.SelectorExpr {
	fieldIdent := r.GetDeclaringIdent(fieldDecl)
	return r.functionContext.getCachedSelectorExpr(fieldDecl, fieldOf, fieldIdent)
}
