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
	"fmt"
	"go/ast"
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

type varAssertionNode struct {
	assertionNodeCommon

	// declaring identifier for this variable
	decl *types.Var
}

func (v *varAssertionNode) MinimalString() string {
	return fmt.Sprintf("var<%s>", v.decl.Name())
}

// DefaultTrigger for a varAssertionNode is special cased to read annotations for variables and
// parameters, but otherwise is always NoVarAssign{}
func (v *varAssertionNode) DefaultTrigger() annotation.ProducingAnnotationTrigger {
	if v.Root() == nil {
		panic("v.DefaultTrigger should only be called on nodes present in a valid assertion tree")
	}
	fdecl := v.Root().FuncObj()
	if annotation.VarIsParam(fdecl, v.decl) {
		return annotation.ParamAsProducer(fdecl, v.decl)
	}
	if annotation.VarIsRecv(fdecl, v.decl) {
		return annotation.MethodRecv{
			TriggerIfNilable: annotation.TriggerIfNilable{
				Ann: annotation.RecvAnnotationKey{FuncDecl: fdecl}},
			VarDecl: v.decl,
		}
	}
	if annotation.VarIsGlobal(v.decl) {
		return annotation.GlobalVarRead{
			TriggerIfNilable: annotation.TriggerIfNilable{
				Ann: annotation.GlobalVarAnnotationKey{
					VarDecl: v.decl}}}
	}

	// By process of elimination we know that here `v` is a local variable

	// if `v` is a struct (e.g., var s S), not a struct pointer, then analyze it for its fields. Note that here we don't
	// want to analyze fields of an unassigned struct pointer, since at this point the pointer itself is nil.
	// TODO: below logic won't be required once we standardize the expression `var s S` by replacing it with `S{}` in the
	//  preprocessing phase
	if !util.TypeIsDeeplyPtr(v.decl.Type()) {
		if structType := util.TypeAsDeeplyStruct(v.decl.Type()); structType != nil {
			if v.Root().functionContext.isDepthOneFieldCheck() {
				v.Root().addProductionForVarFieldNode(v, v.BuildExpr(v.Root().Pass(), nil))
			}
			return annotation.ProduceTriggerNever{} // indicating that the struct object itself is not nil
		}
	}

	return annotation.NoVarAssign{VarObj: v.decl}
}

// BuildExpr for a varAssertionNode returns the underlying variable's AST node
func (v *varAssertionNode) BuildExpr(_ *analysis.Pass, _ ast.Expr) ast.Expr {
	if v.Root() == nil {
		panic("v.BuildExpr should only be called on nodes present in a valid assertion tree")
	}
	return v.Root().GetDeclaringIdent(v.decl)
}
