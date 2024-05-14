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

package downstream

import (
	"go.uber.org/factexport/upstream"
)

func foo() {
	// The contract sub-analyzer does not really report potential nil panics, the following
	// calls are just to ensure we add the upstream dependency and the sub-analyzer is able to
	// import facts about it.
	upstream.ExportedManual(nil)
	upstream.ExportedInferred(nil)
}

// This is a local function that has a contract that should be combined with the imported facts.
// contract(nonnil -> nonnil)
func localManual(p *int) *int {
	if p != nil {
		a := 1
		return &a
	}
	return nil
}
