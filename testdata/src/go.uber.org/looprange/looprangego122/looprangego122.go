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
package looprangego122

// Test for checking range over basic types, such as integers and strings.
// TODO: move this testcase to `looprange.go` once NilAway starts to support Go 1.22.
func testRangeOverBasicTypes(j int) {
	switch j {
	case 0:
		for i := range 10 {
			print(i)
		}
	case 1:
		n := 10
		for i := range n {
			print(i)
		}
	case 2:
		for i := range "hello" {
			print(i)
		}
	case 3:
		s := "hello"
		for i := range s {
			print(i)
		}
	}
}
