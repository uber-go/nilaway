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

// Package hook implements a hook framework for NilAway where it hooks into different parts to
// provide additional context for certain function calls. This is useful for well-known standard
// or 3rd party libraries where we can encode certain knowledge about them (e.g.,
// `assert.Nil(t, x)` implies `x == nil`) and use that to provide better analysis.
package hook

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

// As checks a function call AST node to see if it is one of the trusted functions, and if it is
// then runs the corresponding action and returns that as the output along with a bool indicating
// success or failure. For example, a binary expression `x != nil` is returned for trusted function
// `assert.NotNil(t, x)`, while a `TrustedFuncNonnil` producer is returned for `errors.New(s)`
func As(expr ast.Expr, p *analysis.Pass) (any, bool) {
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

var nonnilProducer action = func(call *ast.CallExpr, _ int, _ *analysis.Pass) any {
	return &annotation.ProduceTrigger{
		Annotation: &annotation.TrustedFuncNonnil{ProduceTriggerNever: &annotation.ProduceTriggerNever{}},
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

// The constant (enum) values below represent the possible values of an expected expression in a comparison
// E.g., `Equal(1, len(s))`, where `1` is the expected expression and is assigned the value `_greaterThanZero`.
// E.g., `Equal(nil, err)`, where `nil` is the expected expression and is assigned the value `_nil`.
type expectedValue int

const (
	_unknown expectedValue = iota // init value when the expected expression value is yet to be determined
	_zero
	_greaterThanZero
	_nil
	_false
)

// requireComparators handles slightly more sophisticated cases of comparisons. We currently support:
// - slice length comparison (e.g., `Equal(1, len(s))`, implying len(s) > 0, meaning s is nonnil)
// - nil comparison (e.g., `Equal(nil, err)`).
var requireComparators action = func(call *ast.CallExpr, startIndex int, pass *analysis.Pass) any {
	// Comparator function calls must have at least two arguments.
	if len(call.Args[startIndex:]) < 2 {
		return nil
	}

	// We now find the actual and expected expressions, where expected is the constant value that actual expression is
	// compared against. For example, in `Equal(1, len(s))`, expected is 1, and actual is `s`. However, the position
	// of the actual and expected expressions can be swapped, e.g., `Equal(len(s), 1)`. We handle both cases below. For
	// example, for length comparison, we search for the slice expression, the other will be treated as length expression.

	var actualExpr ast.Expr
	var actualExprIndex int
	expectedExprValue := _unknown

	for argIndex, expr := range call.Args[startIndex : startIndex+2] {
		switch expr := expr.(type) {
		case *ast.CallExpr:
			// Check if the expression is `len(<slice_expr>)`.
			wrapperFunc, ok := expr.Fun.(*ast.Ident)
			if !ok {
				continue
			}
			if pass.TypesInfo.ObjectOf(wrapperFunc) != util.BuiltinLen || len(expr.Args) != 1 {
				continue
			}
			// Check if `<slice_expr>` is of slice type.
			sliceExpr, lenExpr := expr.Args[0], call.Args[startIndex+1-argIndex]
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

			actualExpr = sliceExpr
			actualExprIndex = argIndex
			if v == 0 {
				expectedExprValue = _zero
			} else if v > 0 {
				expectedExprValue = _greaterThanZero
			}

		case *ast.Ident:
			// Check if the expected expression is `nil`.
			if expr.Name == "nil" {
				actualExpr = call.Args[startIndex+1-argIndex]
				actualExprIndex = argIndex
				expectedExprValue = _nil
			}
		}
	}

	// likely represents a case that we don't support.
	if expectedExprValue == _unknown {
		return nil
	}

	// we now generate the comparators based on the semantics of the function.
	return generateComparators(call, actualExpr, actualExprIndex, expectedExprValue)
}

// requireZeroComparators handles a special case of comparators checking for zero values (e.g., `Empty(err)`, i.e. err == nil).
// We currently support the following cases of zero comparators:
// - `nil` for pointers
// - `false` for booleans
// - `len == 0` for slices, maps, and channels
var requireZeroComparators action = func(call *ast.CallExpr, index int, pass *analysis.Pass) any {
	expr := call.Args[index]
	if expr == nil {
		return nil
	}

	exprType := pass.TypesInfo.TypeOf(expr).Underlying()
	switch t := exprType.(type) {
	case *types.Pointer, *types.Interface:
		return generateComparators(call, expr, index, _nil)
	case *types.Slice, *types.Map, *types.Chan:
		return generateComparators(call, expr, index, _zero)
	case *types.Basic:
		if t.Kind() == types.Bool {
			return generateComparators(call, expr, index, _false)
		}
	}

	return nil
}

// generateComparators generates comparators based on the semantics of the function.
func generateComparators(call *ast.CallExpr, actualExpr ast.Expr, actualExprIndex int, expectedVal expectedValue) any {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	funcName := sel.Sel.Name

	// Now, based on the semantics of the function, we can create artificial nonnil checks for
	// the following cases.
	// - slice length comparison. E.g., `Equal(1, len(s))`, implying len(s) > 0, meaning s is nonnil.
	//   Here, actualExpr is `s` and expectedExprValue is `_greaterThanZero`, which translates to the binary expression
	//   `s != nil` being added to the CFG. Similarly, for `Equal(len(s), 0)`, we add `s == nil` to the CFG.
	// - nil comparison. E.g., `Equal(nil, err)`, where actualExpr is `err` and expectedExprValue is `_nil`, which
	//   translates to the binary expression `err == nil` being added to the CFG.
	switch funcName {
	case "Equal", "Equalf", "Empty", "Emptyf": // len(s) == [positive_int], expr == nil
		switch expectedVal {
		case _greaterThanZero:
			return newNilBinaryExpr(actualExpr, token.NEQ)
		case _nil:
			return newNilBinaryExpr(actualExpr, token.EQL)
		case _false:
			return negatedSelfExpr(call, actualExprIndex, nil)
		}
	case "NotEqual", "NotEqualf", "NotEmpty", "NotEmptyf": // len(s) != [zero], expr != nil
		switch expectedVal {
		case _zero, _nil:
			return newNilBinaryExpr(actualExpr, token.NEQ)
		case _false:
			return selfExpr(call, actualExprIndex, nil)
		}

	// Note the check for `actualExprIndex` in the following cases, we need to make sure the slice expr
	// is at the correct position since these are inequality checks.
	case "Greater", "Greaterf": // len(s) > [non_negative_int]
		if actualExprIndex == 0 && (expectedVal == _zero || expectedVal == _greaterThanZero) {
			return newNilBinaryExpr(actualExpr, token.NEQ)
		}
	case "GreaterOrEqual", "GreaterOrEqualf": // len(s) >= [positive_int]
		if actualExprIndex == 0 && expectedVal == _greaterThanZero {
			return newNilBinaryExpr(actualExpr, token.NEQ)
		}
	case "Less", "Lessf": // [non_negative_int] < len(s)
		if actualExprIndex == 1 && (expectedVal == _zero || expectedVal == _greaterThanZero) {
			return newNilBinaryExpr(actualExpr, token.NEQ)
		}
	case "LessOrEqual", "LessOrEqualf": // [positive_int] <= len(s)
		if actualExprIndex == 1 && expectedVal == _greaterThanZero {
			return newNilBinaryExpr(actualExpr, token.NEQ)
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
		funcNameRegex:  regexp.MustCompile(`^(Greater(f)?|Less(f)?|Equal(f)?|GreaterOrEqual(f)?|LessOrEqual(f)?|NotEqual(f)?)$`),
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
		funcNameRegex:  regexp.MustCompile(`^(Greater(f)?|Less(f)?|Equal(f)?|GreaterOrEqual(f)?|LessOrEqual(f)?|NotEqual(f)?)$`),
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
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(assert|require)$`),
		funcNameRegex:  regexp.MustCompile(`^(Empty(f)?|NotEmpty(f)?)$`),
	}: {action: requireZeroComparators, argIndex: 1},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		funcNameRegex:  regexp.MustCompile(`^(Empty(f)?|NotEmpty(f)?)$`),
	}: {action: requireZeroComparators, argIndex: 0},
}
