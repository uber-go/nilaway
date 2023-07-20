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

// <nilaway no inference>
package packageC

import (
	"go.uber.org/methodimplementation/multipackage/packageA"
	"go.uber.org/methodimplementation/multipackage/packageB"
)

func M9() packageA.I9 {
	var v packageA.I9 = &packageB.A9{"abc"}
	return v
}
