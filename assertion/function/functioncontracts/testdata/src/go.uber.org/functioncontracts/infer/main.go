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

// <nilaway contract enable>
package infer

import (
	"math/rand"
)

func onlyLocalVar(x *int) *int {
	// SSA:
	// 0: {}
	//   x != nil:*int
	//   if t0 goto 1 else 2
	// 1: {x: nonnil}
	// 	 jump 2
	// 2: {x: nonnil, y: nonnil} OR {x: nil, y: nil}
	// 	 phi [0: nil:*int, 1: x] #y
	//   t1 != nil:*int
	//   if t2 goto 3 else 4
	// 3: {x: nonnil, y: nonnil}
	//   return t1
	// 4: {x: nil, y: nil}
	//   return x
	var y *int
	if x != nil {
		y = x
	}
	if y != nil {
		return y
	}
	return x
}

func unknownCondition(x *int) *int {
	// SSA:
	// 0: {}
	//   x != nil:*int
	//   if t0 goto 1 else 2
	// 1: {x: nonnil}
	// 	 jump 2
	// 2: {x: nonnil, y: nonnil} OR {x: nil, y: nil}
	// 	 phi [0: nil:*int, 1: x] #y
	//   phi [0: 0:int, 1: 1:int] #z
	//   t2 == 0:int
	//   if t3 goto 3 else 4
	// 3: {x: nonnil, y: nonnil} OR {x: nil, y: nil}
	//   jump 4
	// 4: {x: nonnil, y: nonnil, x(t4): nonnil} OR {x: nil, y: nil, x(t4): nil}
	//   phi [2: x, 3: t1] #x
	//   return t4
	var y *int = nil
	var z int = 0
	if x != nil {
		y = x
		z = 1
	}
	if z == 0 { // represents some unknown condition
		x = y
	}
	return x
}

func noLocalVar(x *int) *int {
	if x != nil {
		return new(int)
	}
	// Return nonnil or nil randomly if x is nil
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		return nil
	}
}

// contract(nonnil -> nonnil) does not hold.
func nonnilToNil(x *int) *int {
	if x == nil {
		return new(int)
	}
	return nil
}

// contract(nonnil -> nonnil) does not hold.
func nonnilToUnknown(x *int) *int {
	if x == nil {
		return nil
	}
	// Return nonnil or nil randomly if x is nonnil
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		return nil
	}
}

// contract(nonnil -> nonnil) does not hold.
func unknownToNil(x *int) *int {
	return nil
}

type S struct {
	f *int
}

// The function has valid contract(nonnil -> nonnil) but we are not able to infer it now. SSA
// create a new value for each occurence of s.f, we would not learn nilness of it from the table.
// To infer contract successfully for this function, we need to look deeply at pointer
// indirections.
func field(s *S) *int {
	// SSA:
	// Block 0:
	//   s == nil:*S // t0
	//   if t0 goto 1 else 2
	// Block 1:
	//   return nil:*int
	// Block 2:
	//   &s.f [#0] // t1
	//   *t1 // t2
	//   t2 == nil:*int
	//   if t3 goto 3 else 4
	// Block 3:
	//   new int (new) // t4
	//   &s.f [#0] // t5
	//   *t5 = t4
	//   jump 4
	// Block 4:
	//   &s.f [#0] // t6
	//   *t6
	//   return t7
	if s == nil {
		return nil
	}
	if s.f == nil {
		s.f = new(int)
	}
	return s.f
}

// exceeds maximum number of paths so we do not infer anything
func manyBranchesLocalVar(x *int) []*int {
	v1 := nilOrNonnilIntPointer()
	v2 := nilOrNonnilIntPointer()
	v3 := nilOrNonnilIntPointer()
	v4 := nilOrNonnilIntPointer()
	v5 := nilOrNonnilIntPointer()
	v6 := nilOrNonnilIntPointer()
	v7 := nilOrNonnilIntPointer()
	v8 := nilOrNonnilIntPointer()
	v9 := nilOrNonnilIntPointer()
	v10 := nilOrNonnilIntPointer()
	v11 := nilOrNonnilIntPointer()
	v12 := nilOrNonnilIntPointer()
	v13 := nilOrNonnilIntPointer()
	v14 := nilOrNonnilIntPointer()
	v15 := nilOrNonnilIntPointer()
	var vs []*int
	if x == nil {
		return vs
	}
	if v1 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v2 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v3 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v4 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v5 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v6 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v7 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v8 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v9 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v10 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v11 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v12 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v13 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v14 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	if v15 != nil {
		vs = append(vs, nilOrNonnilIntPointer())
	}
	return vs
}

func nilOrNonnilIntPointer() *int {
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		return nil
	}
}

// contract(nonnil -> nonnil) does not hold since b is a bool which cannot have nil as a valid
// value.
func nilIncompatibleType(b bool) *int {
	if b {
		return new(int)
	} else {
		return new(int)
	}
}

// contract(nonnil -> nonnil) holds but we do not infer.
func alwaysReturnNonnil(x *int) *int {
	return new(int)
}

// contract(nonnil -> nonnil) does not hold.
func learnOuterFromUnderlyingMakeInterface(i any) any {
	if i == nil {
		return i
	}
	var s []int // *ssa.MakeInterface: make any <- []int (nil:[]int)
	return s    // We should learn s is nil
}

type I interface {
	m()
}

type SI struct{}

func (s *SI) m() {}

func nilOrNonnilSI() *SI {
	if rand.Float64() > 0.5 {
		return new(SI)
	} else {
		return nil
	}
}

// contract(nonnil -> nonnil) holds.
func learnUnderlyingFromOuterMakeInterface(in I) I {
	if in == nil {
		return in
	}
	si := nilOrNonnilSI()
	i := I(si) // *ssa.MakeInterface: make I <- *SI (si)
	if i == nil {
		// We should learn si is nil from i is nil
		return new(SI)
	}
	// We should learn si is nonnil from i is nonnil
	return si
}

// we are not able to infer contract(nonnil -> nonnil) here because for now nil is defined as
// strict nil, which does not include the empty slice.
func nonEmptySliceToNonnil(s []int) []int {
	// SSA:
	// Block 0:
	//   t0: *ssa.Call: len(s)
	//   *ssa.Jump: jump 1
	// Block 1:
	//   t1: *ssa.Phi: phi [0: nil:[]int, 2: t10] #ns
	//   t2: *ssa.Phi: phi [0: -1:int, 2: t3]
	//   t3: *ssa.BinOp: t2 + 1:int
	//   t4: *ssa.BinOp: t3 < t0
	//       *ssa.If: if t4 goto 2 else 3
	// Block 2:
	//   t5: *ssa.IndexAddr: &s[t3]
	//   t6: *ssa.UnOp: *t5
	//   t7: *ssa.Alloc: new [1]int (varargs)
	//   t8: *ssa.IndexAddr: &t7[0:int]
	//   	 *ssa.Store: *t8 = t6
	//   t9: *ssa.Slice: slice t7[:]
	//   t10: *ssa.Call: append(t1, t9...)
	//       *ssa.Jump: jump 1
	// Block 3:
	//   *ssa.Return: return t1
	var ns []int
	for _, e := range s {
		ns = append(ns, e)
	}
	return ns
}

type STR struct {
	f *int
}

func twoCondsMerge(x *STR) *STR {
	if x == nil || x.f == nil {
		return x
	}
	return x
}

func unknownToUnknownButSameValue(x *int) *int {
	return x
}
