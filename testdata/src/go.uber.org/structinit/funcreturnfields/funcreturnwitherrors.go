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
Package funcreturnfields Tests when nilability flows through the field of return of a function or a method
<nilaway struct enable>
*/
package funcreturnfields

// Testing interaction with error semantics: 1.
type myErr struct{}

func (myErr) Error() string { return "myErr message" }

func giveEmptyACompositeWithErr() (*A11, error) {
	return &A11{}, &myErr{}
}

func m09() *int {
	t, err := giveEmptyACompositeWithErr()
	if err != nil {
		return new(int)
	}
	// This should not give an error (giveEmptyACompositeWithErr never returns without error, so this is actually unreachable)
	return t.aptr.ptr
}

// Testing interaction with error semantics: 2.
func giveEmptyACompositeWithErr2() (*A11, error) {
	return &A11{}, nil
}

func m88() *int {
	t, err := giveEmptyACompositeWithErr2()
	if err != nil {
		return new(int)
	}
	// This should give an error
	return t.aptr.ptr //want "field `ptr` of `t.aptr` accessed"
}

// Testing interaction with error semantics: 3.
func dummy() bool { return true }
func giveEmptyACompositeWithErr3() (*A11, error) {
	if dummy() {
		return &A11{}, nil
	} else {
		return &A11{}, &myErr{}
	}
}

func m78() *int {
	t, err := giveEmptyACompositeWithErr3()
	if err != nil {
		return new(int)
	}
	// This should give an error
	return t.aptr.ptr //want "field `ptr` of `t.aptr` accessed"
}

// Testing interaction with error semantics: 4.
func giveEmptyACompositeWithErr4() (*A11, error) {
	return &A11{aptr: new(A11)}, nil
}

func m48() *int {
	t, err := giveEmptyACompositeWithErr4()
	if err != nil {
		return new(int)
	}
	// This should not give an error
	return t.aptr.ptr
}

// Testing interaction with error semantics: 5.
func giveEmptyACompositeWithErr5() (*A11, error) {
	if dummy() {
		return &A11{aptr: new(A11)}, nil
	} else {
		return &A11{}, &myErr{}
	}
}

func m58() *int {
	t, err := giveEmptyACompositeWithErr5()
	if err != nil {
		return new(int)
	}
	// This should not give an error
	return t.aptr.ptr
}

// Testing interaction with error semantics: 6.
func giveEmptyACompositeWithErr6() (*A11, error) {
	if dummy() {
		return &A11{}, nil
	} else {
		return &A11{aptr: new(A11)}, &myErr{}
	}
}

func m68() *int {
	t, err := giveEmptyACompositeWithErr6()
	if err != nil {
		return new(int)
	}
	// This should give an error
	return t.aptr.ptr //want "field `ptr` of `t.aptr` accessed"
}

// test case for named return
func m98() (a *A11, e error) {
	a = &A11{}
	return
}

func callM98() {
	t, err := m98()
	if err == nil {
		print(t.aptr.ptr) //want "field `aptr` returned by result 0 of function"
	}
}
