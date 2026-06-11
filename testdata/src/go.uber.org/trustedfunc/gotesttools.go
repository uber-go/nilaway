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

// nilable(x)
func testGoTestTools(t *testing.T, x any) any {
	switch 0.0 {
	case 1.0:
		y, err := errs()
		gtassert.NilError(t, err)
		return y
	case 1.1:
		y, err := errs()
		consume(err)
		return y //want "returned"
	case 2.0:
		y, err := errs()
		gtassert.Error(t, err, "oops")
		return y //want "returned"
	case 2.1:
		_, err := errs()
		gtassert.Error(t, err, "oops")
		takesNonnil(err)
	case 2.2:
		_, err := errs()
		gtassert.ErrorContains(t, err, "oops")
		takesNonnil(err)
	case 3.0:
		gtassert.Assert(t, x != nil)
		return x
	case 3.1:
		// `Assert` with a non-boolean argument (e.g., a `cmp.Comparison`) should have no
		// narrowing effect (and, importantly, not crash the analysis).
		gtassert.Assert(t, x)
		return x //want "returned"
	case 3.2:
		// `Assert` with an error-typed argument passes iff the error is nil, just like `NilError`.
		y, err := errs()
		gtassert.Assert(t, err)
		return y
	case 3.3:
		// The error form narrows the error to nil (not nonnil): a passing `Assert(t, err)` means
		// `err == nil`, so passing it to a nonnil-expecting function must still be reported.
		_, err := errs()
		gtassert.Assert(t, err)
		takesNonnil(err) //want "passed"
	case 4.0:
		// `ErrorIs` is intentionally unmodeled (see `_splitBlockOn` in the hook package for the
		// rationale), so `err` must still be considered nilable after the call.
		_, err := errs()
		gtassert.ErrorIs(t, err, nil)
		takesNonnil(err) //want "passed"
	case 5.0:
		// The legacy v1/v2 import path `gotest.tools/assert` (without `/v3`) has identical
		// semantics and must be matched by the same trusted function entries.
		y, err := errs()
		gtassertv2.NilError(t, err)
		return y
	case 5.1:
		_, err := errs()
		gtassertv2.Error(t, err, "oops")
		takesNonnil(err)
	}
	return 0
}
