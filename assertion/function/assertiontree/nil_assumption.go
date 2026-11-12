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
// deletes every consumer carrying it. Crossing a write to the variable strips the nonce again, so a
// tag picked up after the write never reaches a check before it.
//
// A variable is stable when every write to it is an assignment or declaration visible in the
// function's own CFG: the receiver, the parameters, and the locals declared in the body, minus
// variables that are address-taken, ranged into, or assigned inside a nested function literal.
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

	stableVars, unstable := classifyVars(body, pass.TypesInfo)
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
	for variable := range unstable {
		delete(stableVars, variable)
	}

	return &nilAssumptions{
		nonceGenerator: nonceGenerator,
		stableVars:     stableVars,
		nonces:         make(map[*types.Var]guard.Nonce),
	}
}

// classifyVars returns the variables declared in body and the variables whose value may change
// somewhere backpropagation cannot see: through their address, through a range clause, or inside a
// nested function literal.
func classifyVars(body *ast.BlockStmt, info *types.Info) (declared, unstable map[*types.Var]struct{}) {
	declared = make(map[*types.Var]struct{})
	unstable = make(map[*types.Var]struct{})
	record := func(set map[*types.Var]struct{}, expr ast.Expr) {
		if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
			if variable, ok := info.ObjectOf(ident).(*types.Var); ok {
				set[variable] = struct{}{}
			}
		}
	}

	var walk func(subtree ast.Node, insideClosure bool)
	walk = func(subtree ast.Node, insideClosure bool) {
		ast.Inspect(subtree, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncLit:
				walk(node.Body, true)
				return false
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					if insideClosure {
						record(unstable, lhs)
					} else if node.Tok == token.DEFINE {
						record(declared, lhs)
					}
				}
			case *ast.ValueSpec:
				if insideClosure {
					break
				}
				for _, name := range node.Names {
					record(declared, name)
				}
			case *ast.RangeStmt:
				// The preprocessor rewrites range clauses into assignments that stripOnWrite sees,
				// but it gives up silently when the CFG has an unexpected shape, so ranged
				// variables are excluded outright rather than trusting that rewrite.
				record(unstable, node.Key)
				record(unstable, node.Value)
			case *ast.UnaryExpr:
				if node.Op == token.AND {
					record(unstable, node.X)
				}
			case *ast.SelectorExpr:
				if takesAddressImplicitly(node, info) {
					record(unstable, node.X)
				}
			}
			return true
		})
	}
	walk(body, false)
	return declared, unstable
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

// stripOnWrite removes the nonces of the stable variables that node assigns or declares from
// every pending consumer. Only *ast.AssignStmt and *ast.ValueSpec nodes write stable variables.
func (assumptions *nilAssumptions) stripOnWrite(root *RootAssertionNode, node ast.Node) {
	if len(assumptions.nonces) == 0 {
		return
	}

	var targets []ast.Expr
	switch node := node.(type) {
	case *ast.AssignStmt:
		targets = node.Lhs
	case *ast.ValueSpec:
		targets = toExprSlice(node.Names)
	default:
		return
	}

	var stale []guard.Nonce
	for _, target := range targets {
		ident, isIdent := ast.Unparen(target).(*ast.Ident)
		if !isIdent {
			continue
		}
		variable, isVar := root.Pass().TypesInfo.ObjectOf(ident).(*types.Var)
		if !isVar {
			continue
		}
		if nonce, checked := assumptions.nonces[variable]; checked {
			stale = append(stale, nonce)
		}
	}
	if len(stale) == 0 {
		return
	}

	carriesStale := func(consumer *annotation.ConsumeTrigger) bool {
		for _, nonce := range stale {
			if consumer.Guards.Contains(nonce) {
				return true
			}
		}
		return false
	}
	mapConsumers(root, func(consumers []*annotation.ConsumeTrigger) []*annotation.ConsumeTrigger {
		if !slices.ContainsFunc(consumers, carriesStale) {
			// Most writes find nothing tagged, so keep the shared slice instead of copying it.
			return consumers
		}
		out := make([]*annotation.ConsumeTrigger, 0, len(consumers))
		for _, consumer := range consumers {
			if !carriesStale(consumer) {
				out = append(out, consumer)
				continue
			}
			// Copy before mutating: the same trigger may be shared with other blocks' trees.
			stripped := consumer.Copy()
			stripped.Guards.Remove(stale...)
			out = append(out, stripped)
		}
		return out
	})
}

// mapConsumers replaces the consume triggers of every node below root with the result of applying
// transform to them.
func mapConsumers(node AssertionNode, transform func([]*annotation.ConsumeTrigger) []*annotation.ConsumeTrigger) {
	for _, child := range node.Children() {
		child.SetConsumeTriggers(transform(child.ConsumeTriggers()))
		mapConsumers(child, transform)
	}
}
