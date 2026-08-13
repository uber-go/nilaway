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
	"golang.org/x/tools/go/ssa"
)

// lazyInitSuppressed recognizes only the struct-init-v2 producer/field-access
// pair.  In particular, it does not alter ordinary field-access diagnostics.
func lazyInitSuppressed(fc FunctionContext, producer *annotation.ProduceTrigger, consumer *annotation.ConsumeTrigger) bool {
	if fc.ssaFunc == nil || producer == nil || consumer == nil {
		return false
	}
	if _, ok := producer.Annotation.(*annotation.StructFieldNil); !ok {
		return false
	}
	if _, ok := consumer.Annotation.(*annotation.FldAccess); !ok {
		return false
	}

	fieldAddr, field := fieldAddrForExpr(fc, consumer.Expr)
	if fieldAddr == nil || field == nil || !baseDefinitelyNonNil(fieldAddr.X, fieldAddr) {
		return false
	}

	// An unknown or nil store anywhere in this function invalidates this coarse
	// proof.  This deliberately errs on the side of retaining the diagnostic.
	establishing := make(map[*ssa.BasicBlock]bool)
	for _, block := range fc.ssaFunc.Blocks {
		for _, instr := range block.Instrs {
			store, ok := instr.(*ssa.Store)
			if !ok || !sameField(store.Addr, fieldAddr) {
				continue
			}
			if !definitelyNonNil(store.Val) {
				return false
			}
			if instructionBefore(block, instr, fieldAddr) {
				establishing[block] = true
			}
		}
	}

	// A guard establishes the field only on its non-nil edge.  This covers both
	// direct field guards and nil-safe GetF-style guards without treating a
	// block reached from the nil edge as established too.
	establishedEdges := make(map[ssaEdge]bool)
	for _, block := range fc.ssaFunc.Blocks {
		eqSucc, nonNilSucc, ok := nilGuard(block, fieldAddr)
		if ok && eqSucc != nil && nonNilSucc != nil {
			establishedEdges[ssaEdge{from: block, to: nonNilSucc}] = true
		}
	}

	return allPathsEstablished(fieldAddr.Block(), establishing, establishedEdges, map[*ssa.BasicBlock]bool{}, map[*ssa.BasicBlock]bool{})
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

func sameField(addr ssa.Value, want *ssa.FieldAddr) bool {
	got, ok := addr.(*ssa.FieldAddr)
	return ok && got.Field == want.Field && got.X == want.X
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

func baseDefinitelyNonNil(base ssa.Value, deref *ssa.FieldAddr) bool {
	switch base := base.(type) {
	case *ssa.Global:
		return true
	case *ssa.Alloc:
		return base.Block().Dominates(deref.Block()) &&
			(base.Block() != deref.Block() || instructionBefore(base.Block(), base, deref))
	}
	return false
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

func nilGuard(block *ssa.BasicBlock, field *ssa.FieldAddr) (*ssa.BasicBlock, *ssa.BasicBlock, bool) {
	if len(block.Instrs) == 0 || len(block.Succs) != 2 {
		return nil, nil, false
	}
	ifInstr, ok := block.Instrs[len(block.Instrs)-1].(*ssa.If)
	if !ok {
		return nil, nil, false
	}
	condition, ok := ifInstr.Cond.(*ssa.BinOp)
	if !ok || condition.Op != token.EQL || !isNilConst(condition.X) && !isNilConst(condition.Y) {
		return nil, nil, false
	}
	value := condition.X
	if isNilConst(value) {
		value = condition.Y
	}
	if candidate := directFieldValue(value); candidate != nil && sameField(candidate, field) {
		return block.Succs[0], block.Succs[1], true
	}
	call, ok := value.(*ssa.Call)
	if !ok {
		return nil, nil, false
	}
	callee := call.Call.StaticCallee()
	if callee == nil || len(callee.Name()) < 3 || callee.Name()[:3] != "Get" {
		return nil, nil, false
	}
	// Getter guards are correlated only by the exact base SSA value and the
	// conventional Get<Field> name; arbitrary calls are not trusted.
	if callee.Name()[3:] != field.Name() || len(call.Call.Args) == 0 || call.Call.Args[0] != field.X {
		return nil, nil, false
	}
	return block.Succs[0], block.Succs[1], true
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
