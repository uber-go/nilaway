package hook

import (
	"go/ast"
	"regexp"
	"slices"

	"golang.org/x/tools/go/analysis"
)

func TerminatingCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	return slices.ContainsFunc(_terminatingCalls, func(sig trustedFuncSig) bool { return sig.match(pass, call) })
}

var _terminatingCalls = []trustedFuncSig{
	// `zap.Fatal` / `zap.Fatalf` / `zap.Fatalln`
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?go\.uber\.org/zap.Logger$`),
		funcNameRegex:  regexp.MustCompile(`^Fatal$`),
	},
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?go\.uber\.org/zap.SugaredLogger$`),
		funcNameRegex:  regexp.MustCompile(`^Fatal(f|ln|w)?$`),
	},
}
