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
	"go/types"
	"regexp"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util"
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

	// Check if the function is an error wrapper function
	if isErrorWrapperFunc(pass, call) {
		return nonnilProducer(call)
	}

	return nil
}

// isErrorWrapperFunc implements a heuristic to identify error wrapper functions (e.g., `errors.Wrapf(err, "message")`).
// It does this by applying the following criteria:
// - The function must have at least one argument of error-implementing type.
// - The function can return several values, but at least one of them must be of error-implementing type.
func isErrorWrapperFunc(pass *analysis.Pass, call *ast.CallExpr) bool {
	funcIdent := util.FuncIdentFromCallExpr(call)
	if funcIdent == nil {
		return false
	}

	obj := pass.TypesInfo.ObjectOf(funcIdent)
	if obj == nil {
		return false
	}

	funcObj, ok := obj.(*types.Func)
	if !ok {
		return false
	}
	if util.FuncIsErrReturning(funcObj) {
		for _, arg := range call.Args {
			if callExpr, ok := arg.(*ast.CallExpr); ok {
				return isErrorWrapperFunc(pass, callExpr)
			}

			if argIdent, ok := arg.(*ast.Ident); ok {
				if argObj := pass.TypesInfo.ObjectOf(argIdent); argObj != nil {
					if types.Implements(argObj.Type(), util.ErrorType.Underlying().(*types.Interface)) {
						return true
					}
				}
			}
		}
	}
	return false
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
