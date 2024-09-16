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
	"go/constant"
	"go/token"
	"go/types"
	"regexp"

	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// SplitBlockOn splits the CFG block on seeing matched trusted functions, where the condition is
// the returned expression. For example, a binary expression `x != nil` is returned for trusted
// function `assert.NotNil(t, x)`, and the CFG block is split as if it were written like
// `if x != nil { <...code after the function call...> }`. This helps NilAway understand the
// nilability of the arguments after certain functions with side effects.
func SplitBlockOn(pass *analysis.Pass, call *ast.CallExpr) ast.Expr {
	for sig, act := range _splitBlockOn {
		if sig.match(pass, call) {
			return act.action(pass, call, act.argIndex)
		}
	}
	return nil
}

// splitBlockOnAction defines the effect the trusted function can have on its argument `argIndex`.
type splitBlockOnAction func(pass *analysis.Pass, call *ast.CallExpr, argIndex int) ast.Expr

// nilBinaryExpr returns `expr == nil`. This is useful, for example, in asserting nilability of an object in the `testify` library: `assert.Nil(t, obj)`, which gets interpreted as `if obj == nil {...}` by preprocess
var nilBinaryExpr splitBlockOnAction = func(_ *analysis.Pass, call *ast.CallExpr, argIndex int) ast.Expr {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return newNilBinaryExpr(call.Args[argIndex], token.EQL)
}

// nonnilBinaryExpr returns `expr != nil`. This is useful, for example, in asserting non-nilability of an object in the `testify` library: `assert.NotNil(t, obj)`, which gets interpreted as `if obj != nil {...}` by preprocess
var nonnilBinaryExpr splitBlockOnAction = func(_ *analysis.Pass, call *ast.CallExpr, argIndex int) ast.Expr {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return newNilBinaryExpr(call.Args[argIndex], token.NEQ)
}

// selfExpr returns the expression itself. Currently, this is meant for only checking boolean expressions, implying `if expr {...}`, i.e., `if expr == true {...}`.
// This is useful, for example, is asserting a boolean true value in the `testify` library: `assert.True(t, ok)`, which gets interpreted as `if ok {...}` by preprocess
var selfExpr splitBlockOnAction = func(_ *analysis.Pass, call *ast.CallExpr, argIndex int) ast.Expr {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return call.Args[argIndex]
}

// negatedSelfExpr is same as selfExpr, but returns a negated expr. E.g., `assert.False(t, ok)`, which gets interpreted as `if !ok {...}` by preprocess
var negatedSelfExpr splitBlockOnAction = func(_ *analysis.Pass, call *ast.CallExpr, argIndex int) ast.Expr {
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
var requireComparators splitBlockOnAction = func(pass *analysis.Pass, call *ast.CallExpr, startIndex int) ast.Expr {
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
var requireZeroComparators splitBlockOnAction = func(pass *analysis.Pass, call *ast.CallExpr, index int) ast.Expr {
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
func generateComparators(call *ast.CallExpr, actualExpr ast.Expr, actualExprIndex int, expectedVal expectedValue) ast.Expr {
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
			return negatedSelfExpr(nil, call, actualExprIndex)
		}
	case "NotEqual", "NotEqualf", "NotEmpty", "NotEmptyf": // len(s) != [zero], expr != nil
		switch expectedVal {
		case _zero, _nil:
			return newNilBinaryExpr(actualExpr, token.NEQ)
		case _false:
			return selfExpr(nil, call, actualExprIndex)
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
var requireLen splitBlockOnAction = func(pass *analysis.Pass, call *ast.CallExpr, startIndex int) ast.Expr {
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

// _splitBlockOn defines the map of trusted functions and their corresponding actions on a
// particular argument.
var _splitBlockOn = map[trustedFuncSig]struct {
	action   splitBlockOnAction
	argIndex int
}{
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
