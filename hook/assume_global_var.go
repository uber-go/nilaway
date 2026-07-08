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

package hook

import (
	"go/ast"
	"regexp"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/analysishelper"
)

// AssumeGlobalVarNonnil returns a nonnil producer for reads of well-known standard-library global
// variables that are documented to be non-nil but whose initializers NilAway would otherwise infer
// as possibly-nil. For example, `os.Stdout`/`os.Stderr`/`os.Stdin` are assigned from `NewFile`,
// which returns nil only for a negative file descriptor, and `os.Args` is always populated by the
// runtime; in practice all of these are non-nil. Modeling them here removes a large class of false
// positives on idiomatic code like `io.Copy(os.Stdout, r)` or `os.Args[1]`. If the given selector
// does not match any known global, nil is returned.
func AssumeGlobalVarNonnil(pass *analysishelper.EnhancedPass, sel *ast.SelectorExpr) *annotation.ProduceTrigger {
	for i := range _assumeGlobalVarsNonnil {
		if _assumeGlobalVarsNonnil[i].matchSel(pass, sel) {
			return &annotation.ProduceTrigger{
				Annotation: &annotation.TrustedFuncNonnil{ProduceTriggerNever: &annotation.ProduceTriggerNever{}},
				Expr:       sel,
			}
		}
	}
	return nil
}

// _assumeGlobalVarsNonnil is the set of standard-library global variables assumed to be non-nil.
var _assumeGlobalVarsNonnil = []trustedSig{
	// `os.Stdout`, `os.Stderr`, `os.Stdin`, and `os.Args` are documented to be non-nil.
	{
		kind:           _var,
		enclosingRegex: regexp.MustCompile(`^os$`),
		nameRegex:      regexp.MustCompile(`^(Stdout|Stderr|Stdin|Args)$`),
	},
}
