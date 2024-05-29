package preprocess

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/cfg"
)

func TestPreprocessor_FixNoReturnBlock(t *testing.T) {
	t.Parallel()

	const code = `
import "log"

func foo(a bool) {
  if a {
    log.Fatal("fatal") 
  }
}
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", "", parser.ParseComments|parser.ParseComments)
	require.NoError(t, err)
	var funcDecl *ast.FuncDecl
	for _, decl := range f.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok {
			funcDecl = d
		}
	}
	require.NotNil(t, funcDecl)

	graph := cfg.New(funcDecl.Body, nil /* mayReturn */)
	require.NotEmpty(t, graph)
}
