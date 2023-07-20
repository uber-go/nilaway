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

package nilcheck

var dummy bool

// nilable(result 0)
func retNil() *int {
	return nil
}

// ---- testReturn tests different cases in the return scenario ----

// nilable(v)
func testReturn(i int, v *int) bool {
	x := &i
	switch i {
	case 0:
		return v != nil && *v == 1
	case 1:
		return v != nil || *v == 1 //want "nilable value dereferenced"
	case 2:
		return v == nil && x != nil && *v == 1 //want "nilable value dereferenced"
	case 3:
		return (v == nil || x != nil) && *v == 1 //want "nilable value dereferenced"
	case 4:
		return (x != nil && *v == 1) && (v != nil && *x == 0) //want "nilable value dereferenced"
	case 5:
		return v != nil && x != nil && *v == 1 && *x == 0
	case 6:
		return v != nil || dummy && *v == 1 //want "nilable value dereferenced"
	case 7:
		return (nil != v || nil == v) && *v == 1 //want "nilable value dereferenced"
	case 8:
		return retNil() != nil && *retNil() == 1
	case 9:
		return *v == 1 && v != nil //want "nilable value dereferenced"
	case 10:
		return (v != nil) && *v == 1
	case 11:
		return (v != nil && x != nil) && *v == 1
	case 12:
		return (!(v == nil)) && *v == 1
	case 13:
		return (!(v != nil)) && *v == 1 //want "nilable value dereferenced"
	case 14:
		return (v == nil && dummy) || *v == 1 //want "nilable value dereferenced"
	case 15:
		return v == nil && dummy || *v == 1 //want "nilable value dereferenced"
	case 16:
		// below is a rather difficult case for NilAway. It requires full SAT solving capability that NilAway currently does not support. Hence, in this case it reports a False Positive.
		return (v != nil || dummy) && (!dummy || nil != v) && *v == 1 //want "nilable value dereferenced"
	case 17:
		return !(!(v == nil)) && *v == 1 //want "nilable value dereferenced"
	case 18:
		return !(!(v != nil)) && *v == 1
	case 19:
		return !(v != v) && *v == 1 //want "nilable value dereferenced"
	case 20:
		return !(v != v) || *v == 1 //want "nilable value dereferenced"
	}
	return true
}

// ---- testAssignment tests similar cases as return, but for the assignment scenario ----

// nilable(v)
func testAssignment(i int, v *int) bool {
	var x bool
	y := &i

	switch i {
	case 0:
		x = v != nil && *v == 1
	case 1:
		x = v != nil || *v == 1 //want "nilable value dereferenced"
	case 2:
		x = v == nil && y != nil && *v == 1 //want "nilable value dereferenced"
	case 3:
		x = (v == nil || y != nil) && *v == 1 //want "nilable value dereferenced"
	case 4:
		x = (y != nil && *v == 1) && (v != nil && *y == 0) //want "nilable value dereferenced"
	case 5:
		x = v != nil && y != nil && *v == 1 && *y == 0
	case 6:
		z := v != nil || dummy && *v == 1 //want "nilable value dereferenced"
		x = z
	case 7:
		x = (nil != v || nil == v) && *v == 1 //want "nilable value dereferenced"
	case 8:
		x = retNil() != nil && *retNil() == 1
	case 9:
		x = *v == 1 && v != nil //want "nilable value dereferenced"
	}
	return x
}

// ---- testParam tests similar cases as return, but for the parameter passing scenario ----

func takesBool(p bool) {}

// nilable(v)
func testParam(i int, v *int) {
	x := &i
	switch i {
	case 0:
		takesBool(v != nil && *v == 1)
	case 1:
		takesBool(v != nil || *v == 1) //want "nilable value dereferenced"
	case 2:
		takesBool(v == nil && x != nil && *v == 1) //want "nilable value dereferenced"
	case 3:
		takesBool((v == nil || x != nil) && *v == 1) //want "nilable value dereferenced"
	case 4:
		takesBool((x != nil && *v == 1) && (v != nil && *x == 0)) //want "nilable value dereferenced"
	case 5:
		takesBool(v != nil && x != nil && *v == 1 && *x == 0)
	case 6:
		takesBool(v != nil || dummy && *v == 1) //want "nilable value dereferenced"
	case 7:
		takesBool((nil != v || nil == v) && *v == 1) //want "nilable value dereferenced"
	case 8:
		takesBool(retNil() != nil && *retNil() == 1)
	case 9:
		takesBool(*v == 1 && v != nil) //want "nilable value dereferenced"
	}
}

// ---- test struct init ----
type A struct {
	f bool
}

// nilable(v)
func testStruct(i int, v *int) bool {
	switch i {
	case 0:
		a := &A{}
		a.f = (v != nil && *v == 1)
		return a.f
	case 1:
		a := &A{}
		a.f = (v != nil || *v == 1) //want "nilable value dereferenced"
		return a.f
	case 2:
		a := &A{v != nil && *v == 1}
		return a.f
	case 3:
		a := &A{v != nil || *v == 1} //want "nilable value dereferenced"
		return a.f
	}
	return false
}

// ---- test global ----
// nilable(globalV1)
var globalV1 *int = nil
var globalV2 = (globalV1 != nil && *globalV1 == 1)
var globalV3 = (globalV1 != nil || *globalV1 == 1) // Unsafe! Currently a false negative: add support in a follow-up diff. Issue tracked in

// ---- test struct with a chain of field accesses ----
// nilable(f)
type X struct {
	f *G
}

// nilable(g)
type G struct {
	g *H
}

type H struct {
	h int
}

// nilable(x)
func testChainedAccesses(x *X) bool {
	// Below gives False Positives for the field accesses of `f` and `g`. Fix this in a follow-up diff. Issue tracked in
	return x != nil && x.f != nil && x.f.g != nil && x.f.g.h == 4 //want "nilable value" "nilable value" "nilable value"
}
