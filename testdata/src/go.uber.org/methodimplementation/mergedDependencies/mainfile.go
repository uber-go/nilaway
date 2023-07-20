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
This is a test for checking support for caching functionality. Affiliations witnessed through upstream packages (A and B) should
not be re-analyzed by downstream ones (mergedDependencies/mainFile.go)
TODO: the caching behavior can only be observed by inspecting the logs. Make it as part of a unit test in the future perhaps by adding mocking of the log

<nilaway no inference>
*/
package mergedDependencies

import (
	"go.uber.org/methodimplementation/mergedDependencies/packageA"
	"go.uber.org/methodimplementation/mergedDependencies/packageB"
)

func main() {
	var i1 packageA.I1 = &packageA.S1{}
	i1.Foo2(i1.Foo1())

	var i2 packageB.I2
	s2 := &packageB.S2{}
	i2 = s2
	i2.Bar()
}
