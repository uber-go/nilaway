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
This test aims to make sure that looping control flow is correctly captured by the fixed point
algorithm. It's TP and TN cases reflect loops that may or may not, respectively, flow a nil value
into the return of a non-nil returning function, which would trigger an error. More than 2
iterations are required to observe the correct behavior.

<nilaway no inference>
*/
package loopflow

func dummyBool() bool {
	return true
}

// this function should have a nil error - the rotation can cause j0 to flow to j3
func rotNilLoop(i int) struct{} {
	var j0 *struct{} = nil
	j1 := new(struct{})
	j2 := new(struct{})
	j3 := new(struct{})
	for dummyBool() {
		k := j3
		j3 = j2
		j2 = j1
		j1 = j0
		j0 = k
	}
	return *j3 //want "dereferenced"
}

// this function should not have a nil error- j0 does not rotate to j3
func rotSafeLoop(i int) struct{} {
	var j0 *struct{} = nil
	j1 := new(struct{})
	var j2 = new(struct{})
	j3 := new(struct{})
	for dummyBool() {
		k := j3
		j3 = j2
		j2 = k

		k = j1
		j1 = j0
		j0 = k
	}
	return *j3
}

// nilable(f)
type A struct {
	f *A
}

func getA() *A {
	return &A{}
}

func infiniteAssertion() {
	a := &A{}
	for dummyBool() {
		a.f = getA()
		a = a.f
	}
	for dummyBool() {
		a = a.f //want "field `f` of `a` accessed"
	}
}

// This is a known false negative. The analysis can be sound in this case if config.StableRoundLimit >= 9. However, that
// would result in degradation of NilAway performance. In practice, we haven't observed a case similar to this, and
// config.StableRoundLimit >= 2 is mostly sufficient for soundness.
// Similar large test that is a false negative can be constructed for any value of config.StableRoundLimit, is currently
// a known limitation of NilAway.
func longRotNilLoop(i int) struct{} {
	var j0 *struct{} = nil
	j1 := new(struct{})
	j2 := new(struct{})
	j3 := new(struct{})
	j4 := new(struct{})
	j5 := new(struct{})
	j6 := new(struct{})
	j7 := new(struct{})
	j8 := new(struct{})
	j9 := new(struct{})
	for dummyBool() {
		j9 = j8
		j8 = j7
		j7 = j6
		j6 = j5
		j5 = j4
		j4 = j3
		j3 = j2
		j2 = j1
		j1 = j0
	}
	// j9 is nilable and thus this is a false negative
	return *j9
}
