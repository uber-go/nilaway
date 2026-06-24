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

// Tests the (value, error) constructor pattern: a struct returned alongside a non-nil (or unknown)
// error is never observed by a caller that checks `err != nil`, so its possibly-nil fields must not
// poison the success-path summary; but a caller that ignores the error is still flagged.

package funcreturnfields

import (
	"errors"
	"fmt"
)

// 1. Always returns a non-nil error: the nil-fielded value is never observed on the success path.
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
	// No error: giveEmptyACompositeWithErr never returns a nil error, so this is unreachable.
	return t.aptr.ptr
}

// 2. Returns the nil-fielded value with a NIL error: the field nilability reaches the success path.
func giveEmptyACompositeWithErr2() (*A11, error) {
	return &A11{}, nil
}

func m88() *int {
	t, err := giveEmptyACompositeWithErr2()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr //want "accessed field `ptr`"
}

// 3. One success path returns the nil-fielded value: it reaches the caller's success path.
func dummy() bool { return true }
func giveEmptyACompositeWithErr3() (*A11, error) {
	if dummy() {
		return &A11{}, nil
	}
	return &A11{}, &myErr{}
}

func m78() *int {
	t, err := giveEmptyACompositeWithErr3()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr //want "accessed field `ptr`"
}

// 4. Success path returns a fully-initialized value: no error.
func giveEmptyACompositeWithErr4() (*A11, error) {
	return &A11{aptr: new(leaf)}, nil
}

func m48() *int {
	t, err := giveEmptyACompositeWithErr4()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr
}

// 5. Success path initializes the field; only the error path leaves it nil -> no success-path error.
func giveEmptyACompositeWithErr5() (*A11, error) {
	if dummy() {
		return &A11{aptr: new(leaf)}, nil
	}
	return &A11{}, &myErr{}
}

func m58() *int {
	t, err := giveEmptyACompositeWithErr5()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr
}

// 6. Success path leaves the field nil; the error path initializes it -> success-path error.
func giveEmptyACompositeWithErr6() (*A11, error) {
	if dummy() {
		return &A11{}, nil
	}
	return &A11{aptr: new(leaf)}, &myErr{}
}

func m68() *int {
	t, err := giveEmptyACompositeWithErr6()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr //want "accessed field `ptr`"
}

// 7. Discharge must recognize error constructors (`errors.New`, `fmt.Errorf`), not just non-nil
// error literals; both must discharge the nil-fielded error-path return.
func giveEmptyACompositeWithErrNew() (*A11, error) {
	if dummy() {
		return &A11{aptr: new(leaf)}, nil
	}
	return &A11{}, errors.New("boom")
}

func m07a() *int {
	t, err := giveEmptyACompositeWithErrNew()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr
}

func giveEmptyACompositeWithErrf() (*A11, error) {
	if dummy() {
		return &A11{aptr: new(leaf)}, nil
	}
	return &A11{}, fmt.Errorf("boom: %d", 1)
}

func m07b() *int {
	t, err := giveEmptyACompositeWithErrf()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr
}

// 8. Success path fills the field from a local variable (not an inline allocation); a non-nil local
// must be tracked as non-nil at the return. The error path is discharged by `errors.New`.
func giveACompositeFromLocalWithErr() (*A11, error) {
	if dummy() {
		return &A11{}, errors.New("boom")
	}
	p := new(leaf)
	return &A11{aptr: p}, nil
}

func m08a() *int {
	t, err := giveACompositeFromLocalWithErr()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr
}

// 9. Same local-variable success path, but the local is genuinely nil -> the field nilability must
// still flow to the success path and be reported.
func giveNilLocalComposite() (*A11, error) {
	if dummy() {
		return &A11{aptr: new(leaf)}, errors.New("boom")
	}
	var p *leaf
	return &A11{aptr: p}, nil
}

func m09b() *int {
	t, err := giveNilLocalComposite()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr //want "accessed field `ptr`"
}

// 10. Caller-side guard: a caller that derefs the value without checking the error must be flagged,
// because it may observe the error-path value where the field is nil (m58, which checks the error,
// stays safe).
func m10unchecked() *int {
	t, _ := giveEmptyACompositeWithErr5() // error ignored: t may be the error-path (nil-field) value
	return t.aptr.ptr                     //want "accessed field `ptr`"
}

// 11. Discarding the error with `_` can never discharge the guard: there is no error variable to
// check.
func m11discarded() *int {
	t, _ := giveACompositeFromLocalWithErr()
	return t.aptr.ptr //want "accessed field `ptr`"
}

// 12. Error-path zero struct paired with an error variable (not a constructor), returned inside an
// `if err != nil` guard. `err` is not definitely non-nil but not definitely nil either, so the
// error-path zero struct's fields must not be bound into the summary; a caller that checks the error
// is safe.
func giveACompositeOrErrVar() (*A11, error) {
	_, err := giveEmptyACompositeWithErr4() // err: a variable of unknown nilability
	if err != nil {
		return &A11{}, err // zero struct + error variable inside the guard: must not bind
	}
	return &A11{aptr: new(leaf)}, nil
}

func m12errVarChecked() *int {
	t, err := giveACompositeOrErrVar()
	if err != nil {
		return new(int)
	}
	return t.aptr.ptr // safe: error-path zero struct not bound; success path sets aptr
}

// 13. Same constructor, but the caller does not check the error: the caller-side guard still flags
// it, because the value may be the error-path zero struct.
func m13errVarUnchecked() *int {
	t, _ := giveACompositeOrErrVar()
	return t.aptr.ptr //want "accessed field `ptr`"
}
