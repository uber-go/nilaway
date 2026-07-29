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
	"go.uber.org/nilaway/util/typeshelper"
)

type funcAssertionNode struct {
	assertionNodeCommon

	// declaring identifier for this function
	decl *types.Func
	args []ast.Expr
	// call identifies the source call. With -experimental-struct-init-v2 enabled, shallowEqNodes
	// uses its position to distinguish nodes only when the result has a unique whole-result
	// parameter source.
	call *ast.CallExpr
}

func (f *funcAssertionNode) MinimalString() string {
	return fmt.Sprintf("func<%s>", f.decl.Name())
}

// DefaultTrigger for a function node is that function's return annotation
func (f *funcAssertionNode) DefaultTrigger() annotation.ProducingAnnotationTrigger {
	if typeshelper.FuncNumResults(f.decl) != 1 {
		panic("only functions with singular result should be entered into the assertion tree")
	}

	// Parameter-sourced results use call-scoped sites instead of the shared return.
	if root := f.Root(); root != nil && root.functionContext.functionConfig.EnableStructInitV2 &&
		root.resultValueHasParamSource(f.decl, 0) {
		if f.call != nil {
			if siteProducer, ok := root.shallowCallResultSiteProducer(f.call, f.decl, 0); ok {
				return siteProducer
			}
		}
		return &annotation.ProduceTriggerNever{}
	}

	if f.decl.Type().(*types.Signature).Recv() != nil {
		return &annotation.MethodReturn{
			TriggerIfNilable: &annotation.TriggerIfNilable{
				Ann: annotation.RetKeyFromRetNum(f.decl, 0)}}
	}
	return &annotation.FuncReturn{
		TriggerIfNilable: &annotation.TriggerIfNilable{
			Ann: annotation.RetKeyFromRetNum(f.decl, 0)}}
}

// BuildExpr for a function node adds that function to `expr` as a method call
func (f *funcAssertionNode) BuildExpr(expr ast.Expr) ast.Expr {
	if f.Root() == nil {
		panic("f.BuildExpr should only be called on nodes present in a valid assertion tree")
	}
	genFunc := func() ast.Expr {
		if expr == nil {
			return f.Root().GetDeclaringIdent(f.decl)
		}
		return &ast.SelectorExpr{
			X:   expr,
			Sel: f.Root().GetDeclaringIdent(f.decl),
		}
	}
	return &ast.CallExpr{
		Fun:      genFunc(),
		Lparen:   0,
		Args:     f.args,
		Ellipsis: 0,
		Rparen:   0,
	}
}
