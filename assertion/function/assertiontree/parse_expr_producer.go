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
	"go/token"
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion/function/producer"
	"go.uber.org/nilaway/util"
)

// ParseExprAsProducer takes an expression, and determines whether it is `trackable` - i.e. if it is a
// linear sequence of variable reads, field reads, indexes by `stable` expressions, and function calls
// with `stable` arguments. An expression is `stable` if our static analysis assume that multiple
// syntactic occurrences of it will always yield the same value - i.e. they are assumed to be constant.
//
// This function and the cases in which it returns a sequence of nodes serve as our internal
// definition of `trackable`, and similarly the function isStable below serves as our internal
// definition of `stable`.
//
// # Of its two return values, shallowSeq and producer, only one will be non-nil
//
// In the case that expr is trackable, shallowSeq will be non-nil, and contain the AssertionNodes
// without pointers between them that characterize the give expression.
//
// In the case that expr is not trackable, shallowSeq will be nil. If expr is known to be non-nil
// (e.g. a non-nil constant) then producer will be nil too, but otherwise it will be a slice of
// produceTriggers encapsulating the conditions under which expr could be nil. The slice will have
// length 1 for every expr except multiply returning functions, for which it will have length equal
// to the number of returns of that function.
//
// The function also takes a flag doNotTrack which, if set to true, always treats the expr
// as non-trackable and gives its producer trigger or nil if it's not a nilable expression.
//
// ParseExprAsProducer will panic if passed the empty expression `_`
//
// nilable(shallowSeq)
// nilable(producers)
//
// TODO: split this up into smaller functions with more granular documentation
func (r *RootAssertionNode) ParseExprAsProducer(expr ast.Expr, doNotTrack bool) (
	shallowSeq TrackableExpr, producers []producer.ParsedProducer) {

	parseIdent := func(expr *ast.Ident) (TrackableExpr, []producer.ParsedProducer) {
		if util.IsEmptyExpr(expr) {
			panic("the empty identifier is not an expression - don't pass it to ParseExprAsProducer")
		}
		if r.isNil(expr) {
			return nil, []producer.ParsedProducer{producer.ShallowParsedProducer{
				Producer: &annotation.ProduceTrigger{
					Annotation: &annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}},
					Expr:       expr,
				},
			}}
		}
		if r.isConst(expr) || r.isBuiltIn(expr) || r.isStable(expr) || r.isTypeName(expr) || r.isFunc(expr) {
			// we assume none of these types of identifiers return nil
			// TODO: refine this handling of constants
			return nil, nil
		}

		funcObj := r.FuncObj()
		varObj := r.ObjectOf(expr).(*types.Var)
		if doNotTrack {
			if annotation.VarIsRecv(funcObj, varObj) {
				return nil, []producer.ParsedProducer{producer.DeepParsedProducer{
					ShallowProducer: &annotation.ProduceTrigger{
						Annotation: &annotation.MethodRecv{
							TriggerIfNilable: &annotation.TriggerIfNilable{
								Ann: &annotation.RecvAnnotationKey{FuncDecl: funcObj}},
							VarDecl: varObj,
						},
						Expr: expr,
					},
					DeepProducer: &annotation.ProduceTrigger{
						Annotation: annotation.DeepNilabilityOfVar(funcObj, varObj),
						Expr:       expr,
					},
				}}
			}
			varProducer := func() *annotation.ProduceTrigger {
				if annotation.VarIsParam(funcObj, varObj) {
					return &annotation.ProduceTrigger{
						Annotation: annotation.ParamAsProducer(funcObj, varObj),
						Expr:       expr,
					}
				}
				if annotation.VarIsGlobal(varObj) {
					return &annotation.ProduceTrigger{
						Annotation: &annotation.GlobalVarRead{
							TriggerIfNilable: &annotation.TriggerIfNilable{
								Ann: &annotation.GlobalVarAnnotationKey{
									VarDecl: varObj,
								}}},
						Expr: expr,
					}
				}
				// in the case of a totally unrecognized identifier - we assume nilability
				return &annotation.ProduceTrigger{
					Annotation: &annotation.ProduceTriggerTautology{},
					Expr:       expr,
				}
			}
			return nil, []producer.ParsedProducer{producer.DeepParsedProducer{
				ShallowProducer: varProducer(),
				DeepProducer: &annotation.ProduceTrigger{
					Annotation: annotation.DeepNilabilityOfVar(funcObj, varObj),
					Expr:       expr,
				},
			}}
		}
		if r.isPkgName(expr) {
			panic("ParseExprAsProducer should not be called on bare package names")
		}

		// by process of elimination, it's a variable, so track it!
		return TrackableExpr{&varAssertionNode{decl: r.ObjectOf(expr).(*types.Var)}}, nil
	}

	// this function represents the case in which we have identified that the value of the
	// expression being parsed flows from a _deep read_ to the expression `deepExpr`.
	// this function is only to be used in cases when we have determined that the parsed
	// expression is not trackable.
	parseDeepRead := func(
		recv TrackableExpr, // the already parsed prefix to `expr` - all we care about is whether nil
		deepExpr ast.Expr, // the expression we identified is being deeply read for this parse
		expr ast.Expr, // the overall expression being parsed - used to construct `annotation.ProduceTrigger`s
		rproducers []producer.ParsedProducer, // the, possibly already set, parse of `deepExpr`
		// in general - our goal is to obtain the parse of `deepExpr` - then lift its deep producer to
		// the shallow producer of a new `ParsedProducer`, and populate the new deep producer by a default
		// based on type name if applicable
	) []producer.ParsedProducer {

		if recv != nil {
			// this is so that if the first time we parsed this we determined it was trackable,
			// then re re-parse to obtain as a non-tracked producer
			// an example case is that the receiver was trackable in an index expression,
			// but the index was non-literal
			_, rproducers = r.ParseExprAsProducer(deepExpr, true)
		}

		if len(rproducers) > 1 {
			panic("this should only be reachable if a multiply returning function is " +
				"passed to a deep read such as an index - a case that should result in a type error")
		}

		if rproducers != nil && rproducers[0].IsDeep() {
			return []producer.ParsedProducer{producer.DeepParsedProducer{
				ShallowProducer: &annotation.ProduceTrigger{
					Annotation: rproducers[0].GetDeep().Annotation,
					Expr:       expr,
				},
				// there is no possible source for a doubly deep nilability annotation except
				// the named type of the expression
				DeepProducer: &annotation.ProduceTrigger{
					Annotation: annotation.DeepNilabilityAsNamedType(r.Pass().TypesInfo.Types[expr].Type),
					Expr:       expr,
				},
			}}
		}

		// if we reach here - that should mean that expr.X is not deeply nilable, so we know this
		// read cannot produce nil
		return []producer.ParsedProducer{producer.ShallowParsedProducer{Producer: &annotation.ProduceTrigger{
			Annotation: &annotation.ProduceTriggerNever{},
			Expr:       expr,
		}}}
	}

	switch expr := expr.(type) {
	case *ast.Ident:
		return parseIdent(expr)
	case *ast.SelectorExpr:
		if r.isPkgName(expr.X) {
			// if we've reduced to a package-qualified identifier like pkg.A, just interpret it
			// as a bare identifier
			return parseIdent(expr.Sel)
		}

		if r.isBuiltIn(expr.Sel) || r.isFunc(expr.Sel) {
			// we assume builtins aren't nilable
			// functions are definitely not nilable
			return nil, nil
		}

		fldReadProduce := func() []producer.ParsedProducer {
			fldObj := r.ObjectOf(expr.Sel).(*types.Var)
			return []producer.ParsedProducer{producer.DeepParsedProducer{
				ShallowProducer: &annotation.ProduceTrigger{
					Annotation: &annotation.FldRead{
						TriggerIfNilable: &annotation.TriggerIfNilable{
							Ann: &annotation.FieldAnnotationKey{
								FieldDecl: fldObj}}},
					Expr: expr,
				},
				DeepProducer: &annotation.ProduceTrigger{
					Annotation: annotation.DeepNilabilityOfFld(fldObj),
					Expr:       expr,
				},
			}}
		}

		if doNotTrack {
			// treat as non-trackable
			return nil, fldReadProduce()
		}

		if recv, _ := r.ParseExprAsProducer(expr.X, false); recv != nil {
			// trackable access to a field
			return append(recv, &fldAssertionNode{decl: r.ObjectOf(expr.Sel).(*types.Var),
				functionContext: r.functionContext}), nil
		}
		// non-trackable access to a field - just return a produce trigger for that field
		return nil, fldReadProduce()

	case *ast.CallExpr:
		// we delay this check until we're sure we have to make it, as it could be expensive
		litArgs := func() bool {
			for _, expr := range expr.Args {
				if !r.isStable(expr) {
					return false
				}
			}
			return true
		}

		if ret, ok := AsTrustedFuncAction(expr, r.Pass()); ok {
			if prod, ok := ret.(*annotation.ProduceTrigger); ok {
				return nil, []producer.ParsedProducer{producer.ShallowParsedProducer{Producer: prod}}
			}
		}

		// the cases of a function and method call are different enough here that it would be useless
		// to try to subsume this switch with funcIdentFromCallExpr
		switch fun := expr.Fun.(type) {
		case *ast.Ident: // direct function call
			if !r.isFunc(fun) {
				// The following block implements the basic support for append function where it has
				// only two arguments and the first argument is the same as the lhs of assignment.
				// Since in Go it is allowed to have only one argument in the append method, we need
				// to have a check to make sure that len(expr.Args) > 1
				if fun.Name == BuiltinAppend && len(expr.Args) > 1 {
					// TODO: handle the correlation of return type of append with its first argument .
					// TODO: iterate over the arguments of the append call if it has more than two args
					rec, producers := r.ParseExprAsProducer(expr.Args[1], false)
					return rec, producers
				}

				// We are in the case of built-in functions. The below block particularly checks for the case of the
				// built-in `new` function for struct initialization handling. The `new` function returns a pointer to
				// the passed type (e.g., new(S) returns *S), which is same as creating a struct using composite
				// literal `&S{}`. We are interested in handling this case since all fields of the struct `S` would be
				// uninitialized with a `new(S)`.
				// TODO: below logic won't be required once we standardize the calls by replacing `new(S)` with `&S{}`
				//  in the preprocessing phase after  is implemented.
				if r.functionContext.isDepthOneFieldCheck() && fun.Name == BuiltinNew {
					rproducer := r.parseStructCreateExprAsProducer(expr.Args[0], nil)
					if rproducer != nil {
						return nil, []producer.ParsedProducer{rproducer}
					}
				}

				// for builtin funcs (e.g. new, make), we assume their return is never nil
				// similarly, we assume type casts (e.g. `int(x)`) never return nil
				// anonymous functions will also fall into this case
				return nil, nil
			}
			// non-builtin funcs
			if !doNotTrack && litArgs() {
				return TrackableExpr{&funcAssertionNode{
					decl: r.ObjectOf(fun).(*types.Func), args: expr.Args}}, nil
			}
			// function call has non-literal args, so is not literal, use its return annotation
			// alternatively, doNotTrack was set
			return nil, r.getFuncReturnProducers(fun, expr)

		case *ast.SelectorExpr: // method call
			if !r.isFunc(fun.Sel) {
				// we assume builtins and type casts don't return nil
				return nil, nil
			}
			if doNotTrack {
				return nil, r.getFuncReturnProducers(fun.Sel, expr)
			}
			if litArgs() {
				if r.isPkgName(fun.X) {
					return TrackableExpr{&funcAssertionNode{
						decl: r.ObjectOf(fun.Sel).(*types.Func), args: expr.Args}}, nil
				}
				if recv, _ := r.ParseExprAsProducer(fun.X, false); recv != nil {
					return append(recv, &funcAssertionNode{
						decl: r.ObjectOf(fun.Sel).(*types.Func), args: expr.Args}), nil
				}
				// receiver is not trackable, use its return annotation
				return nil, r.getFuncReturnProducers(fun.Sel, expr)

			}
			// function call has non-literal args, so is not literal, use its return annotation
			return nil, r.getFuncReturnProducers(fun.Sel, expr)

		default:
			// this could result from calling a function returned anonymously from another function, such as f(4)(3), and
			// although theoretically we should track that, we're going to leave it as an unhandled edge case for now
			// TODO: consider handling this case (and similar case in backPropAcrossReturn)
			return nil, nil
		}
	case *ast.IndexExpr:
		recv, rproducers := r.ParseExprAsProducer(expr.X, false)

		if doNotTrack {
			return nil, parseDeepRead(recv, expr.X, expr, rproducers)
		}
		if recv != nil {
			// receiver is trackable
			if r.isStable(expr.Index) {
				// receiver is trackable and index is stable, so return an augmented path
				return append(recv, &indexAssertionNode{
					index:    expr.Index,
					valType:  r.Pass().TypesInfo.Types[expr].Type,
					recvType: r.Pass().TypesInfo.Types[expr.X].Type,
				}), nil
			}
			// index is non-literal, so the expression is not trackable, just return nilable for index without check
			return nil, parseDeepRead(recv, expr.X, expr, rproducers)
		}
		// reciever is non-trackable, just return nilable for index without check
		return nil, parseDeepRead(recv, expr.X, expr, rproducers)
	case *ast.SliceExpr:
		switch {
		// For slice expressions `b[_:0:_]`, the result is always an empty (nilable in
		// NilAway's eyes) slice. (`_` can be anything including empty.)
		case r.isIntZero(expr.High):
			// We should create a nilable producer.
			return nil, []producer.ParsedProducer{producer.ShallowParsedProducer{
				Producer: &annotation.ProduceTrigger{
					Annotation: &annotation.ProduceTriggerTautology{},
					Expr:       expr,
				}}}
		// For slice expressions `b[0:]` and `b[:]`, the result's nilability depends on the
		// nilability of the original slice. Note that you cannot give empty High in 3-index
		// slices.
		case expr.High == nil && (expr.Low == nil || r.isIntZero(expr.Low)):
			// TODO: for now we directly return the trackable expression of the original slice. We
			// should instead properly create a trackable expression for the slice expression. See
			//  for more details.

			if doNotTrack {
				return nil, nil
			}
			// Return the trackable expression of the original slice
			return r.ParseExprAsProducer(expr.X, false)
		// For all other cases, the result must be a nonnil slice.
		default:
			// Returning nil to indicate the slice expression results in a nonnil slice.
			return nil, nil
		}
	case *ast.StarExpr:
		recv, rproducers := r.ParseExprAsProducer(expr.X, false)

		// TODO - if `recv` is trackable, then track expression instead, as in the index case

		return nil, parseDeepRead(recv, expr.X, expr, rproducers)
	case *ast.UnaryExpr:
		if expr.Op == token.ARROW {
			// we've found a receive expression
			_, rproducers := r.ParseExprAsProducer(expr.X, true)
			return nil, parseDeepRead(nil, expr.X, expr, rproducers)
		}
		if expr.Op == token.AND {
			// we treat a struct object pointer (e.g., &A{}) and struct object (e.g., A{}) identically for creating field producers
			t := util.TypeOf(r.Pass(), expr.X)
			if s := util.TypeAsDeeplyStruct(t); s != nil {
				return r.ParseExprAsProducer(expr.X, doNotTrack)
			}
		}
	case *ast.ParenExpr:
		// simply parse the underlying expression
		return r.ParseExprAsProducer(expr.X, doNotTrack)

	case *ast.CompositeLit:
		if r.functionContext.isDepthOneFieldCheck() {
			rproducer := r.parseStructCreateExprAsProducer(expr, expr.Elts)
			if rproducer != nil {
				return nil, []producer.ParsedProducer{rproducer}
			}
		}
		return nil, nil
	}
	// TODO: right now this default case assumes that unhandled expressions are non-nil, consider changing this
	return nil, nil
}

// getFuncReturnProducers returns a list of producers that are triggered at the call expression
func (r *RootAssertionNode) getFuncReturnProducers(ident *ast.Ident, expr *ast.CallExpr) []producer.ParsedProducer {
	funcObj := r.ObjectOf(ident).(*types.Func)

	numResults := util.FuncNumResults(funcObj)
	isErrReturning := util.FuncIsErrReturning(funcObj)

	producers := make([]producer.ParsedProducer, numResults)

	for i := 0; i < numResults; i++ {
		var retKey annotation.Key
		if r.HasContract(funcObj) {
			// Creates a new return site with location information at every call site for a
			// function with contracts. The return site is unique at every call site, even with the
			// same function called.
			retKey = annotation.NewCallSiteRetKey(funcObj, i, r.LocationOf(expr))
		} else {
			retKey = annotation.RetKeyFromRetNum(funcObj, i)
		}

		var fieldProducers []*annotation.ProduceTrigger

		if r.functionContext.isDepthOneFieldCheck() {
			fieldProducers = r.getFieldProducersForFuncReturns(funcObj, i)
		}

		producers[i] = producer.DeepParsedProducer{
			ShallowProducer: &annotation.ProduceTrigger{
				Annotation: &annotation.FuncReturn{
					TriggerIfNilable: &annotation.TriggerIfNilable{
						Ann: retKey,
						// for an error-returning function, all but the last result are guarded
						// TODO: add an annotation that allows more results to escape from guarding
						// such as "error-nonnil" or "always-nonnil"
						NeedsGuard: isErrReturning && i != numResults-1,
					},
				},
				Expr: expr,
			},
			DeepProducer: &annotation.ProduceTrigger{
				Annotation: annotation.DeepNilabilityOfFuncRet(funcObj, i),
				Expr:       expr,
			},
			FieldProducers: fieldProducers,
		}
	}
	return producers
}

// parseStructCreateExprAsProducer parses composite expressions used to initialize a struct e.g. A{f1: v1, f2: v2}
func (r *RootAssertionNode) parseStructCreateExprAsProducer(expr ast.Expr, fieldInitializations []ast.Expr) producer.ParsedProducer {
	exprType := r.Pass().TypesInfo.TypeOf(expr)

	if structType := util.TypeAsDeeplyStruct(exprType); structType != nil {
		numFields := structType.NumFields()
		fieldProducerArray := make([]*annotation.ProduceTrigger, numFields)

		for i := 0; i < numFields; i++ {
			fieldDecl := structType.Field(i)
			field := r.GetDeclaringIdent(fieldDecl)

			if util.TypeBarsNilness(fieldDecl.Type()) {
				// we do not create producers for fields that are not nilable
				continue
			}

			// extract the value assigned to the field in the composite
			fieldVal := util.GetFieldVal(fieldInitializations, field.Name, numFields, i)

			if fieldVal == nil {
				// this means the field is not assigned any value, thus unassigned field should be produced
				fieldProducerArray[i] = &annotation.ProduceTrigger{Annotation: &annotation.UnassignedFld{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}}}
			} else {
				// do not track. Get producer for expression `fieldVal` assigned to the field
				_, fieldProducer := r.ParseExprAsProducer(fieldVal, true)
				if fieldProducer != nil {
					// since we only track field producers at depth one, we ignore deep producers from the field
					fieldProducerArray[i] = fieldProducer[0].GetShallow()
				} else {
					// If the field producer is nil, that means it is not a nilable expression
					fieldProducerArray[i] = &annotation.ProduceTrigger{Annotation: &annotation.ProduceTriggerNever{}}
				}
			}
		}

		return producer.DeepParsedProducer{
			ShallowProducer: &annotation.ProduceTrigger{Annotation: &annotation.ProduceTriggerNever{}},
			DeepProducer:    nil,
			FieldProducers:  fieldProducerArray,
		}
	}

	return nil
}
