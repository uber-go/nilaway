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
This test functions as a sanity check for multi-file handling, just making sure that in the
case of linked files no errors are thrown

<nilaway no inference>
*/
package multifilepackage

import (
	"go.uber.org/multifilepackage/firstpackage"
	"go.uber.org/multifilepackage/secondpackage"
)

func main() {
	a := &firstpackage.A{}
	b := &firstpackage.B{}
	c := secondpackage.C{true}
	c.Branch(a, b).CheckReflect()
}

func safeBoxManipulations() {
	c := secondpackage.CBox{}
	c.Box(nil)
	c.Ptr = nil
}

func unsafeBoxManipulations() *secondpackage.C {
	c := secondpackage.CBox{}
	if true {
		return c.Unbox() //want "nilable value returned"
	} else {
		return c.Ptr //want "nilable value returned"
	}
}
