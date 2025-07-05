package analysishelper

import (
	"go/ast"
	"go/constant"

	"golang.org/x/tools/go/analysis"
)

// EnhancedPass is a drop-in replacement for `*analysis.Pass` that provides additional helper methods
// to make it easier to work with the analysis pass.
type EnhancedPass struct {
	*analysis.Pass
}

// NewEnhancedPass creates a new EnhancedPass from the given *analysis.Pass.
func NewEnhancedPass(pass *analysis.Pass) *EnhancedPass {
	return &EnhancedPass{Pass: pass}
}

// IsZero returns if the given expression is evaluated to integer zero at compile time. For
// example, zero literal, zero const or binary expression that evaluates to zero, e.g., 1 - 1
// should all return true. Note the function will return false for zero string `"0"`.
func (p *EnhancedPass) IsZero(expr ast.Expr) bool {
	tv, ok := p.TypesInfo.Types[expr]
	if !ok {
		return false
	}
	intValue, ok := constant.Val(tv.Value).(int64)
	return ok && intValue == 0
}
