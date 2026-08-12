// Package typeswitchclosure contains a function literal with a type-switch guard
// variable to test that collectClosure does not panic on its references.
package typeswitchclosure

import "go/ast"

// callSite mimics the goconveySoExpr pattern: a function literal assigned to a
// package-level variable, containing a type switch whose guard variable is
// referenced in each case body.
var callSite = func(call *ast.CallExpr, argIndex int) *ast.Ident {
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
	return ident
}
