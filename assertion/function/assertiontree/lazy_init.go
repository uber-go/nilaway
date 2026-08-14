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
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/typeshelper"
	"golang.org/x/tools/go/ssa"
)

// lazyInitSuppressed recognizes same-package StructFieldNil producers and
// caller-pinned StructFieldFromContext producers scoped to this function's own param context.
func lazyInitSuppressed(fc FunctionContext, producer *annotation.ProduceTrigger, consumer *annotation.ConsumeTrigger) bool {
	if fc.ssaFunc == nil || producer == nil || consumer == nil {
		return false
	}
	checkAliasedStores := false
	switch producerAnnotation := producer.Annotation.(type) {
	case *annotation.StructFieldNil:
		checkAliasedStores = true
	case *annotation.StructFieldFromContext:
		site, ok := producerAnnotation.Ann.(*annotation.StructFieldContextSite)
		if !ok || site.Kind != annotation.StructFieldParamContext || site.FuncObj != fc.ssaFunc.Object() {
			return false
		}
		checkAliasedStores = true
	default:
		return false
	}
	if _, ok := consumer.Annotation.(*annotation.FldAccess); !ok {
		return false
	}

	fieldAddr, field := fieldAddrForExpr(fc, consumer.Expr)
	if fieldAddr == nil || field == nil || !baseDefinitelyNonNil(fc, fieldAddr.X, fieldAddr, checkAliasedStores) {
		return false
	}

	return fieldEstablishedOnAllPaths(fc, fieldAddr, fieldAddr, checkAliasedStores)
}

func fieldAddrForExpr(fc FunctionContext, expr ast.Expr) (*ssa.FieldAddr, *types.Var) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	selection := fc.pass.TypesInfo.Selections[sel]
	if selection == nil {
		return nil, nil
	}
	field, ok := selection.Obj().(*types.Var)
	if !ok {
		return nil, nil
	}
	indexes := selection.Index()
	if len(indexes) == 0 {
		return nil, nil
	}
	var found *ssa.FieldAddr
	for _, block := range fc.ssaFunc.Blocks {
		for _, instr := range block.Instrs {
			fa, ok := instr.(*ssa.FieldAddr)
			if !ok || fa.Field != indexes[len(indexes)-1] {
				continue
			}
			if fa.Pos() == expr.Pos() || fa.Pos() == sel.Sel.Pos() || fa.Pos() == sel.X.Pos() {
				if found != nil {
					return nil, nil
				}
				found = fa
			}
		}
	}
	return found, field
}

func fieldAtIndex(t types.Type, index int) *types.Var {
	s := typeshelper.AsDeeplyStruct(t)
	if s == nil || index < 0 || index >= s.NumFields() {
		return nil
	}
	return s.Field(index)
}

func sameField(addr ssa.Value, want *ssa.FieldAddr) bool {
	got, ok := addr.(*ssa.FieldAddr)
	return ok && got.X == want.X && got.Field == want.Field
}

func compatibleField(got, want *ssa.FieldAddr) bool {
	return got.Field == want.Field && types.Identical(got.X.Type(), want.X.Type())
}

func mayAliasBase(a, b ssa.Value) bool {
	if a == b {
		return true
	}
	_, aAlloc := a.(*ssa.Alloc)
	_, bAlloc := b.(*ssa.Alloc)
	if aAlloc && bAlloc {
		return false // distinct direct allocations cannot alias
	}
	return true // phi, parameter, load, call/getter, unknown: fail closed
}

func definitelyNonNil(v ssa.Value) bool {
	switch value := v.(type) {
	case *ssa.Alloc, *ssa.MakeSlice, *ssa.MakeMap, *ssa.MakeChan, *ssa.Function, *ssa.Global:
		return true
	case *ssa.Const:
		return !value.IsNil()
	case *ssa.UnOp:
		_, global := value.X.(*ssa.Global)
		return value.Op == token.MUL && global
	default:
		return false
	}
}

func baseDefinitelyNonNil(fc FunctionContext, base ssa.Value, deref *ssa.FieldAddr, checkAliasedStores bool) bool {
	switch base := base.(type) {
	case *ssa.Global:
		return true
	case *ssa.Alloc:
		return base.Block().Dominates(deref.Block()) &&
			(base.Block() != deref.Block() || instructionBefore(base.Block(), base, deref))
	case *ssa.Parameter:
		return parameterIsStable(fc, base)
	case *ssa.Phi:
		return phiDefinitelyNonNil(fc, base, deref, map[*ssa.Phi]bool{})
	case *ssa.UnOp:
		if base.Op == token.MUL {
			if fa, ok := base.X.(*ssa.FieldAddr); ok {
				return proveFieldNonNil(fc, fa, deref, map[*ssa.FieldAddr]bool{}, checkAliasedStores)
			}
		}
	}
	return false
}

func phiDefinitelyNonNil(fc FunctionContext, phi *ssa.Phi, deref *ssa.FieldAddr, visiting map[*ssa.Phi]bool) bool {
	if visiting[phi] || len(phi.Edges) != len(phi.Block().Preds) {
		return false
	}
	visiting[phi] = true
	defer delete(visiting, phi)
	if !phi.Block().Dominates(deref.Block()) {
		return false
	}
	for i, edge := range phi.Edges {
		if definitelyNonNil(edge) {
			continue
		}
		if nested, ok := edge.(*ssa.Phi); ok {
			if !phiDefinitelyNonNil(fc, nested, deref, visiting) {
				return false
			}
			continue
		}
		pred := phi.Block().Preds[i]
		nonNilSucc, guardOK := nilGuardValue(pred, edge)
		if !guardOK || nonNilSucc != phi.Block() {
			return false
		}
	}
	return true
}

func parameterIsStable(fc FunctionContext, param *ssa.Parameter) bool {
	for _, block := range fc.ssaFunc.Blocks {
		for _, instr := range block.Instrs {
			if store, ok := instr.(*ssa.Store); ok && store.Addr == param {
				return false
			}
		}
	}
	return true
}

func proveFieldNonNil(fc FunctionContext, field, deref *ssa.FieldAddr, visiting map[*ssa.FieldAddr]bool, checkAliasedStores bool) bool {
	if visiting[field] {
		return false
	}
	visiting[field] = true
	defer delete(visiting, field)
	if !baseDefinitelyNonNil(fc, field.X, field, checkAliasedStores) {
		return false
	}
	return fieldEstablishedOnAllPaths(fc, field, deref, checkAliasedStores)
}

// fieldEstablishedOnAllPaths reports whether every store to field keeps it non-nil and,
// together with non-nil guard edges, establishes the field on all paths to deref.
func fieldEstablishedOnAllPaths(fc FunctionContext, field, deref *ssa.FieldAddr, checkAliasedStores bool) bool {
	establishing := make(map[*ssa.BasicBlock]bool)
	for _, block := range fc.ssaFunc.Blocks {
		for _, instr := range block.Instrs {
			store, ok := instr.(*ssa.Store)
			if !ok {
				continue
			}
			fa, ok := store.Addr.(*ssa.FieldAddr)
			if !ok || !compatibleField(fa, field) {
				continue
			}
			if sameField(store.Addr, field) {
				if !definitelyNonNil(store.Val) {
					return false
				}
				if block != deref.Block() || instructionBefore(block, instr, deref) {
					establishing[block] = true
				}
			} else if checkAliasedStores && mayAliasBase(fa.X, field.X) {
				if !definitelyNonNil(store.Val) {
					return false
				}
			}
		}
	}

	// A guard establishes the field only on its non-nil edge.  This covers both
	// direct field guards and nil-safe GetF-style guards without treating a
	// block reached from the nil edge as established too.
	establishedEdges := make(map[ssaEdge]bool)
	for _, block := range fc.ssaFunc.Blocks {
		_, nonNilSucc, ok := nilGuard(fc, block, field)
		if ok && nonNilSucc != nil {
			establishedEdges[ssaEdge{from: block, to: nonNilSucc}] = true
		}
	}
	return allPathsEstablished(deref.Block(), establishing, establishedEdges, map[*ssa.BasicBlock]bool{}, map[*ssa.BasicBlock]bool{})
}

func instructionBefore(block *ssa.BasicBlock, first ssa.Instruction, second ssa.Instruction) bool {
	firstIndex, secondIndex := -1, -1
	for i, instr := range block.Instrs {
		if instr == first {
			firstIndex = i
		}
		if instr == second {
			secondIndex = i
		}
	}
	return firstIndex >= 0 && secondIndex >= 0 && firstIndex < secondIndex
}

func nilGuard(fc FunctionContext, block *ssa.BasicBlock, field *ssa.FieldAddr) (*ssa.BasicBlock, *ssa.BasicBlock, bool) {
	value, eqSucc, nonNilSucc, ok := nilGuardCond(block)
	if !ok {
		return nil, nil, false
	}
	if candidate := directFieldValue(value); candidate != nil {
		if candidate.X == field.X && candidate.Field == field.Field {
			return eqSucc, nonNilSucc, true
		}
	}
	call, ok := value.(*ssa.Call)
	if !ok {
		return nil, nil, false
	}
	callee := call.Call.StaticCallee()
	if callee == nil || callee.Pkg == nil {
		return nil, nil, false
	}
	fieldVar := fieldAtIndex(field.X.Type(), field.Field)
	if fieldVar == nil || !strings.HasPrefix(callee.Name(), "Get") || callee.Name()[3:] != fieldVar.Name() || len(call.Call.Args) == 0 || call.Call.Args[0] != field.X || !validGetter(callee, field.Field) {
		return nil, nil, false
	}
	return eqSucc, nonNilSucc, true
}

func validGetter(getter *ssa.Function, field int) bool {
	if getter.Blocks == nil || len(getter.Params) == 0 {
		return false
	}
	hasExactLoad := false
	for _, block := range getter.Blocks {
		for _, instr := range block.Instrs {
			switch instr.(type) {
			case *ssa.Store, *ssa.Call, *ssa.Go, *ssa.Defer, *ssa.Send:
				return false
			}
			ret, ok := instr.(*ssa.Return)
			if !ok {
				continue
			}
			if len(ret.Results) != 1 {
				return false
			}
			if isNilConst(ret.Results[0]) {
				continue
			}
			loaded := directFieldValue(ret.Results[0])
			if loaded == nil || loaded.X != getter.Params[0] || loaded.Field != field {
				return false
			}
			hasExactLoad = true
		}
	}
	return hasExactLoad
}

func nilGuardValue(block *ssa.BasicBlock, value ssa.Value) (nonNilSucc *ssa.BasicBlock, ok bool) {
	guardValue, _, nonNilSucc, ok := nilGuardCond(block)
	if ok && guardValue == value {
		return nonNilSucc, true
	}
	return nil, false
}

func nilGuardCond(block *ssa.BasicBlock) (value ssa.Value, eqSucc, nonNilSucc *ssa.BasicBlock, ok bool) {
	if len(block.Instrs) == 0 || len(block.Succs) != 2 {
		return nil, nil, nil, false
	}
	ifInstr, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if !ok {
		return nil, nil, nil, false
	}
	condition, ok := ifInstr.Cond.(*ssa.BinOp)
	if !ok || condition.Op != token.EQL || isNilConst(condition.X) == isNilConst(condition.Y) {
		return nil, nil, nil, false
	}
	value = condition.X
	if isNilConst(value) {
		value = condition.Y
	}
	return value, block.Succs[0], block.Succs[1], true
}

// isNilConst mirrors the identical helper in functioncontracts/infer.go; kept local to avoid a cross-package SSA utility for one use.
func isNilConst(v ssa.Value) bool {
	constant, ok := v.(*ssa.Const)
	return ok && constant.IsNil()
}

func directFieldValue(v ssa.Value) *ssa.FieldAddr {
	load, ok := v.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return nil
	}
	field, _ := load.X.(*ssa.FieldAddr)
	return field
}

type ssaEdge struct {
	from *ssa.BasicBlock
	to   *ssa.BasicBlock
}

func allPathsEstablished(block *ssa.BasicBlock, establishing map[*ssa.BasicBlock]bool, establishedEdges map[ssaEdge]bool, visiting, memo map[*ssa.BasicBlock]bool) bool {
	if establishing[block] {
		return true
	}
	if result, ok := memo[block]; ok {
		return result
	}
	if visiting[block] {
		return false
	}
	visiting[block] = true
	if len(block.Preds) == 0 {
		delete(visiting, block)
		memo[block] = false
		return false
	}
	for _, pred := range block.Preds {
		if !establishedEdges[ssaEdge{from: pred, to: block}] && !allPathsEstablished(pred, establishing, establishedEdges, visiting, memo) {
			delete(visiting, block)
			memo[block] = false
			return false
		}
	}
	delete(visiting, block)
	memo[block] = true
	return true
}
