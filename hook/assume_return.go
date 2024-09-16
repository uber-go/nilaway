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

package hook

import (
	"go/ast"
	"regexp"

	"go.uber.org/nilaway/annotation"
	"golang.org/x/tools/go/analysis"
)

// AssumeReturn returns the producer for the return value of the given call expression, which would
// have the assumed nilability. This is useful for modeling the return value of stdlib and 3rd party
// functions that are not analyzed by NilAway. For example, "errors.New" is assumed to return a
// nonnil value. If the given call expression does not match any known function, nil is returned.
func AssumeReturn(pass *analysis.Pass, call *ast.CallExpr) *annotation.ProduceTrigger {
	for sig, act := range _assumeReturns {
		if sig.match(pass, call) {
			return act(call)
		}
	}

	return nil
}

type assumeReturnAction func(call *ast.CallExpr) *annotation.ProduceTrigger

var _assumeReturns = map[trustedFuncSig]assumeReturnAction{
	// `errors.New`
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^errors$`),
		funcNameRegex:  regexp.MustCompile(`^New$`),
	}: nonnilProducer,

	// `fmt.Errorf`
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^fmt$`),
		funcNameRegex:  regexp.MustCompile(`^Errorf$`),
	}: nonnilProducer,

	// `github.com/pkg/errors`
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/pkg/errors$`),
		funcNameRegex:  regexp.MustCompile(`^Errorf$`),
	}: nonnilProducer,
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/pkg/errors$`),
		funcNameRegex:  regexp.MustCompile(`^New$`),
	}: nonnilProducer,
}

var nonnilProducer assumeReturnAction = func(call *ast.CallExpr) *annotation.ProduceTrigger {
	return &annotation.ProduceTrigger{
		Annotation: &annotation.TrustedFuncNonnil{ProduceTriggerNever: &annotation.ProduceTriggerNever{}},
		Expr:       call,
	}
}
