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

package anonymousfunc

import (
	"fmt"
	"go/ast"
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/asthelper"
)

// collectClosure collects a set of variables in closure for the given function literal and updates
// the closureMap accordingly. To do that, it iterates over all nodes inside the body of the given
// function literal:
// (1) If the node is a function literal, it recursively calls collectClosure and because
// ast.Inspect uses depth-first search, the innermost function literal will be analyzed first. The
// collected closure variables will also be appended to those of the enclosing function literals,
// modulo the ones defined in the scope of the enclosing function literals.
// (2) If the node is an ident node that represents a variable which is not global, it updates the
// closure set if the variable is not declared in the function literal or any of its nested scopes.
func collectClosure(funcLit *ast.FuncLit, pass *analysishelper.EnhancedPass, closureMap map[*ast.FuncLit][]*VarInfo) {
	// Retrieve the scope of the given function literal
	scope := pass.TypesInfo.Scopes[funcLit.Type]

	var varsFromClosure []*VarInfo
	visited := make(map[*types.Var]bool)
	ast.Inspect(funcLit.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		// closureVar variables required by inner function literals are also required by the current
		// function literal if they are from outer closures.
		case *ast.FuncLit:
			collectClosure(node, pass, closureMap)

			// Any outer closureVar variables that the nested function literals use should also be
			// required by the current function, so we do a post-processing here to add those
			// variables.
			for _, closureVar := range closureMap[node] {
				obj, ok := pass.TypesInfo.ObjectOf(closureVar.Ident).(*types.Var)
				if !ok {
					panic(fmt.Sprintf("identifier %s passed as a variable could not be looked up as one", closureVar.Ident))
				}

				// Update varsFromClosure with ident if it is not declared in this function literal
				if !isDeclaredIn(obj, scope) {
					varsFromClosure = append(varsFromClosure, closureVar)
					visited[obj] = true
				}
			}

			// Stop the recursion of ast.Inspect since further recursion was already handled by the
			// recursive call to collectClosure above.
			return false

		case *ast.Ident:
			// Skip the blank identifier. The type checker creates a *types.Var for `_` in `:=`
			// assignments but never registers it in any scope, so the scope check below would
			// incorrectly treat it as a captured variable. Blank identifiers are write-only
			// discard targets and cannot be closure variables.
			if asthelper.IsEmptyExpr(node) {
				return false
			}

			// Skip if node is not a variable
			if node.Obj == nil || node.Obj.Kind != ast.Var {
				return false
			}

			// Skip the symbolic variable in a type switch header, e.g., `n` in
			// `switch n := v.(type)`. It does not denote a variable by itself: the type checker
			// records a nil object for it in Defs and declares a separate implicit variable in
			// each case clause instead (see types.Info.Implicits). The uses of `n` in the case
			// clauses refer to those implicit variables and are handled below.
			if obj, ok := pass.TypesInfo.Defs[node]; ok && obj == nil {
				return false
			}

			// Get the underlying object for the identifier
			obj, ok := pass.TypesInfo.ObjectOf(node).(*types.Var)
			if !ok {
				panic(fmt.Sprintf("identifier %s passed as a variable could not be looked up as one", node))
			}

			// Skip if node is a global variable
			if annotation.VarIsGlobal(obj) {
				return false
			}

			// Skip if node is declared in this function literal
			if isDeclaredIn(obj, scope) {
				return false
			}

			// Skip if there exists a VarInfo in varsFromClosure that has similar underlying object
			// to avoid adding multiple VarInfo for the same object in varsFromClosure.
			if visited[obj] {
				return false
			}

			// Otherwise, the variable is a closure variable.
			closureVar := &VarInfo{Ident: node, Obj: obj}

			varsFromClosure = append(varsFromClosure, closureVar)
			visited[obj] = true

		}
		return true
	})

	closureMap[funcLit] = varsFromClosure
}

// isDeclaredIn returns true if the variable is declared in the given scope or in any scope nested
// in it, e.g., the scope of an `if` or `for` statement or of a case clause in a type switch. Note
// that looking the variable up in the given scope alone is not enough, since it only finds the
// variables declared directly in that scope.
func isDeclaredIn(obj *types.Var, scope *types.Scope) bool {
	for s := obj.Parent(); s != nil; s = s.Parent() {
		if s == scope {
			return true
		}
	}
	return false
}
