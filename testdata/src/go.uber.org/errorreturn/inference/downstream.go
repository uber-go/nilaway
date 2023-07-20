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

package inference

import "go.uber.org/errorreturn/inference/upstream"

type dummyCallback struct{}

func (dcb dummyCallback) GetErrorObj() error {
	return nil
}

func foo() {
	// False negative case! The reason why the error here does not get reported here is because when analyzing package `upstream`,
	// the non-error trigger for function `problem` is removed by `FilterTriggersForErrorReturn`. Why? Because the annotation site
	// `Result 0 of GetErrorObj` is found to be `Undetermined`, really unknown in this case, since `upstream` does not have the
	// implementation of `GetErrorObj`. The compute function passed to `FilterTriggersForErrorReturn` in`inferred_annotation_map.go`
	// treats all undetermined sites to be non-nil. This is actually a fair assumption based on the understanding that the
	// inference algorithm does not propagate non-nil forward, meaning that the sites are interpreted as "nonnil-hence-left-undetermined".
	// Now when analyzing `downstream.go`, NilAway will indeed realize that `Callback.GetErrorObj()` should be nilable, but by that point,
	// the function `problem` will never be analyzed again, and the error goes unreported.
	//
	// We are leaving this case as a known limitation in NilAway, since we believe this coding pattern to be rare in practice.
	// (This issue is tracked in

	_ = *upstream.EntryPoint(&dummyCallback{}) // error should be reported here
}
