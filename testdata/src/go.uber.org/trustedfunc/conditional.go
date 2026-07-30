//  Copyright (c) 2026 Uber Technologies, Inc.
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

package trustedfunc

import (
	"testing"

	"stubs/github.com/stretchr/testify/assert"
)

// testAssertConditional tests bool-returning testify assertions used as conditionals, e.g.,
// `if assert.NoError(t, err) {...}`: the assertion returns true iff it passed, so the implied
// fact holds inside the then-branch (with no assumption about test termination), while the
// else-branch and the code after the conditional gain no information.
func testAssertConditional(t *testing.T, a *assert.Assertions) any {
	switch 0 {
	case 1:
		y, err := errs()
		if assert.NoError(t, err) {
			print(*y)
		}
	case 2:
		// No narrowing survives past the conditional.
		y, err := errs()
		if assert.NoError(t, err) {
			print(*y)
		}
		print(*y) //want "lacking guarding"
	case 3:
		// The narrowing direction for `Error` is nonnil for the error, so y stays unguarded
		// even inside the then-branch.
		y, err := errs()
		if assert.Error(t, err) {
			print(*y) //want "lacking guarding"
		}
	case 4:
		y, _ := errs()
		if assert.NotNil(t, y) {
			print(*y)
		}
		print(*y) //want "lacking guarding"
	case 5:
		// The narrowing direction for `Nil` is nil, so y must not be treated as nonnil.
		y, _ := errs()
		if assert.Nil(t, y) {
			print(*y) //want "lacking guarding"
		}
	case 6:
		var p *int
		if assert.True(t, p != nil) {
			print(*p)
		}
	case 7:
		var p *int
		if assert.False(t, p == nil) {
			print(*p)
		}
	case 8:
		// Method form on `assert.Assertions`.
		y, err := errs()
		if a.NoError(err) {
			return y
		}
	case 9:
		// The `ok := ...; if ok` form is recognized as well.
		y, err := errs()
		ok := assert.NoError(t, err)
		if ok {
			return y
		}
	}
	return 0
}
