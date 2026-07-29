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
	"slices"
	"sort"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/assertion/function/structfieldeffects"
	"go.uber.org/nilaway/guard"
	"go.uber.org/nilaway/util/asthelper"
	"go.uber.org/nilaway/util/typeshelper"
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
// symbolically, to that function's return context sites (see addCallResultFieldProducers), so
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
		if target, ok := typeshelper.ResolveStaticCallTarget(r.Pass().TypesInfo, call); ok && target.Signature.Results().Len() == 1 {
			if typeshelper.AsDeeplyStruct(target.Signature.Results().At(0).Type()) != nil {
				r.addCallResultFieldProducers(lhsVal, call, target.Origin, 0)
			}
		}
	}
}

// resultPathHasParamSource reports whether funcObj's (resultIdx, resultPath) is supplied by a
// caller argument.
func (r *RootAssertionNode) resultPathHasParamSource(funcObj *types.Func, resultIdx int, resultPath annotation.FieldPath) bool {
	sources := r.functionContext.boundaryFieldEffects.ReturnParamSources(funcObj)
	return sources.HasSource(resultIdx, resultPath)
}

// resultValueHasParamSource reports whether funcObj's resultIdx-th result value itself (not a
// field of it) is supplied by a caller argument — a whole-result source from `return p` or
// `return p.x`.
func (r *RootAssertionNode) resultValueHasParamSource(funcObj *types.Func, resultIdx int) bool {
	return r.resultPathHasParamSource(funcObj, resultIdx, annotation.FieldPath{})
}

// addCallResultFieldProducers adds producers for the accessed fields of a call result. Parameter
// sources use call-scoped sites; unresolved sources are skipped to avoid merging callers.
func (r *RootAssertionNode) addCallResultFieldProducers(
	base ast.Expr,
	call *ast.CallExpr,
	funcObj *types.Func,
	resultIdx int,
) {
	path, _ := r.ParseExprAsProducer(base, false)
	node, _ := r.lookupPath(path)
	if node == nil {
		return
	}
	var paths []accessedFieldPath
	r.collectAccessedFieldPaths(node, base, annotation.FieldPath{}, make(map[*types.Struct]bool), &paths)

	returnEffects := make(map[annotation.FieldPath]bool)
	// Error-returning functions are skipped for now: correlating the fields with the error result
	// to be added in the future.
	if !typeshelper.FuncIsErrReturning(funcObj.Signature()) {
		for _, fieldPath := range r.functionContext.boundaryFieldEffects.ReturnEffectPaths(funcObj, resultIdx) {
			returnEffects[fieldPath] = true
		}
	}

	sources := r.functionContext.boundaryFieldEffects.ReturnParamSources(funcObj)
	for _, p := range paths {
		result := structfieldeffects.IndexedFieldPath{Idx: resultIdx, Path: p.path}
		if r.bindParamSourcedResultFieldAtCall(call, funcObj, sources, result, p.sel) {
			continue
		}
		// Unresolved param sources cannot safely use shared sites.
		if sources.HasSource(result.Idx, result.Path) {
			continue
		}
		r.addSharedCallResultFieldProducer(p.sel, funcObj, resultIdx, p.path, returnEffects[p.path])
	}
}

func (r *RootAssertionNode) addSharedCallResultFieldProducer(
	dst ast.Expr,
	funcObj *types.Func,
	resultIdx int,
	resultPath annotation.FieldPath,
	definiteNil bool,
) {
	site := &annotation.StructFieldContextSite{
		FuncObj: funcObj,
		Kind:    annotation.StructFieldReturnContext,
		Index:   resultIdx,
		Path:    resultPath,
	}
	r.AddProduction(&annotation.ProduceTrigger{
		Annotation: &annotation.StructFieldFromContext{
			TriggerIfNilable: &annotation.TriggerIfNilable{Ann: site},
		},
		Expr: dst,
	})
	if !definiteNil {
		return
	}
	segments := resultPath.Segments()
	r.AddNewTriggers(annotation.FullTrigger{
		Producer: &annotation.ProduceTrigger{
			Annotation: &annotation.StructFieldNil{
				ProduceTriggerTautology: &annotation.ProduceTriggerTautology{},
				FieldName:               segments[len(segments)-1],
			},
			Expr: dst,
		},
		Consumer: &annotation.ConsumeTrigger{
			Annotation: &annotation.StructFieldToContext{
				TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site},
			},
			Expr:   dst,
			Guards: guard.NoGuards(),
		},
	})
}

// bindParamSourcedResultFieldAtCall connects a result field to its argument source through a
// call-scoped return site. It runs before param-out producers so the source reflects post-call state.
func (r *RootAssertionNode) bindParamSourcedResultFieldAtCall(
	call *ast.CallExpr,
	funcObj *types.Func,
	sources structfieldeffects.ReturnParamSources,
	result structfieldeffects.IndexedFieldPath,
	dst ast.Expr,
) bool {
	param, ok := sources.ParamPathFromResultPath(result.Idx, result.Path)
	if !ok {
		return false
	}
	sourceExpr, ok := r.paramPathExprAtCall(call, funcObj.Signature(), param)
	if !ok {
		return false
	}
	site := &annotation.StructFieldContextSite{
		FuncObj:  funcObj,
		Kind:     annotation.StructFieldReturnContext,
		Index:    result.Idx,
		Path:     result.Path,
		Location: r.LocationOf(call),
	}
	r.AddProduction(&annotation.ProduceTrigger{
		Annotation: &annotation.StructFieldFromContext{
			TriggerIfNilable: &annotation.TriggerIfNilable{Ann: site},
		},
		Expr: dst,
	})
	r.bindExprNilabilityToContext(sourceExpr, site)
	return true
}

// bindShallowCallResultArgs supplies call-scoped sites for parameter-sourced result values.
func (r *RootAssertionNode) bindShallowCallResultArgs(call *ast.CallExpr, funcObj *types.Func) {
	sources := r.functionContext.boundaryFieldEffects.ReturnParamSources(funcObj)
	sig := funcObj.Signature()
	for resultIdx := range sig.Results().Len() {
		result := structfieldeffects.IndexedFieldPath{Idx: resultIdx}
		param, ok := sources.ParamPathFromResultPath(result.Idx, result.Path)
		if !ok {
			continue
		}
		sourceExpr, ok := r.paramPathExprAtCall(call, sig, param)
		if !ok {
			continue
		}
		site := &annotation.StructFieldContextSite{
			FuncObj:  funcObj,
			Kind:     annotation.StructFieldReturnContext,
			Index:    result.Idx,
			Path:     result.Path,
			Location: r.LocationOf(call),
		}
		r.bindExprNilabilityToContext(sourceExpr, site)
	}
}

// shallowCallResultSiteProducer returns a call-scoped producer for a parameter-sourced result.
func (r *RootAssertionNode) shallowCallResultSiteProducer(
	call *ast.CallExpr,
	funcObj *types.Func,
	resultIdx int,
) (annotation.ProducingAnnotationTrigger, bool) {
	sources := r.functionContext.boundaryFieldEffects.ReturnParamSources(funcObj)
	result := structfieldeffects.IndexedFieldPath{Idx: resultIdx}
	param, ok := sources.ParamPathFromResultPath(result.Idx, result.Path)
	if !ok {
		return nil, false
	}
	if _, ok := r.paramPathExprAtCall(call, funcObj.Signature(), param); !ok {
		return nil, false
	}
	site := &annotation.StructFieldContextSite{
		FuncObj:  funcObj,
		Kind:     annotation.StructFieldReturnContext,
		Index:    result.Idx,
		Path:     result.Path,
		Location: r.LocationOf(call),
	}
	return &annotation.StructFieldFromContext{
		TriggerIfNilable: &annotation.TriggerIfNilable{Ann: site},
	}, true
}

func (r *RootAssertionNode) paramPathExprAtCall(
	call *ast.CallExpr,
	sig *types.Signature,
	param structfieldeffects.IndexedFieldPath,
) (ast.Expr, bool) {
	var source ast.Expr
	var sourceType types.Type
	switch param.Idx {
	case annotation.ReceiverParamIndex:
		recv := sig.Recv()
		sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if recv == nil || !ok {
			return nil, false
		}
		source, sourceType = sel.X, recv.Type()
	default:
		if param.Idx < 0 || param.Idx >= sig.Params().Len() || param.Idx >= len(call.Args) {
			return nil, false
		}
		source, sourceType = call.Args[param.Idx], sig.Params().At(param.Idx).Type()
	}
	if _, ok := r.Pass().TypesInfo.TypeOf(source).(*types.Tuple); ok {
		// A spread multi-result call does not identify an expression for this parameter.
		return nil, false
	}
	if param.Path.IsRoot() {
		return source, true
	}
	// Selecting a field from nil would panic before the call returns.
	if ident, ok := ast.Unparen(source).(*ast.Ident); ok && r.isNil(ident) {
		return nil, false
	}
	structType := typeshelper.AsDeeplyStruct(sourceType)
	if structType == nil {
		return nil, false
	}
	return r.buildFieldPathSelector(source, structType, param.Path)
}

// bindExprNilabilityToContext supplies site from expr, using static nilability as a fallback.
func (r *RootAssertionNode) bindExprNilabilityToContext(expr ast.Expr, site annotation.Key) {
	consumer := &annotation.ConsumeTrigger{
		Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site}},
		Expr:       expr,
		Guards:     guard.NoGuards(),
	}
	if trackable, _ := r.ParseExprAsProducer(expr, false); trackable != nil {
		r.AddConsumption(consumer)
		return
	}
	r.AddNewTriggers(annotation.FullTrigger{
		Producer: &annotation.ProduceTrigger{Annotation: r.getShallowExprNilabilityProducer(expr), Expr: expr},
		Consumer: consumer,
	})
}

// getShallowExprNilabilityProducer returns the producer encoding the nilability of the value of expr: an
// always-nil producer for an explicit `nil`, the shallow producer of a trackable/nilable
// expression, or Never for a value that cannot be nil (e.g. `&A{}`, `new(A)`, `&local`).
func (r *RootAssertionNode) getShallowExprNilabilityProducer(expr ast.Expr) annotation.ProducingAnnotationTrigger {
	if ident, ok := ast.Unparen(expr).(*ast.Ident); ok && r.isNil(ident) {
		return &annotation.ProduceTriggerTautology{}
	}
	if _, _, ok := r.asStructAllocation(expr); ok {
		return &annotation.ProduceTriggerNever{}
	}
	// The address of any expression is a non-nil pointer at the shallow level. ParseExprAsProducer's
	// `&A{} ≡ A{}` rule re-parses the pointee for field tracking, so its GetShallow returns the
	// pointee's shallow nilability — which defaults to nilable for an unannotated local. That leaks
	// the pointee's deep concern into a shallow answer; short-circuit to Never here.
	if u, ok := ast.Unparen(expr).(*ast.UnaryExpr); ok && u.Op == token.AND {
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

// isLocalRootedValue reports whether expr is a function-local variable or a field chain rooted
// at one. Locals have no boundary annotation and must be resolved through flow-sensitive producers.
func (r *RootAssertionNode) isLocalRootedValue(expr ast.Expr) bool {
	base, _ := asthelper.SplitFieldChain(expr)
	if base == nil {
		return false
	}
	v, ok := r.ObjectOf(base).(*types.Var)
	if !ok {
		return false
	}
	funcObj := r.FuncObj()
	return !annotation.VarIsParam(funcObj, v) && !annotation.VarIsRecv(funcObj, v) && !annotation.VarIsGlobal(v)
}

// accessedFieldPath is one accessed field path under a boundary value, paired with the synthesized
// selector expression that reaches it.
type accessedFieldPath struct {
	sel  ast.Expr
	path annotation.FieldPath
}

// collectAccessedFieldPaths walks the live assertion subtree under node and collects accessed nilable
// field paths, deepest path first. prefix is the field path from the boundary value to node.
func (r *RootAssertionNode) collectAccessedFieldPaths(node AssertionNode, base ast.Expr, prefix annotation.FieldPath, seen map[*types.Struct]bool, out *[]accessedFieldPath) {
	for _, child := range node.Children() {
		fldNode, ok := child.(*fldAssertionNode)
		if !ok {
			continue
		}
		field := fldNode.decl
		sel := r.getSelectorExpr(field, base)
		path := prefix.Child(field.Name())
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
		// A result value supplied by a caller argument is resolved at each call site from that
		// argument.
		if r.resultValueHasParamSource(r.FuncObj(), retIdx) {
			continue
		}
		r.bindValueFieldsToContext(r.FuncObj(), retExpr, structType, annotation.StructFieldReturnContext, retIdx)
	}
}

// bindValueFieldsToContext connects the fields of the value produced by valExpr to the context
// site of targetFunc at (kind, index). For returns, targetFunc is the current function; for
// arguments and receivers, it is the callee. An inline struct allocation creates full triggers
// from its per-field shape; a struct-returning call links the destination site to the callee's
// return site (see bindCallResultFieldsToContext); any other trackable value adds consumers on
// `valExpr.<path>` for the field paths read at the boundary (boundaryReadPaths), matched later
// against the value's own field producers.
func (r *RootAssertionNode) bindValueFieldsToContext(targetFunc *types.Func, valExpr ast.Expr, structType *types.Struct, kind annotation.StructFieldContextKind, index int) {
	funcObj := targetFunc

	allocType, fieldInits, isAlloc := r.asStructAllocation(valExpr)
	if isAlloc {
		r.bindAllocationFieldsToContext(allocType, fieldInits, valExpr, annotation.FieldPath{}, funcObj, kind, index)
		return
	}

	// A struct-returning call forwarded across the boundary (e.g. `return f()` or `g(f())`)
	// has no local field values to inspect; instead, link this boundary site to the callee's
	// return site so the callee's per-field summary flows through.
	if call, ok := ast.Unparen(valExpr).(*ast.CallExpr); ok {
		r.bindCallResultFieldsToContext(call, valExpr, funcObj, kind, index)
		return
	}

	trackablePath, _ := r.ParseExprAsProducer(valExpr, false)
	if trackablePath == nil {
		return
	}

	for _, fieldPath := range r.boundaryReadPaths(funcObj, kind, index) {
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

// boundaryReadPaths returns the field paths that are read through the boundary context site of
// funcObj at (kind, index): the callee's param read-set for an argument or receiver, or the
// callers' return read-set for a return value.
func (r *RootAssertionNode) boundaryReadPaths(funcObj *types.Func, kind annotation.StructFieldContextKind, index int) []annotation.FieldPath {
	switch kind {
	case annotation.StructFieldParamContext:
		return r.functionContext.boundaryFieldEffects.ParamReadPaths(funcObj, index)
	case annotation.StructFieldReturnContext:
		return r.functionContext.boundaryFieldEffects.ReturnReadPaths(funcObj, index)
	}
	return nil
}

// bindCallResultFieldsToContext handles a call result that flows directly into a boundary,
// as in `return g()` or `h(g())`. The result of g is never stored in a variable, so we have
// no assertion-tree node to inspect for its fields. Instead, for each relevant field path,
// we link the two context sites directly: "if g's return field is nilable, then the
// destination site's field is nilable too". This works regardless of how g's return site was
// populated: inline allocation triggers (bindAllocationFieldsToContext), consumers matched
// against the returned value's field producers in the assertion tree (bindValueFieldsToContext),
// or inferred values imported from another package via the Facts mechanism.
//
// The field paths linked are the union of g's known return-effect paths and the paths the
// destination site is known to read, so `return g()` propagates nested fields as deeply as
// g constructs them, not just top-level ones. Calls with multiple results or a non-struct
// result are not handled here.
func (r *RootAssertionNode) bindCallResultFieldsToContext(call *ast.CallExpr, valExpr ast.Expr, targetFunc *types.Func, kind annotation.StructFieldContextKind, index int) {
	source, ok := typeshelper.ResolveStaticCallTarget(r.Pass().TypesInfo, call)
	if !ok || source.Origin == nil || source.Signature.Results().Len() != 1 {
		return
	}
	sourceType := typeshelper.AsDeeplyStruct(source.Signature.Results().At(0).Type())
	if sourceType == nil {
		return
	}
	pathSet := make(map[annotation.FieldPath]bool)
	for _, path := range r.functionContext.boundaryFieldEffects.ReturnEffectPaths(source.Origin, 0) {
		pathSet[path] = true
	}
	for _, path := range r.boundaryReadPaths(targetFunc, kind, index) {
		pathSet[path] = true
	}
	paths := make([]annotation.FieldPath, 0, len(pathSet))
	for path := range pathSet {
		paths = append(paths, path)
	}
	slices.SortFunc(paths, annotation.FieldPath.Compare)
	for _, fieldPath := range paths {
		// Param-sourced paths of the callee are caller-dependent: forwarding them
		// through the shared return sites would merge callers.
		if r.resultPathHasParamSource(source.Origin, 0, fieldPath) {
			continue
		}
		site := &annotation.StructFieldContextSite{FuncObj: targetFunc, Kind: kind, Index: index, Path: fieldPath}
		srcSite := &annotation.StructFieldContextSite{
			FuncObj: source.Origin, Kind: annotation.StructFieldReturnContext, Index: 0, Path: fieldPath,
		}
		r.AddNewTriggers(annotation.FullTrigger{
			Producer: &annotation.ProduceTrigger{
				Annotation: &annotation.StructFieldFromContext{TriggerIfNilable: &annotation.TriggerIfNilable{Ann: srcSite}},
				Expr:       valExpr,
			},
			Consumer: &annotation.ConsumeTrigger{
				Annotation: &annotation.StructFieldToContext{TriggerIfNonNil: &annotation.TriggerIfNonNil{Ann: site}},
				Expr:       valExpr,
				Guards:     guard.NoGuards(),
			},
		})
	}
}

// bindAllocationFieldsToContext binds the per-field nilability of an inline struct allocation to
// the boundary context site of funcObj at (kind, index).
func (r *RootAssertionNode) bindAllocationFieldsToContext(structType *types.Struct, fieldInits []ast.Expr, valExpr ast.Expr, prefix annotation.FieldPath, funcObj *types.Func, kind annotation.StructFieldContextKind, index int) {
	numFields := structType.NumFields()
	for i := range numFields {
		field := structType.Field(i)
		fieldVal := asthelper.GetFieldVal(fieldInits, field.Name(), numFields, i)
		nilable := !typeshelper.TypeBarsNilness(field.Type())
		path := prefix.Child(field.Name())

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
	if typeshelper.AsDeeplyStruct(v.Type()) == nil {
		return
	}
	path, _ := r.ParseExprAsProducer(builtExpr, false)
	node, _ := r.lookupPath(path)
	if node == nil {
		return
	}
	var paths []accessedFieldPath
	r.collectAccessedFieldPaths(node, builtExpr, annotation.FieldPath{}, make(map[*types.Struct]bool), &paths)
	for _, p := range paths {
		site := &annotation.StructFieldContextSite{
			FuncObj: r.FuncObj(), Kind: annotation.StructFieldParamContext, Index: idx, Path: p.path,
		}
		r.AddProduction(&annotation.ProduceTrigger{
			Annotation: &annotation.StructFieldFromContext{
				TriggerIfNilable: &annotation.TriggerIfNilable{Ann: site},
			},
			Expr: p.sel,
		})
	}
}

// buildFieldPathSelector builds the selector expression reaching base.<path>, where path is a
// field path under base's struct type.
func (r *RootAssertionNode) buildFieldPathSelector(base ast.Expr, structType *types.Struct, path annotation.FieldPath) (ast.Expr, bool) {
	cur := base
	curStruct := structType
	for _, name := range path.Segments() {
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
func (r *RootAssertionNode) bindArgAndReceiverFieldsToContext(call *ast.CallExpr, target typeshelper.StaticCallTarget) {
	if recv := target.Signature.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if structType := typeshelper.AsDeeplyStruct(recv.Type()); structType != nil {
				r.bindValueFieldsToContext(target.Origin, sel.X, structType, annotation.StructFieldParamContext, annotation.ReceiverParamIndex)
			}
		}
	}

	for argIdx, arg := range call.Args {
		if argIdx >= target.Signature.Params().Len() {
			break
		}
		structType := typeshelper.AsDeeplyStruct(target.Signature.Params().At(argIdx).Type())
		if structType == nil {
			continue
		}
		r.bindValueFieldsToContext(target.Origin, arg, structType, annotation.StructFieldParamContext, argIdx)
	}
}

// addCallParamOutFieldProducers attaches a callee's post-call field state to its concrete arguments
// and receiver. For `f(x)`, if f's parameter 0 may write `b.c`, it produces
// `x.b.c <- PARAM_OUT(f, 0, "b.c")`. A post-call dereference of x.b.c then consumes f's output
// summary, while fields absent from the write set retain their pre-call producers.
func (r *RootAssertionNode) addCallParamOutFieldProducers(call *ast.CallExpr, target typeshelper.StaticCallTarget) {
	produce := func(arg ast.Expr, structType *types.Struct, index int) {
		paths := r.functionContext.boundaryFieldEffects.ParamWritePaths(target.Origin, index)
		// AddProduction detaches a matched subtree, so nested paths must be produced first.
		sort.SliceStable(paths, func(i, j int) bool {
			return paths[i].NumSegments() > paths[j].NumSegments()
		})
		for _, fieldPath := range paths {
			fieldExpr, ok := r.buildFieldPathSelector(arg, structType, fieldPath)
			if !ok {
				continue
			}
			site := &annotation.StructFieldContextSite{
				FuncObj: target.Origin,
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

	if recv := target.Signature.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			if structType := typeshelper.AsDeeplyStruct(recv.Type()); structType != nil {
				produce(sel.X, structType, annotation.ReceiverParamIndex)
			}
		}
	}
	for index, arg := range call.Args {
		if index >= target.Signature.Params().Len() {
			break
		}
		if structType := typeshelper.AsDeeplyStruct(target.Signature.Params().At(index).Type()); structType != nil {
			produce(arg, structType, index)
		}
	}
}

// bindForwardedParamOut connects a callee's output summary to a forwarder's output summary. For
// `func g(p *A) { f(p) }`, when f's parameter 0 may write b.c, it adds
// `PARAM_OUT(f, 0, b.c) -> PARAM_OUT(g, 0, b.c)`. A field prefix is retained, so passing
// p.inner to f instead targets `PARAM_OUT(g, 0, inner.b.c)`. The write summary is already closed
// over these edges; this supplies each inherited path's context value.
func (r *RootAssertionNode) bindForwardedParamOut(call *ast.CallExpr, target typeshelper.StaticCallTarget) {
	link := func(calleeIndex int, arg ast.Expr) {
		base, prefixSegments := asthelper.SplitFieldChain(arg)
		if base == nil {
			return
		}
		prefix := annotation.NewFieldPath(prefixSegments...)
		param, ok := r.ObjectOf(base).(*types.Var)
		if !ok {
			return
		}
		callerIndex, ok := r.getParamIndex(param)
		if !ok {
			return
		}
		for _, calleePath := range r.functionContext.boundaryFieldEffects.ParamWritePaths(target.Origin, calleeIndex) {
			fieldPath := prefix.Join(calleePath)
			source := &annotation.StructFieldContextSite{
				FuncObj: target.Origin, Kind: annotation.StructFieldParamOutContext, Index: calleeIndex, Path: calleePath,
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

	if recv := target.Signature.Recv(); recv != nil {
		if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
			link(annotation.ReceiverParamIndex, sel.X)
		}
	}
	for index, arg := range call.Args {
		if index >= target.Signature.Params().Len() {
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
	base, segments := asthelper.SplitFieldChain(lhs)
	if base == nil || len(segments) == 0 {
		return
	}
	fieldPath := annotation.NewFieldPath(segments...)
	param, ok := r.ObjectOf(base).(*types.Var)
	if !ok {
		return
	}
	index, ok := r.getParamIndex(param)
	if !ok {
		return
	}
	for _, path := range r.functionContext.boundaryFieldEffects.ParamWritePaths(r.FuncObj(), index) {
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
		// A local-rooted RHS has no boundary annotation to snapshot; attach the param-out
		// supply as a consumption so backpropagation resolves its flow-sensitive producer.
		// Any other RHS snapshots to its provable boundary nilability below.
		if r.isLocalRootedValue(rhs) {
			consumer.Expr = rhs
			r.AddConsumption(consumer)
			return
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
