//  Copyright (c) 2025 Uber Technologies, Inc.
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

package nolint

func localNoLintLint() {
	var p *int
	print(*p) //nolint:nilaway
	print(*p) //nolint:all
	print(*p) // nolint     :   nilaway // Explanation
	print(*p) //nolint
	print(*p) ////nolint:nilaway
}

//nolint:nilaway
func localNoLintFunc() {
	var p *int
	print(*p)
	print(*p)
	print(*p)
}
