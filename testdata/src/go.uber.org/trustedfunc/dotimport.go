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

	. "stubs/github.com/stretchr/testify/require"
)

// testDotImport tests that trusted functions of a dot-imported package (called as bare
// identifiers rather than selector expressions) are matched and narrow just like qualified calls.
func testDotImport(t *testing.T) any {
	switch 0 {
	case 1:
		y, err := errs()
		NoError(t, err)
		print(*y)
	case 2:
		// The narrowing direction for `Error` is nonnil for the error, so y stays unguarded.
		y, err := errs()
		Error(t, err)
		print(*y) //want "lacking guarding"
	case 3:
		var p *int
		True(t, p != nil)
		print(*p)
	case 4:
		// `Equal` routes through generateComparators, which extracts the called name; this
		// exercises that path for bare-identifier calls.
		y, err := errs()
		Equal(t, nil, err)
		print(*y)
	case 5:
		// Function values are NOT matched (the identifier resolves to a variable rather than the
		// trusted function), so no narrowing happens.
		y, err := errs()
		f := NoError
		f(t, err)
		print(*y) //want "lacking guarding"
	}
	return 0
}
