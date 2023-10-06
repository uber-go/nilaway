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

// generics package tests NilAway's ability to handle generics introduced in Go 1.18.
// Currently, we do not have support for generics yet, so this package simply tests that
// NilAway should not panic when seeing ASTs related to generics.
// TODO: Add support for generics.
//
// <nilaway no inference>
package generics

type A struct{}

func (a *A) foo() bool { return true }

type B struct{}

func (b *B) foo() bool { return true }

// AB is a type constraint that states the instantiated type must be either be *A or *B.
// Interestingly, although both A and B implements the foo() method, you must explicitly state
// that the type constraint AB has a method foo() if you would like to use it on AB. This
// limitation is currently documented and an issue is tracked here:
// https://github.com/golang/go/issues/51183.
// Note that if, for example, A does not implement foo(), it will result in a compile error _only_
// if you pass an argument with type A to the genericFunc, but passing a B to it will be OK.
type AB interface {
	*A | *B
	foo() bool
}

// nonnil(x)
func genericFunc[T AB](x T) bool {
	return x.foo()
}

// SumIntsOrFloats sums the values of map m. It supports both int64 and float64 as types for map
// values. This is taken from https://go.dev/doc/tutorial/generics.
// `comparable` here is a new keyword introduced along with generics, it is a type
// constraints that is implemented by all builtin types that can be compared (== and !=), such
// as structs, pointers, channels, int64, float64, and more.
// nonnil(m)
func SumIntsOrFloats[K comparable, V int64 | float64](m map[K]V) V {
	var s V
	for _, v := range m {
		s += v
	}
	return s
}

func useGenericFunc() int64 {
	m := make(map[string]int64)
	// We can omit the type instantiation when calling generic functions, the type inference
	// algorithms will handle it fine.
	return SumIntsOrFloats(m)
}

// GenericStruct is a generic struct that takes multiple type arguments with different constraints.
type GenericStruct[T1 AB, T2 int64 | float64, T3 any] struct {
	x T1
	y T2
	z T3
}

// nonnil(result 0)
func useGenericStruct() []*GenericStruct[*A, int64, *int] {
	i := 1
	// We cannot omit the type arguments when instantiating generic types, it is currently
	// (Go 1.20) not supported.
	return []*GenericStruct[*A, int64, *int]{
		{x: &A{}, y: 42, z: &i},
	}
}

// You can also nest type parameters.
type NestedGenericStruct[T1 AB, T2 GenericStruct[T1, float64, *int]] struct{}

// You can also declare a new type using instantiated generic type underneath.
type fooT1 GenericStruct[*A, int64, *int]

// Type alias works as well.
type fooT2 = GenericStruct[*A, int64, *int]

// You can also declare a new type by instantiating parts of the type arguments.
type fooT3[T1 AB] GenericStruct[T1, int64, *int]

func defineInFunc(p1 any, p2 fooT1) {
	type fooT4[T1 AB] GenericStruct[T1, int64, *int]

	if _, ok := p1.(GenericStruct[*A, int64, *int]); ok {
	}

	switch p1.(type) {
	case GenericStruct[*A, int64, *int]:
	}

	// You can use an instantiated generic type for type casts as well, provided that the
	// underlying type is the same.
	_ = GenericStruct[*A, int64, *int](p2)
}

// Test for a case where we have a generic slice.
func GenericSlice[S ~[]*E, E any](s S) int {
	for _, element := range s {
		// TODO: we do not really handle generics for now, so the elements inside any
		//  generic slices are considered nonnil. We should handle that properly.
		print(*element) // False negative.
	}
	return -1
}

func callGenericSlice() {
	a := []*int{nil, nil, nil}
	GenericSlice(a)
}
