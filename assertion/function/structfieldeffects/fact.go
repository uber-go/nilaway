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

// ParamFieldEffectsPackageFact is the exported parameter field-effect summary for one package.
type ParamFieldEffectsPackageFact struct {
	Functions []FunctionParamFieldEffects
}

// AFact enables use of the facts passing mechanism in Go's analysis framework.
func (*ParamFieldEffectsPackageFact) AFact() {}

// FunctionParamFieldEffects is a function's parameter field-effect summary within a package fact.
type FunctionParamFieldEffects struct {
	FunctionObjectPath objectpath.Path
	ParamReads         []IndexedFieldPath
	ParamWrites        []IndexedFieldPath
}
