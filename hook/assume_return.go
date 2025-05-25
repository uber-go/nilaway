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

	return AssumeReturnForErrorWrapperFunc(pass, call)
}

// AssumeReturnForErrorWrapperFunc returns the producer for the return value of the given call expression which is
// an error wrapper function. This is useful for modeling the return value of error wrapper functions like
// `errors.Wrapf(err, "message")` to return a non-nil error. If the given call expression is not an error wrapper, nil is returned.
func AssumeReturnForErrorWrapperFunc(pass *analysis.Pass, call *ast.CallExpr) *annotation.ProduceTrigger {
	if isErrorWrapperFunc(pass, call) {
		return nonnilProducer(call)
	}
	return nil
}

// isErrorWrapperFunc implements a heuristic to identify error wrapper functions (e.g., `errors.Wrapf(err, "message")`).
// It does this by applying the following criteria:
// - the function must have at least one argument of error-implementing type, and
// - the function must return an error-implementing type as its last return value.
func isErrorWrapperFunc(pass *analysis.Pass, call *ast.CallExpr) bool {
	funcIdent := util.FuncIdentFromCallExpr(call)
	if funcIdent == nil {
		return false
	}

	obj := pass.TypesInfo.ObjectOf(funcIdent)
	if obj == nil {
		return false
	}

	// If the call expr is built-in `new`, then we check if its argument type implements the error interface.
	// This case particularly gets triggered for the expression: `Wrap(new(MyErrorStruct), "message")`.
	if obj == util.BuiltinNew {
		if len(call.Args) == 0 {
			return false
		}
		if argIdent := util.IdentOf(call.Args[0]); argIdent != nil {
			ptr := types.NewPointer(pass.TypesInfo.TypeOf(argIdent))
			if types.Implements(ptr, util.ErrorInterface) {
				return true
			}
		}

		return false
	}

	funcObj, ok := obj.(*types.Func)
	if !ok {
		return false
	}

	if util.FuncIsErrReturning(funcObj.Signature()) {
		args := call.Args

		// If the function is a method, we need to check if the receiver is an error-implementing type.
		// This is to cover the case where some error wrappers facilitate a chaining functionality, i.e., the receiver
		// is an error-implementing type (e.g., Wrap().WithOtherFields()). By adding the receiver to the argument list,
		// we can check if it is an error-implementing type and support this case.
		if funcObj.Type().(*types.Signature).Recv() != nil {
			args = append(args, call.Fun)
		}
		for _, arg := range args {
			if callExpr, ok := ast.Unparen(arg).(*ast.CallExpr); ok {
				if isErrorWrapperFunc(pass, callExpr) {
					return true
				}
			}

			if argIdent := util.IdentOf(arg); argIdent != nil {
				argObj := pass.TypesInfo.ObjectOf(argIdent)
				if util.ImplementsError(argObj) {
					return true
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
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/pkg/errors$`),
		funcNameRegex:  regexp.MustCompile(`^Errorf$`),
	}: nonnilProducer,
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/pkg/errors$`),
		funcNameRegex:  regexp.MustCompile(`^New$`),
	}: nonnilProducer,
}

var nonnilProducer assumeReturnAction = func(call *ast.CallExpr) *annotation.ProduceTrigger {
	return &annotation.ProduceTrigger{
		Annotation: &annotation.TrustedFuncNonnil{ProduceTriggerNever: &annotation.ProduceTriggerNever{}},
		Expr:       call,
	}
}
