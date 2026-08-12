//  Copyright (c) 2024 Uber Technologies, Inc.
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

package upstream

// This tests the export of contracts from the upstream package.

type ExportedMsg struct {
	Field *int
}

type exportedError struct{}

func (*exportedError) Error() string { return "error" }

func ExportedField(msg *ExportedMsg) error { //want ExportedField:"field"
	if msg.Field == nil {
		return &exportedError{}
	}
	return nil
}

func (m *ExportedMsg) ExportedMethod(msg *ExportedMsg) error { //want ExportedMethod:"field"
	if msg.Field == nil {
		return &exportedError{}
	}
	return nil
}

// contract(nonnil -> nonnil)
func ExportedManual(p *int) *int { //want ExportedManual:"&\\[{\\[nonnil\\] \\[nonnil\\]}\\]"
	if p != nil {
		a := 1
		return &a
	}
	return nil
}

func ExportedInferred(p *int) *int { //want ExportedInferred:"&\\[{\\[nonnil\\] \\[nonnil\\]}\\]"
	if p != nil {
		a := 1
		return &a
	}
	return nil
}

// contract(nonnil -> nonnil)
func unexportedManual(p *int) *int { // Notice here we do not want to export the contracts for it.
	if p != nil {
		a := 1
		return &a
	}
	return nil
}

func unexportedInferred(p *int) *int { // Notice here we do not want to export the contracts for it.
	if p != nil {
		a := 1
		return &a
	}
	return nil
}

func ExportedTrue(p *int) bool { //want ExportedTrue:"&\\[{\\[\\] \\[\\] true => arg\\(0\\)\\.nonnil\\}\\]"
	return p != nil
}

type ExportedValidator struct{}

func (ExportedValidator) ExportedTrueMethod(p *int) bool { //want ExportedTrueMethod:"&\\[{\\[\\] \\[\\] true => arg\\(0\\)\\.nonnil\\}\\]"
	return p != nil
}

type ExportedEvent struct{ Action *int }

func (e ExportedEvent) IsValid() bool { //want IsValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\}\\]"
	return e.Action != nil
}
