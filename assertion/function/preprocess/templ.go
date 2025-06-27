package preprocess

import (
	"go/ast"
	"go/types"

	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis/passes/ctrlflow"
	"golang.org/x/tools/go/cfg"
)

// insertSyntheticCallForTemplComponent inserts a synthetic call to the function literal
// that is the first argument to templruntime.GeneratedTemplate in the return statement of
// a templ component function. Conceptually:
//
// return templruntime.GeneratedTemplate(func(... templruntime.GeneratedComponentInput) (... error) { ... })
//
// into:
//
// func(... templruntime.GeneratedComponentInput) (... error) { ... }(templruntime.GeneratedComponentInput{}) // synthetic call
// return templruntime.GeneratedTemplate(func(... templruntime.GeneratedComponentInput) (... error) { ... })  // original return
//
// This is done to ensure that the function literal's invocation is explicit to aid NilAway's analysis.
func (p *Preprocessor) insertSyntheticCallForTemplComponent(graph *cfg.CFG, funcDecl *ast.FuncDecl) {
	if !p.isTemplComponentFunction(funcDecl) {
		return
	}
	cfgs := p.pass.ResultOf[ctrlflow.Analyzer].(*ctrlflow.CFGs)

	for _, block := range graph.Blocks {
		if !block.Live || len(block.Nodes) == 0 {
			continue
		}

		for _, node := range block.Nodes {
			returnStmt, ok := node.(*ast.ReturnStmt)
			if !ok {
				continue
			}
			if p.isGeneratedTemplateReturn(returnStmt) {
				funcLit := returnStmt.Results[0].(*ast.CallExpr).Args[0].(*ast.FuncLit)

				graph.Blocks = cfgs.FuncLit(funcLit).Blocks
				for _, b := range graph.Blocks {
					if !b.Live || len(b.Nodes) == 0 {
						continue
					}

					// Remove all return statements from this block
					filteredNodes := make([]ast.Node, 0, len(b.Nodes))
					for _, node := range b.Nodes {
						if _, ok := node.(*ast.ReturnStmt); !ok {
							filteredNodes = append(filteredNodes, node)
						} else {
							filteredNodes = append(filteredNodes, returnStmt)
						}
					}
					b.Nodes = filteredNodes
				}
				// inlineFuncLitBody(block, returnStmt.Results[0].(*ast.CallExpr).Args[0].(*ast.FuncLit))
				// insertSyntheticCall(block, i)
				return
			}
		}
	}
}

func (p *Preprocessor) isTemplComponentFunction(funcDecl *ast.FuncDecl) bool {
	// Check if the function returns a single result of type `templ.Component`.
	if funcDecl == nil || funcDecl.Type == nil || funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) != 1 {
		return false
	}
	named, ok := p.pass.TypesInfo.TypeOf(funcDecl.Type.Results.List[0].Type).(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	return obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == config.TemplPkgPath && obj.Name() == "Component"
}

func (p *Preprocessor) isGeneratedTemplateReturn(returnStmt *ast.ReturnStmt) bool {
	// Check if it is "templruntime.GeneratedTemplate(...)" call.
	if len(returnStmt.Results) != 1 {
		return false
	}
	callExpr, ok := returnStmt.Results[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	funObj := p.pass.TypesInfo.ObjectOf(sel.Sel)
	return funObj != nil && funObj.Pkg().Path() == config.TemplRuntimePkgPath && funObj.Name() == "GeneratedTemplate"
}
