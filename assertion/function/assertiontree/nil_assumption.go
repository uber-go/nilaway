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

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/guard"
	"go.uber.org/nilaway/util/analysishelper"
)

// nilAssumptions prunes consume triggers that can only be reached through contradictory nil checks
// on the same variable spread across separate statements.
//
// Backpropagation records a non-nil fact when it crosses the "v is non-nil" edge of a check, but it
// records nothing on the "v is nil" edge. A path that crosses the nil edge of one check and later
// (in program order, earlier in backpropagation) the non-nil edge of another check on the same
// variable is therefore analyzed as feasible even though no execution can take it. See issue 378.
//
// Every stable variable gets one nonce. The nil edge of a check on that variable tags every pending
// consumer in the assertion tree with the nonce. The non-nil edge of any check on the same variable
// deletes every consumer carrying it.
type nilAssumptions struct {
	nonceGenerator *guard.NonceGenerator
	stableVars     map[*types.Var]struct{}
	// nonces is filled lazily so that functions without a check on a stable variable allocate
	// nothing.
	nonces map[*types.Var]guard.Nonce
}

// newNilAssumptions collects the stable variables of the function. When funcLit is non-nil the
// parameters come from the literal itself, not from the fake declaration, because the fake
// declaration lists every captured variable as a parameter.
func newNilAssumptions(
	pass *analysishelper.EnhancedPass,
	funcDecl *ast.FuncDecl,
	funcLit *ast.FuncLit,
	nonceGenerator *guard.NonceGenerator,
) *nilAssumptions {
	var (
		fieldLists []*ast.FieldList
		body       *ast.BlockStmt
	)
	if funcLit != nil {
		fieldLists = append(fieldLists, funcLit.Type.Params)
		body = funcLit.Body
	} else {
		// The function analyzer skips declarations without a body before building a context.
		fieldLists = append(fieldLists, funcDecl.Recv, funcDecl.Type.Params)
		body = funcDecl.Body
	}

	stableVars := make(map[*types.Var]struct{})
	for _, fieldList := range fieldLists {
		if fieldList == nil {
			continue
		}
		for _, field := range fieldList.List {
			for _, name := range field.Names {
				if variable, ok := pass.TypesInfo.Defs[name].(*types.Var); ok {
					stableVars[variable] = struct{}{}
				}
			}
		}
	}
	for variable := range writtenVars(body, pass.TypesInfo) {
		delete(stableVars, variable)
	}

	return &nilAssumptions{
		nonceGenerator: nonceGenerator,
		stableVars:     stableVars,
		nonces:         make(map[*types.Var]guard.Nonce),
	}
}

// writtenVars returns the variables whose value may change somewhere in body, including inside
// nested function literals.
func writtenVars(body *ast.BlockStmt, info *types.Info) map[*types.Var]struct{} {
	written := make(map[*types.Var]struct{})
	record := func(expr ast.Expr) {
		if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
			if variable, ok := info.ObjectOf(ident).(*types.Var); ok {
				written[variable] = struct{}{}
			}
		}
	}

	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				record(lhs)
			}
		case *ast.RangeStmt:
			record(node.Key)
			record(node.Value)
		case *ast.UnaryExpr:
			if node.Op == token.AND {
				record(node.X)
			}
		case *ast.SelectorExpr:
			if takesAddressImplicitly(node, info) {
				record(node.X)
			}
		}
		return true
	})
	return written
}

// takesAddressImplicitly reports whether sel is a pointer-receiver method selected on a value that
// is not itself a pointer.
func takesAddressImplicitly(sel *ast.SelectorExpr, info *types.Info) bool {
	selection, ok := info.Selections[sel]
	if !ok || selection.Kind() != types.MethodVal {
		return false
	}

	// A method value always selects a *types.Func that has a receiver.
	receiver := selection.Obj().(*types.Func).Signature().Recv()
	_, receiverIsPointer := receiver.Type().(*types.Pointer)
	_, operandIsPointer := info.TypeOf(sel.X).(*types.Pointer)
	return receiverIsPointer && !operandIsPointer
}

// edgeFuncs returns the functions to run on the true (onNil) and false (onNonNil) edges of cond
// when cond is v == nil for a stable variable v. The preprocessor canonicalizes v != nil,
// nil == v, and short-circuit operators into that shape before this runs.
func (assumptions *nilAssumptions) edgeFuncs(
	pass *analysishelper.EnhancedPass, cond ast.Expr,
) (onNil, onNonNil RootFunc, ok bool) {
	binExpr, isBinary := ast.Unparen(cond).(*ast.BinaryExpr)
	if !isBinary || binExpr.Op != token.EQL {
		return nil, nil, false
	}

	ident, isIdent := ast.Unparen(binExpr.X).(*ast.Ident)
	if !isIdent || !pass.IsNil(binExpr.Y) {
		return nil, nil, false
	}

	// An identifier that is not a variable, such as a function name, yields a nil key, and a nil
	// key is never stable.
	variable, _ := pass.TypesInfo.ObjectOf(ident).(*types.Var)
	if _, isStable := assumptions.stableVars[variable]; !isStable {
		return nil, nil, false
	}

	nonce, seen := assumptions.nonces[variable]
	if !seen {
		nonce = assumptions.nonceGenerator.Next(cond)
		assumptions.nonces[variable] = nonce
	}

	onNil = func(root *RootAssertionNode) {
		mapConsumers(root, func(consumers []*annotation.ConsumeTrigger) []*annotation.ConsumeTrigger {
			return annotation.ConsumeTriggerSliceAsGuarded(consumers, nonce)
		})
	}

	onNonNil = func(root *RootAssertionNode) {
		mapConsumers(root, func(consumers []*annotation.ConsumeTrigger) []*annotation.ConsumeTrigger {
			var kept []*annotation.ConsumeTrigger
			for _, consumer := range consumers {
				if !consumer.Guards.Contains(nonce) {
					kept = append(kept, consumer)
				}
			}
			return kept
		})
	}

	return onNil, onNonNil, true
}

// mapConsumers replaces the consume triggers of every node below root with the result of applying
// transform to them.
func mapConsumers(node AssertionNode, transform func([]*annotation.ConsumeTrigger) []*annotation.ConsumeTrigger) {
	for _, child := range node.Children() {
		child.SetConsumeTriggers(transform(child.ConsumeTriggers()))
		mapConsumers(child, transform)
	}
}
