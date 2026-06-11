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

	. "stubs/github.com/smartystreets/goconvey/convey"
)

// testGoConvey tests goconvey's `So(actual, assertion, expected...)`, whose narrowing fact is
// determined by the assertion argument rather than the called function. The package is
// dot-imported, as is idiomatic for goconvey. `So` is called at function level here because
// NilAway only analyzes `Convey` closure bodies under experimental anonymous-function support.
func testGoConvey(t *testing.T) any {
	switch 0 {
	case 1:
		y, err := errs()
		So(err, ShouldBeNil)
		print(*y)
	case 2:
		// The narrowing direction for `ShouldNotBeNil` is nonnil for the error, so y stays
		// unguarded.
		y, err := errs()
		So(err, ShouldNotBeNil)
		print(*y) //want "lacking guarding"
	case 3:
		y, err := errs()
		So(err, ShouldBeError)
		print(*y) //want "lacking guarding"
	case 4:
		var p *int
		So(p != nil, ShouldBeTrue)
		print(*p)
	case 5:
		var p *int
		So(p == nil, ShouldBeFalse)
		print(*p)
	case 6:
		// Unmodeled assertions have no narrowing effect.
		y, err := errs()
		So(err, ShouldEqual, nil)
		print(*y) //want "lacking guarding"
	case 7:
		// A non-boolean actual with a boolean assertion gets no narrowing (and, importantly,
		// does not crash the analysis).
		var p *int
		So(p, ShouldBeTrue)
		print(*p) //want "unassigned variable `p`"
	case 8:
		// The idiomatic form: `So` inside a `Convey` closure. The closure body is only analyzed
		// under experimental anonymous-function support, so no narrowing is asserted here; this
		// guards the integration against crashes.
		Convey("narrows inside a closure", t, func() {
			y, err := errs()
			So(err, ShouldBeNil)
			consume(y)
		})
	}
	return 0
}
