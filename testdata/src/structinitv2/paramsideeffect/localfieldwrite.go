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

// Tests in-place writes of a param's field from a local variable (`x.f = local`). A provably
// non-nil local must not fire; a nil or possibly-nil local must still fire.

package paramsideeffect

type User struct{ Phone *int }
type Req struct{ U *User }

func getNilUser() *User { return nil }

// Case 1 — direct call result (static snapshot): fires (TP).
func enrichCallResult(r *Req) { r.U = getNilUser() }
func readCallResult() *int {
	r := &Req{}
	enrichCallResult(r)
	return r.U.Phone //want "field `U` of param 0 of `enrichCallResult`"
}

// Case 2 — direct nil (static snapshot): fires (TP).
func enrichNil(r *Req) { r.U = nil }
func readNil() *int {
	r := &Req{}
	enrichNil(r)
	return r.U.Phone //want "field `U` of param 0 of `enrichNil`"
}

// Case 3 — local holding a nil call result (tie -> nilable): fires (TP).
func enrichLocalNilCall(r *Req) {
	u := getNilUser()
	r.U = u
}
func readLocalNilCall() *int {
	r := &Req{}
	enrichLocalNilCall(r)
	return r.U.Phone //want "field `U` of param 0 of `enrichLocalNilCall`"
}

// Case 4 — local declared nil (tie -> nilable): fires (TP).
func enrichLocalVarNil(r *Req) {
	var u *User
	r.U = u
}
func readLocalVarNil() *int {
	r := &Req{}
	enrichLocalVarNil(r)
	return r.U.Phone //want "field `U` of param 0 of `enrichLocalVarNil`"
}

// Case 5 — local holding a non-nil allocation (tie -> non-nil): NO fire.
func enrichLocalNonNil(r *Req) {
	u := &User{}
	r.U = u
}
func readLocalNonNil() *int {
	r := &Req{}
	enrichLocalNonNil(r)
	return r.U.Phone // safe: u is provably non-nil, so r.U is non-nil after the call
}
