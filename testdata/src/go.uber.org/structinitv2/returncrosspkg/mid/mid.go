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

// Package mid is the middle hop of the transitive cross-package return-shape test: it forwards a
// constructor defined in lib, re-exporting its shape so the app package (two hops away) still sees
// the deep nil field.
package mid

import "go.uber.org/structinitv2/returncrosspkg/lib"

// ForwardImportedResult forwards lib.ReturnDeepNil's result, re-exporting the deep shape so a caller
// two packages away inherits it.
func ForwardImportedResult() *lib.Outer {
	x := lib.ReturnDeepNil()
	return x
}
