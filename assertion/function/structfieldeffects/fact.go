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

package structfieldeffects

import "golang.org/x/tools/go/types/objectpath"

// ParamFieldReadsPackageFact is the exported parameter field-read summary for one package.
type ParamFieldReadsPackageFact struct {
	Functions []FunctionParamFieldReads
}

// AFact enables use of the facts passing mechanism in Go's analysis framework.
func (*ParamFieldReadsPackageFact) AFact() {}

// FunctionParamFieldReads is a function's parameter field-read summary within a package fact.
type FunctionParamFieldReads struct {
	FunctionObjectPath objectpath.Path
	ParamReads         []IndexedFieldPath
}
