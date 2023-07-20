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

/*
This is a test for checking that NilAway only analyzes the affiliations within scope. For example, ast is an external
library, affiliations processing of which should be ignored
*/

package methodimplementation

import (
	"go/ast"
	"go/token"
)

type myNode struct{}

// nilable(result 0)
func (myNode) Pos() token.Pos {
	return 1
}

func (myNode) End() token.Pos {
	return 2
}

func m11() {
	var node ast.Node = &ast.FuncDecl{}
	node.Pos()

	var mynode ast.Node = &myNode{}
	mynode.Pos()
}
