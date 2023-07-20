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
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// NOTE: in the future, when we implement  to add contracts, this trusted func mechanism can possibly be replaced with that one.

// AsTrustedFuncAction checks a function call AST node to see if it is one of the trusted functions, and if it is then runs
// the corresponding action and returns that as the output along with a bool indicating success or failure.
// For example, a binary expression `x != nil` is returned for trusted function `assert.NotNil(t, x)`, while a `TrustedFuncNonnil` producer is returned for `errors.New(s)`
func AsTrustedFuncAction(expr ast.Expr, p *analysis.Pass) (any, bool) {
	if call, ok := expr.(*ast.CallExpr); ok {
		for f, a := range trustedFuncs {
			if f.match(call, p) {
				if t := a.action(call, a.argIndex, p); t != nil {
					return t, true
				}
			}
		}
	}
	return nil, false
}

// funcKind indicates the kind of the trusted function:
// (1) _method: it is a method of a struct;
// (2) _func: it is a top-level function of a package.
type funcKind uint8

const (
	_method funcKind = iota
	_func
)

// trustedFuncSig defines the signature of a function that we "trust" to have a certain effect on its arguments, for example.
type trustedFuncSig struct {
	kind           funcKind
	enclosingRegex *regexp.Regexp
	funcNameRegex  *regexp.Regexp
}

// match checks if a given call expression matches with a trusted function's signature. Namely,
// it performs a strict matching for the function / method name and a user-defined regex match for
// the enclosing package or struct path.
func (t *trustedFuncSig) match(call *ast.CallExpr, pass *analysis.Pass) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !t.funcNameRegex.MatchString(sel.Sel.Name) {
		return false
	}

	// Match fully qualified path of the call expression with the expected path specified in `t`
	// if function, match enclosing "<pkg path>". E.g., for `assert.Error(err)`, path = github.com/stretchr/testify/assert
	// if method, match with "<pkg path>.<struct name>". E.g., for `u.Require().Error(err)`, path = github.com/stretchr/testify/require.Assertions
	if funcObj, ok := pass.TypesInfo.ObjectOf(sel.Sel).(*types.Func); ok && funcObj.Pkg() != nil {
		recv := funcObj.Type().(*types.Signature).Recv()
		path := funcObj.Pkg().Path()

		// return early if the kind of `t` and `funcObj` don't match. Both should be functions (or methods) for the match to be performed
		// `recv != nil` implies `funcObj` is a method, while `recv == nil` means it is a function
		if (t.kind == _func && recv != nil) || (t.kind == _method && recv == nil) {
			return false
		}

		// add struct name to the path
		if recv != nil {
			if n, ok := util.UnwrapPtr(recv.Type()).(*types.Named); ok {
				path = path + "." + n.Obj().Name()
			} else {
				// we should likely never hit this case, but is only added for extra safety since
				// `util.TypeAsDeeplyNamed` can return nil
				return false
			}
		}
		return t.enclosingRegex.MatchString(path)
	}
	return false
}

type action func(call *ast.CallExpr, argIndex int, p *analysis.Pass) any

// trustedFuncAction defines the effect the trusted function can have on its argument `argIndex`.
// If `argIndex = -1`, the action is more general (e.g., returning a producer), not specific to any argument
type trustedFuncAction struct {
	action   action
	argIndex int
}

// nilBinaryExpr returns `expr == nil`. This is useful, for example, in asserting nilability of an object in the `testify` library: `assert.Nil(t, obj)`, which gets interpreted as `if obj == nil {...}` by preprocess
var nilBinaryExpr action = func(call *ast.CallExpr, argIndex int, _ *analysis.Pass) any {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return newNilBinaryExpr(call.Args[argIndex], token.EQL)
}

// nonnilBinaryExpr returns `expr != nil`. This is useful, for example, in asserting non-nilability of an object in the `testify` library: `assert.NotNil(t, obj)`, which gets interpreted as `if obj != nil {...}` by preprocess
var nonnilBinaryExpr action = func(call *ast.CallExpr, argIndex int, _ *analysis.Pass) any {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return newNilBinaryExpr(call.Args[argIndex], token.NEQ)
}

// selfExpr returns the expression itself. Currently, this is meant for only checking boolean expressions, implying `if expr {...}`, i.e., `if expr == true {...}`.
// This is useful, for example, is asserting a boolean true value in the `testify` library: `assert.True(t, ok)`, which gets interpreted as `if ok {...}` by preprocess
var selfExpr action = func(call *ast.CallExpr, argIndex int, _ *analysis.Pass) any {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return call.Args[argIndex]
}

// negatedSelfExpr is same as selfExpr, but returns a negated expr. E.g., `assert.False(t, ok)`, which gets interpreted as `if !ok {...}` by preprocess
var negatedSelfExpr action = func(call *ast.CallExpr, argIndex int, _ *analysis.Pass) any {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	arg := call.Args[argIndex]
	return &ast.UnaryExpr{
		OpPos: arg.Pos(),
		Op:    token.NOT,
		X:     arg,
	}
}

var nonnilProducer action = func(call *ast.CallExpr, argIndex int, _ *analysis.Pass) any {
	return &annotation.ProduceTrigger{
		Annotation: annotation.TrustedFuncNonnil{},
		Expr:       call,
	}
}

func newNilBinaryExpr(arg ast.Expr, op token.Token) *ast.BinaryExpr {
	return &ast.BinaryExpr{
		X:     arg,
		OpPos: arg.Pos(),
		Op:    op,
		Y: &ast.Ident{
			NamePos: arg.Pos(),
			Name:    "nil",
		},
	}
}

// requireComparators handles a slightly more sophisticated case for asserting the length of a
// slice, e.g., length of a slice is greater than 0 implies the slice is not nil.
var requireComparators action = func(call *ast.CallExpr, startIndex int, pass *analysis.Pass) any {
	// Comparator function calls must have at least two arguments.
	if len(call.Args[startIndex:]) < 2 {
		return nil
	}

	// We handle multiple comparator functions here, so we store the function name to do different
	// actions depending on their semantics.
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	funcName := sel.Sel.Name

	// The slice expression and the length expression can be at either places (i.e., both
	// `Equal(len(s), 1)` and `Equal(1, len(s))` are allowed), so we search for the slice expression, the
	// other will be treated as length expression.
	for argIndex, expr := range call.Args[startIndex : startIndex+2] {
		// Check if the expression is `len(<slice_expr>)`.
		callExpr, ok := expr.(*ast.CallExpr)
		if !ok {
			continue
		}
		wrapperFunc, ok := callExpr.Fun.(*ast.Ident)
		if !ok {
			continue
		}
		if pass.TypesInfo.ObjectOf(wrapperFunc) != util.BuiltinLen || len(callExpr.Args) != 1 {
			continue
		}

		// Check if `<slice_expr>` is of slice type.
		sliceExpr, lenExpr := callExpr.Args[0], call.Args[startIndex+1-argIndex]
		_, ok = pass.TypesInfo.TypeOf(sliceExpr).Underlying().(*types.Slice)
		if !ok {
			continue
		}

		// Then, we can treat the other argument as the length expression and check its
		// compile-time value.
		typeAndValue, ok := pass.TypesInfo.Types[lenExpr]
		if !ok {
			continue
		}

		v, ok := constant.Val(typeAndValue.Value).(int64)
		if !ok {
			continue
		}

		// Now, based on the semantics of the function, we can create artificial nonnil checks for
		// the following cases.
		switch funcName {
		case "Equal", "Equalf": // len(s) == [positive_int]
			if v > 0 {
				return newNilBinaryExpr(sliceExpr, token.NEQ)
			}
		case "NotEqual", "NotEqualf": // len(s) != [zero]
			if v == 0 {
				return newNilBinaryExpr(sliceExpr, token.NEQ)
			}
		// Note the check for `argIndex` in the following cases, we need to make sure the slice expr
		// is at the correct position since these are inequality checks.
		case "Greater", "Greaterf": // len(s) > [non_negative_int]
			if argIndex == 0 && v >= 0 {
				return newNilBinaryExpr(sliceExpr, token.NEQ)
			}
		case "GreaterOrEqual", "GreaterOrEqualf": // len(s) >= [positive_int]
			if argIndex == 0 && v > 0 {
				return newNilBinaryExpr(sliceExpr, token.NEQ)
			}
		case "Less", "Lessf": // [non_negative_int] < len(s)
			if argIndex == 1 && v >= 0 {
				return newNilBinaryExpr(sliceExpr, token.NEQ)
			}
		case "LessOrEqual", "LessOrEqualf": // [positive_int] <= len(s)
			if argIndex == 1 && v > 0 {
				return newNilBinaryExpr(sliceExpr, token.NEQ)
			}
		}
	}

	return nil
}

// requireLen handles `require.Len` calls for slices: asserting the length of a slice > 0 implies
// the slice is not nil.
var requireLen action = func(call *ast.CallExpr, startIndex int, pass *analysis.Pass) any {
	if len(call.Args[startIndex:]) < 2 {
		return nil
	}

	// Check if the slice and length expressions are valid.
	sliceExpr, lenExpr := call.Args[startIndex], call.Args[startIndex+1]
	if _, ok := pass.TypesInfo.TypeOf(sliceExpr).Underlying().(*types.Slice); !ok {
		return nil
	}

	typeAndValue, ok := pass.TypesInfo.Types[lenExpr]
	if !ok {
		return nil
	}
	if typeAndValue.Value == nil {
		return nil
	}

	v, ok := constant.Val(typeAndValue.Value).(int64)
	if !ok {
		return nil
	}

	// Len(sliceExpr, [positive_int]) implies that the slice is nonnil.
	if v > 0 {
		return newNilBinaryExpr(sliceExpr, token.NEQ)
	}

	return nil
}

// trustedFuncs defines the map of trusted functions and their actions
var trustedFuncs = map[trustedFuncSig]trustedFuncAction{
	// `suite.Suite` and `assert.Assertions`
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		funcNameRegex:  regexp.MustCompile(`^(Nil(f)?|NoError(f)?)$`),
	}: {action: nilBinaryExpr, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		funcNameRegex:  regexp.MustCompile(`^(NotNil(f)?|Error(f)?)$`),
	}: {action: nonnilBinaryExpr, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		funcNameRegex:  regexp.MustCompile(`^True(f)?$`),
	}: {action: selfExpr, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		funcNameRegex:  regexp.MustCompile(`^False(f)?$`),
	}: {action: negatedSelfExpr, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		funcNameRegex:  regexp.MustCompile(`^(Greater(f)?|Less(f)?|(GreaterOr|LessOr)?Equal(f)?)$`),
	}: {action: requireComparators, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		funcNameRegex:  regexp.MustCompile(`^Len(f)?$`),
	}: {action: requireLen, argIndex: 0},

	// `assert` and `require`
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(assert|require)$`),
		funcNameRegex:  regexp.MustCompile(`^(Nil(f)?|NoError(f)?)$`),
	}: {action: nilBinaryExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(assert|require)$`),
		funcNameRegex:  regexp.MustCompile(`^(NotNil(f)?|Error(f)?)$`),
	}: {action: nonnilBinaryExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(assert|require)$`),
		funcNameRegex:  regexp.MustCompile(`^True(f)?$`),
	}: {action: selfExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(assert|require)$`),
		funcNameRegex:  regexp.MustCompile(`^False(f)?$`),
	}: {action: negatedSelfExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(assert|require)$`),
		funcNameRegex:  regexp.MustCompile(`^(Greater(f)?|Less(f)?|(GreaterOr|LessOr)?Equal(f)?)$`),
	}: {action: requireComparators, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(assert|require)$`),
		funcNameRegex:  regexp.MustCompile(`^Len(f)?$`),
	}: {action: requireLen, argIndex: 1},

	// `errors.New`
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^errors$`),
		funcNameRegex:  regexp.MustCompile(`^New$`),
	}: {action: nonnilProducer, argIndex: -1},

	// `fmt.Errorf`
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^fmt$`),
		funcNameRegex:  regexp.MustCompile(`^Errorf$`),
	}: {action: nonnilProducer, argIndex: -1},

	// `github.com/pkg/errors`
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/pkg/errors$`),
		funcNameRegex:  regexp.MustCompile(`^Errorf$`),
	}: {action: nonnilProducer, argIndex: -1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/pkg/errors$`),
		funcNameRegex:  regexp.MustCompile(`^New$`),
	}: {action: nonnilProducer, argIndex: -1},
}

// BuiltinAppend is used to check the builtin append method for slice
const BuiltinAppend = "append"

// BuiltinNew is used to check the builtin `new` function
const BuiltinNew = "new"
