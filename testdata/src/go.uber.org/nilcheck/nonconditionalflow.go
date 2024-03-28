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
		return v != nil || *v == 1 //want "dereferenced"
	case 2:
		return v == nil && x != nil && *v == 1 //want "dereferenced"
	case 3:
		//  It is a false negative. Currently NilAway cannot reason about such complex expressions in a nonconditional
		//  setting. Such code patterns are perhaps not that common in practice, and hence we are not prioritizing
		//  this at the moment.
		// TODO: fix this in a follow-up PR.
		return (v == nil || x != nil) && *v == 1
	case 4:
		return (x != nil && *v == 1) && (v != nil && *x == 0) //want "dereferenced"
	case 5:
		return v != nil && x != nil && *v == 1 && *x == 0
	case 6:
		return v != nil || dummy && *v == 1 //want "dereferenced"
	case 7:
		//  It is a false negative. Currently NilAway cannot reason about complex expressions like this since it does
		//  not have full SAT solving capability. Such code patterns are perhaps not that common in practice, and
		//  hence we are not prioritizing this at the moment.
		// TODO: fix this in a follow-up diff.
		return (nil != v || nil == v) && *v == 1
	case 8:
		return retNil() != nil && *retNil() == 1
	case 9:
		return *v == 1 && v != nil //want "dereferenced"
	case 10:
		return (v != nil) && *v == 1
	case 11:
		return (v != nil && x != nil) && *v == 1
	case 12:
		return (!(v == nil)) && *v == 1
	case 13:
		return (!(v != nil)) && *v == 1 //want "dereferenced"
	case 14:
		return (v == nil && dummy) || *v == 1 //want "dereferenced"
	case 15:
		return v == nil && dummy || *v == 1 //want "dereferenced"
	case 16:
		// below is a rather difficult case for NilAway. It requires full SAT solving capability that NilAway currently does not support. Hence, in this case it reports a False Positive.
		return (v != nil || dummy) && (!dummy || nil != v) && *v == 1 //want "dereferenced"
	case 17:
		return !(!(v == nil)) && *v == 1 //want "dereferenced"
	case 18:
		return !(!(v != nil)) && *v == 1
	case 19:
		return !(v != v) && *v == 1 //want "dereferenced"
	case 20:
		return !(v != v) || *v == 1 //want "dereferenced"
	case 21:
		// This is currently a false negative, since NilAway loses track of the effect of the negation on compound
		// logical expressions in recursive calls. Note that NilAway can handle negations well, if the enclosed
		// expression is atomic. For example, `!(v != nil) && *v == 1` is handled correctly.
		// Such code patterns are perhaps not that common in practice, and hence we are not prioritizing this at the moment.
		// TODO: fix this in a follow-up PR.
		return !(v != nil && x == nil) && *v == 1
	case 22:
		return v == nil || *v == 1
	case 23:
		return v != nil || *v == 1 //want "dereferenced"
	case 24:
		return v == nil || x == nil || *v == 1
	case 25:
		return (x == nil || *v == 1) || (v == nil || *x == 0) //want "dereferenced"
	case 26:
		return v == nil || x == nil || *v == 1 || *x == 0
	case 27:
		return v != nil || x == nil || *v == 1 || *x == 0 //want "dereferenced"
	case 28:
		return (!(v != nil)) || *v == 1
	case 29:
		return retNil() == nil || *retNil() == 1
	case 30:
		return v == nil || dummy || *v == 1
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
		x = v != nil || *v == 1 //want "dereferenced"
	case 2:
		x = v == nil && y != nil && *v == 1 //want "dereferenced"
	case 3:
		//  It is a false negative. Currently NilAway cannot reason about such complex expressions in a nonconditional
		//  setting. Such code patterns are perhaps not that common in practice, and hence we are not prioritizing
		//  this at the moment.
		// TODO: fix this in a follow-up PR.
		x = (v == nil || y != nil) && *v == 1
	case 4:
		x = (y != nil && *v == 1) && (v != nil && *y == 0) //want "dereferenced"
	case 5:
		x = v != nil && y != nil && *v == 1 && *y == 0
	case 6:
		z := v != nil || dummy && *v == 1 //want "dereferenced"
		x = z
	case 7:
		//  It is a false negative. Currently NilAway cannot reason about such complex expressions in a nonconditional
		//  setting. Such code patterns are perhaps not that common in practice, and hence we are not prioritizing
		//  this at the moment.
		// TODO: fix this in a follow-up PR.
		x = (nil != v || nil == v) && *v == 1
	case 8:
		x = retNil() != nil && *retNil() == 1
	case 9:
		x = *v == 1 && v != nil //want "dereferenced"
	case 10:
		x = v == nil || *v == 1
	case 11:
		x = v != nil || *v == 1 //want "dereferenced"
	case 12:
		x = v == nil || y != nil || *v == 1
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
		takesBool(v != nil || *v == 1) //want "dereferenced"
	case 2:
		takesBool(v == nil && x != nil && *v == 1) //want "dereferenced"
	case 3:
		//  It is a false negative. Currently NilAway cannot reason about such complex expressions in a nonconditional
		//  setting. Such code patterns are perhaps not that common in practice, and hence we are not prioritizing
		//  this at the moment.
		// TODO: fix this in a follow-up PR.
		takesBool((v == nil || x != nil) && *v == 1)
	case 4:
		takesBool((x != nil && *v == 1) && (v != nil && *x == 0)) //want "dereferenced"
	case 5:
		takesBool(v != nil && x != nil && *v == 1 && *x == 0)
	case 6:
		takesBool(v != nil || dummy && *v == 1) //want "dereferenced"
	case 7:
		//  It is a false negative. Currently NilAway cannot reason about such complex expressions in a nonconditional
		//  setting. Such code patterns are perhaps not that common in practice, and hence we are not prioritizing
		//  this at the moment.
		// TODO: fix this in a follow-up PR.
		takesBool((nil != v || nil == v) && *v == 1)
	case 8:
		takesBool(retNil() != nil && *retNil() == 1)
	case 9:
		takesBool(*v == 1 && v != nil) //want "dereferenced"
	case 10:
		takesBool(v == nil || *v == 1)
	case 11:
		takesBool(v != nil || *v == 1) //want "dereferenced"
	case 12:
		takesBool(v == nil || x != nil || *v == 1)
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
		a.f = (v != nil || *v == 1) //want "dereferenced"
		return a.f
	case 2:
		a := &A{v != nil && *v == 1}
		return a.f
	case 3:
		a := &A{v != nil || *v == 1} //want "dereferenced"
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
func testChainedAccesses(x *X, i int) bool {
	switch i {
	case 0:
		return x != nil && x.f != nil && x.f.g != nil
	case 1:
		return x != nil && x.f != nil && x.f.g != nil && x.f.g.h == 4
	case 2:
		return x != nil && x.f != nil && x.f.g.h == 4 //want "field `g` accessed field `h`"
	case 3:
		// safe, but condition interspersed with different irrelevant checks
		return x != nil && retNil() == nil && x.f != nil && *x.f == G{} && x.f.g != nil && dummy && x.f.g.h == 4
	case 4:
		return x != nil && x.f != nil && x.f.g.h == 4 && x.f.g != nil //want "field `g` accessed field `h`"
	case 5:
		return x == nil || x.f == nil || x.f.g == nil || x.f.g.h == 1
	case 6:
		return x == nil || x.f == nil || x.f.g.h == 4 //want "field `g` accessed field `h`"
	case 7:
		// safe, but condition interspersed with different irrelevant checks
		return x == nil || retNil() == nil || x.f == nil || *x.f == G{} || x.f.g == nil || dummy || x.f.g.h == 4
	case 8:
		return x == nil || x.f == nil || x.f.g.h == 4 || x.f.g == nil //want "field `g` accessed field `h`"
	}
	return false
}

// ---- test len checks ----
func testLenChecks(s []int, i int) bool {
	var t []int

	switch i {
	case 0:
		return len(s) > 0 && s[0] == 1
	case 1:
		return len(s) > 0 && s[i] == 1
	case 2:
		return len(s) >= 0 && s[0] == 1 //want "sliced into"
	case 3:
		return 0 < len(s) && s[0] == 1
	case 4:
		return 0 > len(s) && s[0] == 1 //want "sliced into"
	case 5:
		return len(s) == 0 && s[0] == 1 //want "sliced into"
	case 6:
		return len(s) == 1 && s[0] == 1
	case 7:
		return len(t) > 0 && len(s) == len(t) && s[0] == 1
	case 8:
		return !(len(s) > 0) && s[0] == 1 //want "sliced into"
	case 9:
		return !(!(len(s) > 0)) && s[0] == 1
	case 10:
		return (!(len(s) > 0)) && s[0] == 1 //want "sliced into"
	case 11:
		return !(!(!(!(!(len(s) > 0))))) && s[0] == 1 //want "sliced into"
	case 12:
		return len(s) > 0 || s[0] == 1 //want "sliced into"
	case 13:
		return len(s) < 0 && len(t) > 0 && s[0] == 1 //want "sliced into"
	case 14:
		//  It is a false negative. Currently NilAway cannot reason about such complex expressions in a nonconditional
		//  setting. Such code patterns are perhaps not that common in practice, and hence we are not prioritizing
		//  this at the moment.
		// TODO: fix this in a follow-up PR.
		return (len(s) == 0 || len(t) > 0) && s[0] == 1
	case 15:
		return (len(t) > 0 && s[0] == 1) && (len(s) > 0 && t[0] == 0) //want "sliced into"
	case 16:
		return len(s) > 0 && len(t) > 0 && s[0] == 1 && t[0] == 0
	case 17:
		return len(s) > 0 || dummy && s[0] == 1 //want "sliced into"
	case 18:
		//  It is a false negative. Currently NilAway cannot reason about such complex expressions in a nonconditional
		//  setting. Such code patterns are perhaps not that common in practice, and hence we are not prioritizing
		//  this at the moment.
		// TODO: fix this in a follow-up PR.
		return (0 == len(s) || 0 > len(s)) && s[0] == 1
	case 19:
		return len(s) <= 0 || s[0] == 1
	case 20:
		return len(s) <= 0 || s[i] == 1
	case 21:
		return len(s) < 0 || s[0] == 1 //want "sliced into"
	case 22:
		return 0 >= len(s) || s[0] == 1
	case 23:
		return 0 < len(s) || s[0] == 1 //want "sliced into"
	case 24:
		return len(s) != 0 || s[0] == 1 //want "sliced into"
	case 25:
		return len(s) != 1 || s[0] == 1
	case 26:
		return len(t) < 0 || len(s) != len(t) || s[0] == 1
	case 27:
		return !(len(s) < 0) || s[0] == 1 //want "sliced into"
	}
	return false
}
