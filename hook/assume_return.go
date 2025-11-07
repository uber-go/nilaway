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
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/typeshelper"
)

// AssumeReturn returns the producer for the return value of the given call expression, which would
// have the assumed nilability. This is useful for modeling the return value of stdlib and 3rd party
// functions that are not analyzed by NilAway. For example, "errors.New" is assumed to return a
// nonnil value. If the given call expression does not match any known function, nil is returned.
func AssumeReturn(pass *analysishelper.EnhancedPass, call *ast.CallExpr) *annotation.ProduceTrigger {
	if trigger := matchTrustedFuncs(pass, call); trigger != nil {
		return trigger
	}
	return AssumeReturnForErrorWrapperFunc(pass, call)
}

func matchTrustedFuncs(pass *analysishelper.EnhancedPass, call *ast.CallExpr) *annotation.ProduceTrigger {
	for sig, act := range _assumeReturns {
		if sig.match(pass, call) {
			return act(call)
		}
	}
	return nil
}

// AssumeReturnForErrorWrapperFunc returns the producer for the return value of the given call expression which is
// an error wrapper function. This is useful for modeling the return value of error wrapper functions like
// `errors.Wrapf(err, "message")` to return a non-nil error. If the given call expression is not an error wrapper, nil is returned.
func AssumeReturnForErrorWrapperFunc(pass *analysishelper.EnhancedPass, call *ast.CallExpr) *annotation.ProduceTrigger {
	if isErrorWrapperFunc(pass, call) {
		return nonnilProducer(call)
	}
	return nil
}

var _newErrorFuncNameRegex = regexp.MustCompile(`(?i)new[^ ]*error[^ ]*`)

// isErrorWrapperFunc implements a heuristic to identify error wrapper functions (e.g., `errors.Wrapf(err, "message")`).
// It does this by applying the following criteria:
// - the function must have at least one argument of error-implementing type, and
// - the function must return an error-implementing type as its last return value.
func isErrorWrapperFunc(pass *analysishelper.EnhancedPass, call *ast.CallExpr) bool {
	funcIdent := util.FuncIdentFromCallExpr(call)
	if funcIdent == nil {
		return false
	}

	// Return early if the function object is nil or does not return an error.
	var funcObj *types.Func
	if obj := pass.TypesInfo.ObjectOf(funcIdent); obj != nil {
		if fObj, ok := obj.(*types.Func); ok {
			if util.FuncIsErrReturning(typeshelper.GetFuncSignature(fObj.Signature())) {
				funcObj = fObj
			}
		}
	}
	if funcObj == nil {
		return false
	}

	// Check if the function is an error wrapper: consumes an error and returns an error.
	for _, arg := range call.Args {
		// Check if the argument is a call expression.
		if callExpr, ok := arg.(*ast.CallExpr); ok {
			if matchTrustedFuncs(pass, callExpr) != nil {
				// Check if the argument is a trusted error returning function call.
				// Example: `wrapError(errors.New("new error"))`
				return true
			}

			// This is to cover the case `NewInternalError(err.Error())` where the argument is a method call on an error.
			// We want to extract the raw error argument `err` in this case.
			if s, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				t := pass.TypesInfo.TypeOf(s.X)
				if t != nil && util.ImplementsError(t) {
					return true
				}
			}

			// Recursively check if the argument is an error wrapper function call.
			// Example: `wrapError(wrapError(wrapError(err)))`
			if isErrorWrapperFunc(pass, callExpr) {
				return true
			}
		}

		argType := pass.TypesInfo.TypeOf(arg)
		if argType != nil && util.ImplementsError(argType) {
			// Return the raw error argument expression
			return true
		}
	}

	// Check if the function is creating a new error:
	// - consumes a message string and returns an error
	// - matches regex "new*error" as its function name (e.g., `NewInternalError()`)
	if _newErrorFuncNameRegex.MatchString(funcObj.Name()) {
		for i := 0; i < funcObj.Signature().Params().Len(); i++ {
			param := funcObj.Signature().Params().At(i)
			if t, ok := param.Type().(*types.Basic); ok && t.Kind() == types.String && i < len(call.Args) {
				return true
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
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/pkg/errors$`),
		funcNameRegex:  regexp.MustCompile(`^Errorf$`),
	}: nonnilProducer,
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/pkg/errors$`),
		funcNameRegex:  regexp.MustCompile(`^New$`),
	}: nonnilProducer,

	// `errors.Join`
	// Note that `errors.Join` can return nil if all arguments are nil [1]. However, in practice this should rarely
	// happen such that we assume it returns a non-nil error for simplicity. Here we are making a conscious trade-off
	// between soundness and practicality.
	//
	// [1] https://pkg.go.dev/errors#Join
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^errors$`),
		funcNameRegex:  regexp.MustCompile(`^Join$`),
	}: nonnilProducer,
}

var nonnilProducer assumeReturnAction = func(call *ast.CallExpr) *annotation.ProduceTrigger {
	return &annotation.ProduceTrigger{
		Annotation: &annotation.TrustedFuncNonnil{ProduceTriggerNever: &annotation.ProduceTriggerNever{}},
		Expr:       call,
	}
}
