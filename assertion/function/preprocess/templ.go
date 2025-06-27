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

func insertSyntheticCall(block *cfg.Block, returnIndex int) {
	returnStmt := block.Nodes[returnIndex].(*ast.ReturnStmt)
	generatedTemplateCall := returnStmt.Results[0].(*ast.CallExpr)

	// Extract the function literal that is the first argument to templruntime.GeneratedTemplate
	funcLit := generatedTemplateCall.Args[0]

	// // Look for existing templruntime.GeneratedComponentInput type reference to reuse
	// var inputType ast.Expr
	// if funcLitExpr, ok := funcLit.(*ast.FuncLit); ok &&
	// 	funcLitExpr.Type != nil &&
	// 	funcLitExpr.Type.Params != nil &&
	// 	len(funcLitExpr.Type.Params.List) > 0 {
	// 	// Reuse the parameter type from the function literal
	// 	inputType = funcLitExpr.Type.Params.List[0].Type
	// } else {
	// 	// Fallback: create new type reference
	// 	inputType = &ast.SelectorExpr{
	// 		X:   &ast.Ident{Name: "templruntime"},
	// 		Sel: &ast.Ident{Name: "GeneratedComponentInput"},
	// 	}
	// }
	//
	// // Create a synthetic call to this function literal with empty GeneratedComponentInput
	// syntheticCall := &ast.CallExpr{
	// 	Fun:    funcLit,
	// 	Lparen: generatedTemplateCall.Lparen, // Reuse position
	// 	Args: []ast.Expr{
	// 		&ast.CompositeLit{
	// 			Type: inputType,
	// 		},
	// 	},
	// 	Rparen: generatedTemplateCall.Rparen, // Reuse position
	// }
	//
	// _ = &ast.ExprStmt{X: syntheticCall}

	newNodes := make([]ast.Node, len(block.Nodes)+1)
	copy(newNodes[:returnIndex], block.Nodes[:returnIndex])
	if lit, ok := funcLit.(*ast.FuncLit); ok {
		// If the function literal is a FuncLit, we need to insert its body as a new node
		inlinedNodes := make([]ast.Node, 0, len(lit.Body.List))
		for _, stmt := range lit.Body.List {
			if _, ok := stmt.(*ast.ReturnStmt); ok {
				continue
			}
			inlinedNodes = append(inlinedNodes, stmt)
		}
		newNodes = append(newNodes[:returnIndex], inlinedNodes...)
		newNodes = append(newNodes, returnStmt)
	} else {
		panic("Expected a function literal in the return statement of a templ component function")
		// Otherwise, we just insert the function literal as is
		newNodes[returnIndex] = funcLit
	}
	// newNodes[returnIndex] = funcLit.(*ast.FuncLit).Body
	// copy(newNodes[returnIndex+1:], block.Nodes[returnIndex:])
	block.Nodes = newNodes
}

// func inlineFuncLitBody(block *cfg.Block, funcLit *ast.FuncLit) {
// 	// Inline the body of the function literal into the block
// 	if funcLit.Body == nil {
// 		return
// 	}
//
// 	newNodes := make([]ast.Node, 0, len(block.Nodes)+len(funcLit.Body.List))
// 	newNodes = append(newNodes, block.Nodes[:len(block.Nodes)-1]...) // Keep all but the last node (the return statement)
// 	inlinedNodes := make([]ast.Node, 0, len(funcLit.Body.List))
// 	for _, stmt := range funcLit.Body.List {
// 		if _, ok := stmt.(*ast.ReturnStmt); ok {
// 			continue
// 		}
// 		inlinedNodes = append(inlinedNodes, stmt)
// 	}
// 	newNodes = append(newNodes, inlinedNodes...)                 // Add the function body statements
// 	newNodes = append(newNodes, block.Nodes[len(block.Nodes)-1]) // Add the return statement back
//
// 	block.Nodes = newNodes
// }
