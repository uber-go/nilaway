//  Copyright (c) 2025 Uber Technologies, Inc.
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

package analysishelper

import (
	"go/ast"
	"go/constant"

	"golang.org/x/tools/go/analysis"
)

// EnhancedPass is a drop-in replacement for `*analysis.Pass` that provides additional helper methods
// to make it easier to work with the analysis pass.
type EnhancedPass struct {
	*analysis.Pass
}

// NewEnhancedPass creates a new EnhancedPass from the given *analysis.Pass.
func NewEnhancedPass(pass *analysis.Pass) *EnhancedPass {
	return &EnhancedPass{Pass: pass}
}

// IsZero returns if the given expression is evaluated to integer zero at compile time. For
// example, zero literal, zero const or binary expression that evaluates to zero, e.g., 1 - 1
// should all return true. Note the function will return false for zero string `"0"`.
func (p *EnhancedPass) IsZero(expr ast.Expr) bool {
	tv, ok := p.TypesInfo.Types[expr]
	if !ok {
		return false
	}
	intValue, ok := constant.Val(tv.Value).(int64)
	return ok && intValue == 0
}
