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

package assertiontree

import (
	"fmt"
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util"
)

type funcAssertionNode struct {
	assertionNodeCommon

	// declaring identifier for this function
	decl *types.Func
}

func (f *funcAssertionNode) MinimalString() string {
	return fmt.Sprintf("func<%s>", f.decl.Name())
}

// DefaultTrigger for a function node is that function's return annotation
func (f *funcAssertionNode) DefaultTrigger() annotation.ProducingAnnotationTrigger {
	if util.FuncNumResults(f.decl) != 1 {
		panic("only functions with singular result should be entered into the assertion tree")
	}

	if f.decl.Type().(*types.Signature).Recv() != nil {
		return annotation.MethodReturn{
			TriggerIfNilable: annotation.TriggerIfNilable{
				Ann: annotation.RetKeyFromRetNum(f.decl, 0)}}
	}
	return annotation.FuncReturn{
		TriggerIfNilable: annotation.TriggerIfNilable{
			Ann: annotation.RetKeyFromRetNum(f.decl, 0)}}
}
