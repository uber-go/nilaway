package hook

import (
	"go/ast"
	"go/token"
	"regexp"

	"golang.org/x/tools/go/analysis"
)

// ReplaceConditional replaces a call to a matched function with the returned expression. This is
// useful for modeling stdlib and 3rd party functions that return a single boolean value, which
// implies nilability of the arguments. For example, `errors.As(err, &target)` implies
// `target != nil`, so it can be replaced with `target != nil`.
//
// If the call does not match any known function, nil is returned.
func ReplaceConditional(pass *analysis.Pass, call *ast.CallExpr) ast.Expr {
	for sig, act := range _replaceConditionals {
		if sig.match(pass, call) {
			return act(call, pass)
		}
	}
	return nil
}

type replaceConditionalAction func(call *ast.CallExpr, p *analysis.Pass) ast.Expr

// _errorAsAction replaces a call to `errors.As(err, &target)` with the expression `target != nil`.
var _errorAsAction replaceConditionalAction = func(call *ast.CallExpr, p *analysis.Pass) ast.Expr {
	if len(call.Args) != 2 {
		return nil
	}
	unaryExpr, ok := call.Args[1].(*ast.UnaryExpr)
	if !ok {
		return nil
	}
	if unaryExpr.Op != token.AND {
		return nil
	}
	return newNilBinaryExpr(unaryExpr.X, token.NEQ)
}

var _replaceConditionals = map[trustedFuncSig]replaceConditionalAction{
	{
		kind:           _func,
		enclosingRegex: regexp.MustCompile(`^errors$`),
		funcNameRegex:  regexp.MustCompile(`^As$`),
	}: _errorAsAction,
}
