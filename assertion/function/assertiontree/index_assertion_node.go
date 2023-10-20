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
	"go/types"

	"go.uber.org/nilaway/annotation"
	"golang.org/x/tools/go/analysis"
)

type indexAssertionNode struct {
	assertionNodeCommon
	index ast.Expr

	// we need to remember the type of the values of this index because there is no other way
	// to look it up - unlike fields and functions there is no sufficient identifier to store
	valType types.Type

	// here we store the type of the reciever to this indexAssertionNode -
	// specifically to determine if it is a map
	recvType types.Type
}

func (i *indexAssertionNode) MinimalString() string {
	return "index"
}

// DefaultTrigger for an index node is the deep nilability annotation of its parent type
func (i *indexAssertionNode) DefaultTrigger() annotation.ProducingAnnotationTrigger {
	return deepNilabilityTriggerOf(i.Parent())
}

// BuildExpr for an index node adds that index to `expr`
func (i *indexAssertionNode) BuildExpr(_ *analysis.Pass, expr ast.Expr) ast.Expr {
	return &ast.IndexExpr{
		X:      expr,
		Lbrack: 0,
		Index:  i.index,
		Rbrack: 0,
	}
}
