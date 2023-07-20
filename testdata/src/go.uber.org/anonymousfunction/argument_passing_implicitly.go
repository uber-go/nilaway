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

// This package aims to test nilability behavior for simple cases in anonymous functions.
// <nilaway anonymous function enable>
package anonymousfunction

func testNilFlowFromClosure() {

	var t *int
	func() { // want "Annotation on Param 0: 't'"
		print(*t)
	}()

	i := 1
	t = &i
	func() {
		print(*t)
	}()

	func() {
		print(*t)
		print(*t)
	}()

	t = nil

	// (Note: below 'want' captures the grouped error reported for both the print calls)
	func() { //want "Annotation on Param 0: 't'"
		print(*t)
		print(*t)
	}()

	t = &i

	func() {
		print(*t)
		t = nil
		print(*t) // want "Value literal nil at"
	}()

	// TODO we will report an error here after updating the return type of function literals to include variables from closure
	print(*t)

	// test nested anonymous functions

	var t1 *int
	func() {
		var t2 *int
		func() { // want "Annotation on Param 0: 't1'" "Annotation on Param 1: 't2'"
			print(*t1)
			print(*t2)
		}()
	}()

	c := 1
	t3 := &c // t5 is nonnil now
	func() {
		print(*t3)
		var t4 *int
		// the following error is coming from t4 but not from t3
		func() { // want "Annotation on Param 1: 't4'"
			print(*t3) // this should be ok
			print(*t4) // this is not
			if t4 != nil {
				print(*t4) // this is ok
			}
		}()
	}()

	var t5 *int
	func() { // want "Annotation on Param 2: 't5'" "Annotation on Param 0: 't1'"
		print(*t1)
		print(*t3)
		print(*t5)
	}()

}
