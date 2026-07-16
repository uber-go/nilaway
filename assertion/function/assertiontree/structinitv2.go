//  Copyright (c) 2026 Uber Technologies, Inc.
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
	"sort"
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/guard"
	"go.uber.org/nilaway/util/asthelper"
	"go.uber.org/nilaway/util/typeshelper"
	"golang.org/x/tools/go/types/typeutil"
)

// asStructAllocation inspects expr and, if it allocates a struct value, returns the (deeply
// resolved) struct type together with the composite-literal element expressions (nil when the
// allocation has no explicit field initializers, e.g. `new(A)`). The boolean result reports
// whether expr is a struct allocation.
//
// Recognized forms: `A{...}`, `&A{...}`, and `new(A)`.
func (r *RootAssertionNode) asStructAllocation(expr ast.Expr) (*types.Struct, []ast.Expr, bool) {
	switch e := expr.(type) {
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return r.asStructAllocation(e.X)
		}
	case *ast.ParenExpr:
		return r.asStructAllocation(e.X)
	case *ast.CompositeLit:
		if structType := typeshelper.AsDeeplyStruct(r.Pass().TypesInfo.TypeOf(e)); structType != nil {
			return structType, e.Elts, true
		}
	case *ast.CallExpr:
		if ident, ok := e.Fun.(*ast.Ident); ok && r.ObjectOf(ident) == typeshelper.BuiltinNew {
			// new(A) yields a *A whose fields are all zero-valued (nil for nilable fields).
			if structType := typeshelper.AsDeeplyStruct(r.Pass().TypesInfo.TypeOf(e)); structType != nil {
				return structType, nil, true
			}
		}
	}
	return nil, nil, false
}

// addAllocationFieldProducers attaches, for every nilable field of a struct value allocated on the
// RHS of an assignment and bound to lhsVal, a producer describing that field's nilability at
// this allocation site:
//   - a field with no initializer  -> StructFieldNil (definitely nil)
//   - a field initialized to expr e -> the (shallow) producer of e (e.g. nonnil for `&A{}` or
//     `new(A)`, nil for an explicit `nil`)
//
// If the RHS is instead a call to a struct-returning function, the value's fields are bound,
// symbolically, to that function's return context sites (see addContextFieldProducers), so
// the nilability flows interprocedurally through inference.
//
// It must be called before the generic assignment handling produces lhsVal itself, because the
// latter detaches the lhsVal subtree (including the field nodes we target) once produced.
func (r *RootAssertionNode) addAllocationFieldProducers(lhsVal, rhsVal ast.Expr) {
	if structType, fieldInits, ok := r.asStructAllocation(rhsVal); ok {
		r.addFieldProducers(structType, fieldInits, lhsVal)
		return
	}

	// `lhs := f()` where f returns a single struct value: bind lhs's fields to f's return
	// context sites. (Multi-return calls are handled by the many-to-one assignment path.)
	if call, ok := ast.Unparen(rhsVal).(*ast.CallExpr); ok {
		if funcObj := typeutil.StaticCallee(r.Pass().TypesInfo, call); funcObj != nil {
			sig := funcObj.Type().(*types.Signature)
			if sig.Results().Len() == 1 {
				if structType := typeshelper.AsDeeplyStruct(sig.Results().At(0).Type()); structType != nil {
					r.addContextFieldProducers(structType, lhsVal, funcObj, annotation.StructFieldReturnContext, 0)
				}
			}
		}
	}
}

// getShallowExprNilabilityProducer returns the producer encoding the nilability of the value of expr: an
// always-nil producer for an explicit `nil`, the shallow producer of a trackable/nilable
// expression, or Never for a value that cannot be nil (e.g. `&A{}`, `new(A)`).
func (r *RootAssertionNode) getShallowExprNilabilityProducer(expr ast.Expr) annotation.ProducingAnnotationTrigger {
	if ident, ok := ast.Unparen(expr).(*ast.Ident); ok && r.isNil(ident) {
		return &annotation.ProduceTriggerTautology{}
	}
	if _, _, ok := r.asStructAllocation(expr); ok {
		return &annotation.ProduceTriggerNever{}
	}
	if _, producers := r.ParseExprAsProducer(expr, true); len(producers) != 0 {
		return producers[0].GetShallow().Annotation
	}
	return &annotation.ProduceTriggerNever{}
}

// getFieldInitNilabilityProducer returns the producer encoding the nilability of field i of a struct
// allocation with the given field initializers.
func (r *RootAssertionNode) getFieldInitNilabilityProducer(structType *types.Struct, fieldInits []ast.Expr, i int) annotation.ProducingAnnotationTrigger {
	field := structType.Field(i)
	fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), structType.NumFields(), i)
	if fieldVal == nil {
		return &annotation.StructFieldNil{
			ProduceTriggerTautology: &annotation.ProduceTriggerTautology{},
			FieldName:               field.Name(),
		}
	}
	return r.getShallowExprNilabilityProducer(fieldVal)
}

// addFieldProducers performs the per-field producer attachment described on
// addAllocationFieldProducers for a concrete struct allocation. fieldInits may be nil.
func (r *RootAssertionNode) addFieldProducers(structType *types.Struct, fieldInits []ast.Expr, base ast.Expr) {
	numFields := structType.NumFields()
	for i := range numFields {
		field := structType.Field(i)
		fieldSel := r.getSelectorExpr(field, base)
		fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), numFields, i)
		nilable := !typeshelper.TypeBarsNilness(field.Type())

		switch {
		case fieldVal != nil:
			if innerType, innerInits, ok := r.asStructAllocation(fieldVal); ok {
				r.addFieldProducers(innerType, innerInits, fieldSel)
			}
		case !nilable:
			if innerType := typeshelper.AsDeeplyStruct(field.Type()); innerType != nil {
				r.addFieldProducers(innerType, nil, fieldSel)
			}
		}

		if !nilable {
			continue
		}
		r.AddProduction(&annotation.ProduceTrigger{
			Annotation: r.getFieldInitNilabilityProducer(structType, fieldInits, i),
			Expr:       fieldSel,
		})
	}
}

// accessedFieldPath is one accessed field path under a boundary value, paired with the synthesized
// selector expression that reaches it.
type accessedFieldPath struct {
	sel  ast.Expr
	path string
}

// collectAccessedFieldPaths walks the live assertion subtree under node and collects accessed nilable
// field paths, deepest path first. prefix is the dotted path from the boundary value to node.
func (r *RootAssertionNode) collectAccessedFieldPaths(node AssertionNode, base ast.Expr, prefix string, seen map[*types.Struct]bool, out *[]accessedFieldPath) {
	for _, child := range node.Children() {
		fldNode, ok := child.(*fldAssertionNode)
		if !ok {
			continue
		}
		field := fldNode.decl
		sel := r.getSelectorExpr(field, base)
		path := field.Name()
		if prefix != "" {
			path = prefix + "." + field.Name()
		}
		if inner := typeshelper.AsDeeplyStruct(field.Type()); inner == nil || !seen[inner] {
			if inner != nil {
				seen[inner] = true
			}
			r.collectAccessedFieldPaths(child, sel, path, seen, out)
			if inner != nil {
				delete(seen, inner)
			}
		}
		if !typeshelper.TypeBarsNilness(field.Type()) {
			*out = append(*out, accessedFieldPath{sel: sel, path: path})
		}
	}
}

// addContextFieldProducers attaches, for each accessed nilable field path under base, a producer
// making `base.<path>` nil iff the corresponding return/param context site of funcObj is inferred
// nilable.
func (r *RootAssertionNode) addContextFieldProducers(_ *types.Struct, base ast.Expr, funcObj *types.Func, kind annotation.StructFieldContextKind, index int) {
	path, _ := r.ParseExprAsProducer(base, false)
	node, _ := r.lookupPath(path)
	if node == nil {
		return
	}
	var paths []accessedFieldPath
	r.collectAccessedFieldPaths(node, base, "", make(map[*types.Struct]bool), &paths)
	for _, p := range paths {
		site := &annotation.StructFieldContextSite{
			FuncObj: funcObj, Kind: kind, Index: index, Path: p.path,
		}
		r.AddProduction(&annotation.ProduceTrigger{
			Annotation: &annotation.StructFieldFromContext{
				TriggerIfNilable: &annotation.TriggerIfNilable{Ann: site},
			},
			Expr: p.sel,
		})
	}
}

// bindReturnFieldsToContext binds, at a return statement, the fields of each struct-typed return
// value to that result's return context site, so the returned value's per-field nilability
// becomes the function's return summary.
func (r *RootAssertionNode) bindReturnFieldsToContext(node *ast.ReturnStmt) {
	sig := r.FuncObj().Type().(*types.Signature)
	if len(node.Results) != sig.Results().Len() {
		return
	}
	if typeshelper.FuncIsErrReturning(sig) {
		errExpr := node.Results[sig.Results().Len()-1]
		// We are not attaching a consumer when we are sure that the the err is definitely non-nil
		if _, definitelyNonNil := r.getShallowExprNilabilityProducer(errExpr).(*annotation.ProduceTriggerNever); definitelyNonNil {
			return
		}
	}
	for retIdx, retExpr := range node.Results {
		structType := typeshelper.AsDeeplyStruct(sig.Results().At(retIdx).Type())
		if structType == nil {
			continue
		}
		r.bindValueFieldsToContext(r.FuncObj(), retExpr, structType, annotation.StructFieldReturnContext, retIdx)
	}
}

// bindValueFieldsToContext connects the fields of the value produced by valExpr to the context
// site of targetFunc at (kind, index). Inline allocations and trackable values are bound; a
// struct-returning call forwarded across the boundary is unsupported (see below).
func (r *RootAssertionNode) bindValueFieldsToContext(targetFunc *types.Func, valExpr ast.Expr, structType *types.Struct, kind annotation.StructFieldContextKind, index int) {
	funcObj := targetFunc

	// A struct-returning call forwarded across a boundary (e.g. `return f()` or `g(f())`) is
	// unsupported, so we do nothing for it. Binding it would compose the callee's per-field return
	// summary into this boundary, which requires the return-read demand to be transitively closed
	// over forwarding edges (as the effects prepass does for param reads). That closure is not
	// computed for returns, as it is incomplete for cross package returns (return read info flows in
	// the opposite direction of analysis facts)
	if _, ok := ast.Unparen(valExpr).(*ast.CallExpr); ok {
		return
	}

	allocType, fieldInits, isAlloc := r.asStructAllocation(valExpr)
	if isAlloc {
		r.bindAllocationFieldsToContext(allocType, fieldInits, valExpr, "", funcObj, kind, index)
		return
	}

	trackablePath, _ := r.ParseExprAsProducer(valExpr, false)
	if trackablePath == nil {
		return
	}

	var demanded []string
	switch kind {
	case annotation.StructFieldParamContext:
		demanded = r.functionContext.paramFieldEffects.ParamReadPaths(funcObj, index)
	case annotation.StructFieldReturnContext:
		demanded = r.functionContext.paramFieldEffects.ReturnReadPaths(funcObj, index)
	}
	for _, fieldPath := range demanded {
		sel, ok := r.buildFieldPathSelector(valExpr, structType, fieldPath)
		if !ok {
			continue
		}
		site := &annotation.StructFieldContextSite{
			FuncObj: funcObj, Kind: kind, Index: index, Path: fieldPath,
		}
		r.AddConsumption(&annotation.ConsumeTrigger{
			Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site}},
			Expr:       sel,
			Guards:     guard.NoGuards(),
		})
	}
}

// bindAllocationFieldsToContext binds the per-field nilability of an inline struct allocation to
// the boundary context site of funcObj at (kind, index).
func (r *RootAssertionNode) bindAllocationFieldsToContext(structType *types.Struct, fieldInits []ast.Expr, valExpr ast.Expr, prefix string, funcObj *types.Func, kind annotation.StructFieldContextKind, index int) {
	numFields := structType.NumFields()
	for i := range numFields {
		field := structType.Field(i)
		fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), numFields, i)
		nilable := !typeshelper.TypeBarsNilness(field.Type())
		path := field.Name()
		if prefix != "" {
			path = prefix + "." + field.Name()
		}

		switch {
		case fieldVal != nil:
			if innerType, innerInits, ok := r.asStructAllocation(fieldVal); ok {
				r.bindAllocationFieldsToContext(innerType, innerInits, valExpr, path, funcObj, kind, index)
			}
		case !nilable:
			if innerType := typeshelper.AsDeeplyStruct(field.Type()); innerType != nil {
				r.bindAllocationFieldsToContext(innerType, nil, valExpr, path, funcObj, kind, index)
			}
		}

		if !nilable {
			continue
		}
		site := &annotation.StructFieldContextSite{
			FuncObj: funcObj, Kind: kind, Index: index, Path: path,
		}
		r.AddNewTriggers(annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{Annotation: r.getFieldInitNilabilityProducer(structType, fieldInits, i), Expr: valExpr},
			Consumer: &annotation.ConsumeTrigger{
				Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site}},
				Expr:       valExpr,
				Guards:     guard.NoGuards(),
			},
		})
	}
}

// addZeroValueFieldProducers attaches StructFieldNil producers for the nilable fields of a struct value
// that is the zero value.
func (r *RootAssertionNode) addZeroValueFieldProducers(varNode AssertionNode, base ast.Expr) {
	children := make([]AssertionNode, len(varNode.Children()))
	copy(children, varNode.Children())

	for _, child := range children {
		fldNode, ok := child.(*fldAssertionNode)
		if !ok || typeshelper.TypeBarsNilness(fldNode.decl.Type()) {
			continue
		}
		selExpr := r.getSelectorExpr(fldNode.decl, base)
		r.AddProduction(&annotation.ProduceTrigger{
			Annotation: &annotation.StructFieldNil{
				ProduceTriggerTautology: &annotation.ProduceTriggerTautology{},
				FieldName:               fldNode.decl.Name(),
			},
			Expr: selExpr,
		})
	}
}

// getParamIndex returns the parameter index of v within the current function's signature, or the
// receiver index for the receiver, and whether v is a parameter/receiver at all.
func (r *RootAssertionNode) getParamIndex(v *types.Var) (int, bool) {
	sig := r.FuncObj().Signature()
	if recv := sig.Recv(); recv != nil && recv == v {
		return annotation.ReceiverParamIndex, true
	}
	for i := range sig.Params().Len() {
		if sig.Params().At(i) == v {
			return i, true
		}
	}
	return 0, false
}

// addParamFieldProducers attaches, at function entry, producers making each nilable field of
// a struct-typed parameter/receiver nil iff the corresponding parameter context site is inferred
// nilable.
func (r *RootAssertionNode) addParamFieldProducers(builtExpr ast.Expr) {
	ident, ok := builtExpr.(*ast.Ident)
	if !ok {
		return
	}
	v, ok := r.ObjectOf(ident).(*types.Var)
	if !ok {
		return
	}
	idx, ok := r.getParamIndex(v)
	if !ok {
		return
	}
	structType := typeshelper.AsDeeplyStruct(v.Type())
	if structType == nil {
		return
	}
	r.addContextFieldProducers(structType, builtExpr, r.FuncObj(), annotation.StructFieldParamContext, idx)
}

// buildFieldPathSelector builds the selector expression reaching base.<path>, where path is a
// dotted field path under base's struct type.
func (r *RootAssertionNode) buildFieldPathSelector(base ast.Expr, structType *types.Struct, path string) (ast.Expr, bool) {
	cur := base
	curStruct := structType
	for _, name := range strings.Split(path, ".") {
		if curStruct == nil {
			return nil, false
		}
		var field *types.Var
		for i := range curStruct.NumFields() {
			if curStruct.Field(i).Name() == name {
				field = curStruct.Field(i)
				break
			}
		}
		if field == nil {
			return nil, false
		}
		cur = r.getSelectorExpr(field, cur)
		curStruct = typeshelper.AsDeeplyStruct(field.Type())
	}
	return cur, true
}

// bindArgAndReceiverFieldsToContext binds, at a function or method call, the fields of each struct-typed
// argument to the callee's corresponding parameter context site.
func (r *RootAssertionNode) bindArgAndReceiverFieldsToContext(call *ast.CallExpr, funcObj *types.Func) {
	sig := funcObj.Signature()

	if recv := sig.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if structType := typeshelper.AsDeeplyStruct(recv.Type()); structType != nil {
				r.bindValueFieldsToContext(funcObj, sel.X, structType, annotation.StructFieldParamContext, annotation.ReceiverParamIndex)
			}
		}
	}

	for argIdx, arg := range call.Args {
		if argIdx >= sig.Params().Len() {
			break
		}
		structType := typeshelper.AsDeeplyStruct(sig.Params().At(argIdx).Type())
		if structType == nil {
			continue
		}
		r.bindValueFieldsToContext(funcObj, arg, structType, annotation.StructFieldParamContext, argIdx)
	}
}

// addCallParamOutFieldProducers attaches a callee's post-call field state to its concrete arguments
// and receiver. For `f(x)`, if f's parameter 0 may write `b.c`, it produces
// `x.b.c <- PARAM_OUT(f, 0, "b.c")`. A post-call dereference of x.b.c then consumes f's output
// summary, while fields absent from the write set retain their pre-call producers.
func (r *RootAssertionNode) addCallParamOutFieldProducers(call *ast.CallExpr, funcObj *types.Func) {
	sig := funcObj.Signature()
	produce := func(arg ast.Expr, structType *types.Struct, index int) {
		paths := r.functionContext.paramFieldEffects.ParamWritePaths(funcObj, index)
		// AddProduction detaches a matched subtree, so nested paths must be produced first.
		sort.SliceStable(paths, func(i, j int) bool {
			return strings.Count(paths[i], ".") > strings.Count(paths[j], ".")
		})
		for _, fieldPath := range paths {
			fieldExpr, ok := r.buildFieldPathSelector(arg, structType, fieldPath)
			if !ok {
				continue
			}
			site := &annotation.StructFieldContextSite{
				FuncObj: funcObj,
				Kind:    annotation.StructFieldParamOutContext,
				Index:   index,
				Path:    fieldPath,
			}
			r.AddProduction(&annotation.ProduceTrigger{
				Annotation: &annotation.StructFieldFromContext{
					TriggerIfNilable: &annotation.TriggerIfNilable{Ann: site},
				},
				Expr: fieldExpr,
			})
		}
	}

	if recv := sig.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if structType := typeshelper.AsDeeplyStruct(recv.Type()); structType != nil {
				produce(sel.X, structType, annotation.ReceiverParamIndex)
			}
		}
	}
	for index, arg := range call.Args {
		if index >= sig.Params().Len() {
			break
		}
		if structType := typeshelper.AsDeeplyStruct(sig.Params().At(index).Type()); structType != nil {
			produce(arg, structType, index)
		}
	}
}

// bindForwardedParamOut connects a callee's output summary to a forwarder's output summary. For
// `func g(p *A) { f(p) }`, when f's parameter 0 may write b.c, it adds
// `PARAM_OUT(f, 0, "b.c") -> PARAM_OUT(g, 0, "b.c")`. A field prefix is retained, so passing
// p.inner to f instead targets `PARAM_OUT(g, 0, "inner.b.c")`. The write summary is already closed
// over these edges; this supplies each inherited path's context value.
func (r *RootAssertionNode) bindForwardedParamOut(call *ast.CallExpr, callee *types.Func) {
	sig := callee.Signature()
	link := func(calleeIndex int, arg ast.Expr) {
		base, prefix := asthelper.SplitFieldChain(arg)
		if base == nil {
			return
		}
		param, ok := r.ObjectOf(base).(*types.Var)
		if !ok {
			return
		}
		callerIndex, ok := r.getParamIndex(param)
		if !ok {
			return
		}
		for _, calleePath := range r.functionContext.paramFieldEffects.ParamWritePaths(callee, calleeIndex) {
			fieldPath := calleePath
			if prefix != "" {
				fieldPath = prefix + "." + fieldPath
			}
			source := &annotation.StructFieldContextSite{
				FuncObj: callee, Kind: annotation.StructFieldParamOutContext, Index: calleeIndex, Path: calleePath,
			}
			destination := &annotation.StructFieldContextSite{
				FuncObj: r.FuncObj(), Kind: annotation.StructFieldParamOutContext, Index: callerIndex, Path: fieldPath,
			}
			r.AddNewTriggers(annotation.FullTrigger{
				Producer: &annotation.ProduceTrigger{
					Annotation: &annotation.StructFieldFromContext{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: source}},
					Expr:       arg,
				},
				Consumer: &annotation.ConsumeTrigger{
					Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: destination}},
					Expr:       arg,
					Guards:     guard.NoGuards(),
				},
			})
		}
	}

	if recv := sig.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			link(annotation.ReceiverParamIndex, sel.X)
		}
	}
	for index, arg := range call.Args {
		if index >= sig.Params().Len() {
			break
		}
		link(index, arg)
	}
}

// bindParamFieldWriteToContext records a direct parameter or receiver field write as the callee's
// post-call output. For `func f(p *A) { p.b.c = value }`, it connects
// `value -> PARAM_OUT(f, 0, "b.c")`. For a local value, the consumer is attached to the local so
// ordinary intraprocedural flow supplies the context. The write-summary check excludes local field
// assignments from this boundary.
func (r *RootAssertionNode) bindParamFieldWriteToContext(lhs, rhs ast.Expr) {
	base, fieldPath := asthelper.SplitFieldChain(lhs)
	if base == nil || fieldPath == "" {
		return
	}
	param, ok := r.ObjectOf(base).(*types.Var)
	if !ok {
		return
	}
	index, ok := r.getParamIndex(param)
	if !ok {
		return
	}
	for _, path := range r.functionContext.paramFieldEffects.ParamWritePaths(r.FuncObj(), index) {
		if path != fieldPath {
			continue
		}
		site := &annotation.StructFieldContextSite{
			FuncObj: r.FuncObj(),
			Kind:    annotation.StructFieldParamOutContext,
			Index:   index,
			Path:    fieldPath,
		}
		consumer := &annotation.ConsumeTrigger{
			Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site}},
			Expr:       lhs,
			Guards:     guard.NoGuards(),
		}
		if ident, ok := ast.Unparen(rhs).(*ast.Ident); ok {
			if v, ok := r.ObjectOf(ident).(*types.Var); ok {
				if _, isParam := r.getParamIndex(v); !isParam && !annotation.VarIsGlobal(v) {
					consumer.Expr = rhs
					r.AddConsumption(consumer)
					return
				}
			}
		}
		r.AddNewTriggers(annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{
				Annotation: r.getShallowExprNilabilityProducer(rhs),
				Expr:       rhs,
			},
			Consumer: consumer,
		})
		return
	}
}
