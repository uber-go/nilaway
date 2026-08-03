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

// Regression guard: the map-field variant of decode_localvar_field. A map field set from a local
// variable holding a non-nil map literal is tracked as non-nil, so an index-write to it does not
// panic and is not flagged. The true-positive control below (a nil map variable) must still fire.

type fieldVarState struct {
	tags map[string]string
}

// fieldVarWriteFromVar: `tags` is a non-nil map literal, so the index-write cannot panic.
func fieldVarWriteFromVar() {
	tags := map[string]string{"a": "b"}
	state := &fieldVarState{tags: tags} // map field set from a non-nil local
	state.tags["step"] = "x"            // safe — no //want
	_ = state
}

// fieldVarWriteInlineOK is the negative control: the field is an inline map literal. Isolates the
// variable-vs-literal split.
func fieldVarWriteInlineOK() {
	state := &fieldVarState{tags: map[string]string{"a": "b"}}
	state.tags["step"] = "x" // safe — no //want
	_ = state
}

// fieldVarWriteFromNilVar is the true-positive control: the field is set from a nil map variable, so
// the index-write genuinely panics and must still be reported.
func fieldVarWriteFromNilVar() {
	var tags map[string]string // nil
	state := &fieldVarState{tags: tags}
	state.tags["step"] = "x" //want "written to at an index"
	_ = state
}
