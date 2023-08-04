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
	"go/types"

	"go.uber.org/nilaway/annotation"
)

type fldAssertionNode struct {
	assertionNodeCommon

	// declaring identifier for this field
	decl *types.Var

	functionContext FunctionContext
}

func (f *fldAssertionNode) MinimalString() string {
	return fmt.Sprintf("fld<%s>", f.decl.Name())
}

// GetAncestorVarAssertionNode returns the varAssertionNode node that is ancestor of the fldAssertionNode i.e. it is the
// varAssertionNode that lies on the path from root node to fldAssertionNode. Thus, if the fldAssertionNode represents the
// expression `o.f.g.h` then we return the varAssertion node corresponding to `o`
// Returns nil otherwise if there is no ancestor varAssertion node
func (f *fldAssertionNode) GetAncestorVarAssertionNode() *varAssertionNode {
	var curNode AssertionNode = f
	for curNode != nil {
		curNode = curNode.Parent()

		if res, ok := curNode.(*varAssertionNode); ok {
			return res
		}
	}
	return nil
}

// DefaultTrigger for a field node is that field's annotation
func (f *fldAssertionNode) DefaultTrigger() annotation.ProducingAnnotationTrigger {
	if f.functionContext.isDepthOneFieldCheck() {
		varNode := f.GetAncestorVarAssertionNode()
		// If the field is not produced by a variable we default to the FieldAnnotationKey
		// Similarly, for a global variable we default to the FieldAnnotationKey
		if varNode != nil && !annotation.VarIsGlobal(varNode.decl) {
			return annotation.FldRead{
				TriggerIfNilable: annotation.TriggerIfNilable{
					Ann: annotation.EscapeFieldAnnotationKey{
						FieldDecl: f.decl,
					}}}
		}
	}
	return annotation.FldRead{
		TriggerIfNilable: annotation.TriggerIfNilable{
			Ann: annotation.FieldAnnotationKey{
				FieldDecl: f.decl,
			}}}
}
