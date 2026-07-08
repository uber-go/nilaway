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

package limitations

// Regression guard: a pointer field initialized from a local variable that holds a non-nil
// allocation is tracked as non-nil, the same as an inline composite literal. A field set from a
// trackable lvalue (local variable, parameter, or field chain) is resolved against the variable's
// real value, so the safe deref below carries no //want.

type decodePayload struct{ id *int }
type decodeOAuthCode struct{ Payload *decodePayload }

type decodeErr struct{}

func (decodeErr) Error() string { return "decode failed" }

// decodeHelper allocates a fresh payload and returns it with a nil error on success; the only
// nil-payload return is paired with a non-nil error.
func decodeHelper(enc string) (*decodePayload, error) {
	pl := &decodePayload{}
	if enc == "" {
		return nil, decodeErr{}
	}
	return pl, nil
}

// decodeAndDeref: `payload` is provably non-nil here (the err!=nil path returned), so the field set
// from it is non-nil at the deref.
func decodeAndDeref(enc string) *int {
	payload, err := decodeHelper(enc)
	if err != nil {
		return nil
	}
	verified := &decodeOAuthCode{Payload: payload} // field set from the checked, non-nil local
	return verified.Payload.id                     // safe — no //want
}

// decodeInlineLiteralOK is the negative control: the field is set from an inline composite literal
// instead of a local variable, isolating the variable-vs-literal split.
func decodeInlineLiteralOK() *int {
	verified := &decodeOAuthCode{Payload: &decodePayload{}}
	return verified.Payload.id // safe — no //want
}
