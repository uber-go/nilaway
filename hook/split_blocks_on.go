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

	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/asthelper"
	"go.uber.org/nilaway/util/typeshelper"
)

// SplitBlockOn splits the CFG block on seeing matched trusted functions, where the condition is
// the returned expression. For example, a binary expression `x != nil` is returned for trusted
// function `assert.NotNil(t, x)`, and the CFG block is split as if it were written like
// `if x != nil { <...code after the function call...> }`. This helps NilAway understand the
// nilability of the arguments after certain functions with side effects.
func SplitBlockOn(pass *analysishelper.EnhancedPass, call *ast.CallExpr) ast.Expr {
	for sig, act := range _splitBlockOn {
		if sig.matchCall(pass, call) {
			return act.action(pass, call, act.argIndex)
		}
	}
	return nil
}

// splitBlockOnAction defines the effect the trusted function can have on its argument `argIndex`.
type splitBlockOnAction func(pass *analysishelper.EnhancedPass, call *ast.CallExpr, argIndex int) ast.Expr

// nilBinaryExpr returns `expr == nil`. This is useful, for example, in asserting nilability of an object in the `testify` library: `assert.Nil(t, obj)`, which gets interpreted as `if obj == nil {...}` by preprocess
var nilBinaryExpr splitBlockOnAction = func(_ *analysishelper.EnhancedPass, call *ast.CallExpr, argIndex int) ast.Expr {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return newNilBinaryExpr(call.Args[argIndex], token.EQL)
}

// nonnilBinaryExpr returns `expr != nil`. This is useful, for example, in asserting non-nilability of an object in the `testify` library: `assert.NotNil(t, obj)`, which gets interpreted as `if obj != nil {...}` by preprocess
var nonnilBinaryExpr splitBlockOnAction = func(_ *analysishelper.EnhancedPass, call *ast.CallExpr, argIndex int) ast.Expr {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return newNilBinaryExpr(call.Args[argIndex], token.NEQ)
}

// selfExpr returns the expression itself. Currently, this is meant for only checking boolean expressions, implying `if expr {...}`, i.e., `if expr == true {...}`.
// This is useful, for example, is asserting a boolean true value in the `testify` library: `assert.True(t, ok)`, which gets interpreted as `if ok {...}` by preprocess
var selfExpr splitBlockOnAction = func(_ *analysishelper.EnhancedPass, call *ast.CallExpr, argIndex int) ast.Expr {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	return call.Args[argIndex]
}

// negatedSelfExpr is same as selfExpr, but returns a negated expr. E.g., `assert.False(t, ok)`, which gets interpreted as `if !ok {...}` by preprocess
var negatedSelfExpr splitBlockOnAction = func(_ *analysishelper.EnhancedPass, call *ast.CallExpr, argIndex int) ast.Expr {
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

// boolOrErrorExpr handles assertion arguments declared as `interface{}`, e.g., gotest.tools'
// `assert.Assert(t, comparison BoolOrComparison)`, whose argument may be:
//   - a boolean expression, e.g., `assert.Assert(t, x != nil)`: behaves like selfExpr;
//   - an error value, e.g., `assert.Assert(t, err)`: behaves like nilBinaryExpr, since a nil error
//     means success while a non-nil error fails the assertion. Only interface types (`error`
//     itself or interfaces embedding it) qualify: a nil value of such a type stays nil when passed
//     as `interface{}`, whereas a concrete error type would be wrapped in a non-nil interface and
//     always fail the assertion (even a typed-nil pointer);
//   - anything else (e.g., a `cmp.Comparison` closure): no narrowing is applied.
var boolOrErrorExpr splitBlockOnAction = func(pass *analysishelper.EnhancedPass, call *ast.CallExpr, argIndex int) ast.Expr {
	if argIndex < 0 || argIndex >= len(call.Args) {
		return nil
	}
	if isBoolExpr(pass, call.Args[argIndex]) {
		return selfExpr(pass, call, argIndex)
	}
	t := pass.TypesInfo.TypeOf(call.Args[argIndex])
	if t == nil {
		return nil
	}
	if _, ok := t.Underlying().(*types.Interface); ok && typeshelper.ImplementsError(t) {
		return nilBinaryExpr(pass, call, argIndex)
	}
	return nil
}

// _goconveyAssertions matches the package paths where goconvey's `Should*` assertions are
// defined: the `convey` package itself (which re-exports them as package-level variables, e.g.,
// `var ShouldBeNil = assertions.ShouldBeNil`) and the underlying assertions package (both its
// current `smarty` and historical `smartystreets` homes, for users importing it directly).
var _goconveyAssertions = regexp.MustCompile(`^(stubs/)?(github\.com/smartystreets/goconvey/convey|github\.com/smarty(streets)?/assertions)$`)

// goconveySoExpr handles goconvey's `So(actual, assertion, expected...)`, where the narrowing fact is
// determined by the assertion argument rather than the called function: e.g.,
// `So(err, ShouldBeNil)` implies `err == nil` afterwards. The assertion argument is resolved to
// its package-level object (a var re-exported by `convey`, or a function of the assertions
// package), and only the nilability-relevant assertions are modeled; any other assertion (or a
// locally-defined custom one) yields no narrowing.
var goconveySoExpr splitBlockOnAction = func(pass *analysishelper.EnhancedPass, call *ast.CallExpr, argIndex int) ast.Expr {
	// The assertion argument sits right after the actual expression.
	if argIndex+1 >= len(call.Args) {
		return nil
	}
	var ident *ast.Ident
	switch assert := call.Args[argIndex+1].(type) {
	case *ast.Ident:
		ident = assert
	case *ast.SelectorExpr:
		ident = assert.Sel
	default:
		return nil
	}
	obj := pass.TypesInfo.ObjectOf(ident)
	if obj == nil || obj.Pkg() == nil || !_goconveyAssertions.MatchString(obj.Pkg().Path()) {
		return nil
	}

	// For the boolean assertions, the actual argument is declared `interface{}`; only narrow
	// when it is statically a boolean expression (anything else fails the assertion at runtime
	// anyway).
	switch obj.Name() {
	case "ShouldBeNil":
		return nilBinaryExpr(pass, call, argIndex)
	case "ShouldNotBeNil", "ShouldBeError":
		return nonnilBinaryExpr(pass, call, argIndex)
	case "ShouldBeTrue":
		if isBoolExpr(pass, call.Args[argIndex]) {
			return selfExpr(pass, call, argIndex)
		}
	case "ShouldBeFalse":
		if isBoolExpr(pass, call.Args[argIndex]) {
			return negatedSelfExpr(pass, call, argIndex)
		}
	}
	return nil
}

// isBoolExpr reports whether the expression is statically of boolean type.
func isBoolExpr(pass *analysishelper.EnhancedPass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	basic, ok := t.Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Bool
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
var requireComparators splitBlockOnAction = func(pass *analysishelper.EnhancedPass, call *ast.CallExpr, startIndex int) ast.Expr {
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
			if pass.TypesInfo.ObjectOf(wrapperFunc) != typeshelper.BuiltinLen || len(expr.Args) != 1 {
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
var requireZeroComparators splitBlockOnAction = func(pass *analysishelper.EnhancedPass, call *ast.CallExpr, index int) ast.Expr {
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
	ident := asthelper.FuncIdentFromCallExpr(call)
	if ident == nil {
		return nil
	}
	funcName := ident.Name

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
var requireLen splitBlockOnAction = func(pass *analysishelper.EnhancedPass, call *ast.CallExpr, startIndex int) ast.Expr {
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
var _splitBlockOn = map[trustedSig]struct {
	action   splitBlockOnAction
	argIndex int
}{
	// `suite.Suite` and `assert.Assertions`
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		nameRegex:      regexp.MustCompile(`^(Nil(f)?|NoError(f)?)$`),
	}: {action: nilBinaryExpr, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		nameRegex:      regexp.MustCompile(`^(NotNil(f)?|Error(f)?|ErrorContains(f)?|EqualError(f)?)$`),
	}: {action: nonnilBinaryExpr, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		nameRegex:      regexp.MustCompile(`^True(f)?$`),
	}: {action: selfExpr, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		nameRegex:      regexp.MustCompile(`^False(f)?$`),
	}: {action: negatedSelfExpr, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		nameRegex:      regexp.MustCompile(`^(Greater(f)?|Less(f)?|Equal(f)?|GreaterOrEqual(f)?|LessOrEqual(f)?|NotEqual(f)?)$`),
	}: {action: requireComparators, argIndex: 0},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		nameRegex:      regexp.MustCompile(`^Len(f)?$`),
	}: {action: requireLen, argIndex: 0},

	// `assert` and `require`
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(assert|require)$`),
		nameRegex:      regexp.MustCompile(`^(Nil(f)?|NoError(f)?)$`),
	}: {action: nilBinaryExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(assert|require)$`),
		nameRegex:      regexp.MustCompile(`^(NotNil(f)?|Error(f)?|ErrorContains(f)?|EqualError(f)?)$`),
	}: {action: nonnilBinaryExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(assert|require)$`),
		nameRegex:      regexp.MustCompile(`^True(f)?$`),
	}: {action: selfExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(assert|require)$`),
		nameRegex:      regexp.MustCompile(`^False(f)?$`),
	}: {action: negatedSelfExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(assert|require)$`),
		nameRegex:      regexp.MustCompile(`^(Greater(f)?|Less(f)?|Equal(f)?|GreaterOrEqual(f)?|LessOrEqual(f)?|NotEqual(f)?)$`),
	}: {action: requireComparators, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(assert|require)$`),
		nameRegex:      regexp.MustCompile(`^Len(f)?$`),
	}: {action: requireLen, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(assert|require)$`),
		nameRegex:      regexp.MustCompile(`^(Empty(f)?|NotEmpty(f)?)$`),
	}: {action: requireZeroComparators, argIndex: 1},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/stretchr/testify/(suite\.Suite|assert\.Assertions|require\.Assertions)$`),
		nameRegex:      regexp.MustCompile(`^(Empty(f)?|NotEmpty(f)?)$`),
	}: {action: requireZeroComparators, argIndex: 0},

	// `gotest.tools/v3/assert`, as well as its legacy v1/v2 form `gotest.tools/assert` with
	// identical semantics. Note that `ErrorIs` is deliberately NOT modeled with nonnil narrowing:
	// `errors.Is(nil, nil)` is true, so `assert.ErrorIs(t, err, nil)` can pass with a nil error.
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?gotest\.tools(/v3)?/assert$`),
		nameRegex:      regexp.MustCompile(`^NilError$`),
	}: {action: nilBinaryExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?gotest\.tools(/v3)?/assert$`),
		nameRegex:      regexp.MustCompile(`^(Error|ErrorContains)$`),
	}: {action: nonnilBinaryExpr, argIndex: 1},
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?gotest\.tools(/v3)?/assert$`),
		nameRegex:      regexp.MustCompile(`^Assert$`),
	}: {action: boolOrErrorExpr, argIndex: 1},

	// `github.com/smartystreets/goconvey/convey`, which is typically dot-imported. Under the
	// default `FailureHalts` mode, a failed `So` panics and is recovered by the `Convey` runner,
	// halting the enclosing scope; the opt-in `FailureContinues` mode is over-approximated the
	// same way as testify's non-fatal `assert` (see the comment on this table).
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?github\.com/smartystreets/goconvey/convey$`),
		nameRegex:      regexp.MustCompile(`^So$`),
	}: {action: goconveySoExpr, argIndex: 0},
}
