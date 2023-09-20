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
This is a very simple test that just checks direct return of nilable and non-nilable values
to ensure that correct diagnostics are thrown in some common cases

<nilaway no inference>
*/
package testdata

/*
nilable(f)
*/
type A struct {
	f *A
	g *A
}

/*
nilable(c)
*/
func (a *A) foo(b, c *A) *A {
	switch 0 {
	case 1:
		return a
	case 2:
		return a.f //want "returned from `foo.*`"
	case 3:
		return a.g
	case 4:
		return b
	case 5:
		return b.f //want "returned from `foo.*`"
	case 6:
		return b.g
	case 7:
		return c //want "returned from `foo.*`"
	case 8:
		return c.f //want "returned from `foo.*`" "accessed field `f`"
	case 9:
		return c.g //want "accessed field `g`"
	default:
		return nil //want "returned from `foo.*`"
	}
}
