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

	gtassertv2 "stubs/gotest.tools/assert"
	gtassert "stubs/gotest.tools/v3/assert"
)

func testGoTestTools(t *testing.T) any {
	switch 0.0 {
	case 1.0:
		y, err := errs()
		gtassert.NilError(t, err)
		print(*y)
	case 1.1:
		y, err := errs()
		consume(err)
		print(*y) //want "lacking guarding"
	case 2.0:
		// The narrowing direction for `Error` is nonnil for the error, so y stays unguarded.
		y, err := errs()
		gtassert.Error(t, err, "oops")
		print(*y) //want "lacking guarding"
	case 2.1:
		y, err := errs()
		gtassert.ErrorContains(t, err, "oops")
		print(*y) //want "lacking guarding"
	case 3.0:
		var p *int
		gtassert.Assert(t, p != nil)
		print(*p)
	case 3.1:
		// `Assert` with a non-boolean, non-error argument (e.g., a `cmp.Comparison`) should have
		// no narrowing effect (and, importantly, not crash the analysis).
		var p *int
		gtassert.Assert(t, p)
		print(*p) //want "unassigned variable `p`"
	case 3.2:
		// `Assert` with an error-typed argument passes iff the error is nil, just like `NilError`.
		y, err := errs()
		gtassert.Assert(t, err)
		print(*y)
	case 4.0:
		// `ErrorIs` is intentionally unmodeled (see `_splitBlockOn` in the hook package for the
		// rationale), so `err` must still be considered nilable after the call.
		y, err := errs()
		gtassert.ErrorIs(t, err, nil)
		print(*y) //want "lacking guarding"
	case 5.0:
		// The legacy v1/v2 import path `gotest.tools/assert` (without `/v3`) has identical
		// semantics and must be matched by the same trusted function entries.
		y, err := errs()
		gtassertv2.NilError(t, err)
		print(*y)
	case 5.1:
		y, err := errs()
		gtassertv2.Error(t, err, "oops")
		print(*y) //want "lacking guarding"
	}
	return 0
}
