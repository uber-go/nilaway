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
This is a test for checking support for downstream/upstream dependencies. For example, interface and struct defined in
different packages and their instantiation(s) witnessed in yet another package(s). Also, this test checks the use of
cache passed across packages to avoid re-analysis of the affiliations.

<nilaway no inference>
*/
package multipackage

import (
	"go/ast"

	"go.uber.org/methodimplementation/multipackage/packageA"
	"go.uber.org/methodimplementation/multipackage/packageB"
	"go.uber.org/methodimplementation/multipackage/packageC"
)

func m10() packageA.I9 {
	param2(&packageB.A9{})

	packageC.M9()

	var v packageA.I9
	v = &packageB.A9{"xyz"}
	return v
}

func param2(i packageA.I9) {
	// do something ...
}

func m11() {
	var node ast.Node = &ast.FuncDecl{}
	node.Pos()
}
