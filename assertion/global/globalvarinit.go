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

package global

import (
	"go/ast"
	"go/types"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// analyzeValueSpec returns full triggers corresponding to the declaration
func analyzeValueSpec(pass *analysis.Pass, spec *ast.ValueSpec, initFuncDecl *ast.FuncDecl) []annotation.FullTrigger {
	var fullTriggers []annotation.FullTrigger

	consumers := getGlobalConsumers(pass, spec, initFuncDecl)

	for i, ident := range spec.Names {
		if consumers[i] == nil {
			continue
		}

		var prod *annotation.ProduceTrigger
		// Case: variables are not initialized
		// All the variables in this case have same type
		if len(spec.Values) == 0 {
			prod = &annotation.ProduceTrigger{
				Annotation: &annotation.ProduceTriggerTautology{},
				Expr:       ident,
			}
		} else if len(spec.Names) == len(spec.Values) {
			// Case: variables are initialized and the assignment is 1-1
			prod = getGlobalProducer(pass, spec, i, i)
		} else {
			// Case: variables are initialized using a multiple return function
			prod = getGlobalProducer(pass, spec, i, 0)
		}

		if prod != nil {
			fullTriggers = append(fullTriggers,
				annotation.FullTrigger{
					Producer: prod,
					Consumer: consumers[i],
				})
		}
	}

	return fullTriggers
}

// Returns a list of consumers corresponding to a global level variable declaration
func getGlobalConsumers(pass *analysis.Pass, valspec *ast.ValueSpec, initFuncDecl *ast.FuncDecl) []*annotation.ConsumeTrigger {
	consumers := make([]*annotation.ConsumeTrigger, len(valspec.Names))

	for i, name := range valspec.Names {
		// Types that are not nilable are eliminated here
		if !util.TypeBarsNilness(pass.TypesInfo.TypeOf(name)) && !util.IsEmptyExpr(name) && !hasGlobalVarAssignInInitFunc(valspec, initFuncDecl) {
			v := pass.TypesInfo.ObjectOf(name).(*types.Var)
			consumers[i] = &annotation.ConsumeTrigger{
				Annotation: &annotation.GlobalVarAssign{
					TriggerIfNonNil: &annotation.TriggerIfNonNil{
						Ann: &annotation.GlobalVarAnnotationKey{
							VarDecl: v,
						}}},
				Expr:         name,
				Guards:       util.NoGuards(),
				GuardMatched: false,
			}
		}
	}
	return consumers
}

// Checks if all the global variables represented by spec are assigned values within the init function.
// It returns true if all variables are assigned, false otherwise.
// If initFuncDecl is nil, it returns false.
func hasGlobalVarAssignInInitFunc(spec *ast.ValueSpec, initFuncDecl *ast.FuncDecl) bool {
	if initFuncDecl == nil {
		return false
	}
	assignedVars := make(map[string]bool)
	for _, name := range spec.Names {
		assignedVars[name.Name] = false
	}
	ast.Inspect(initFuncDecl.Body, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok {
					if _, exists := assignedVars[ident.Name]; exists {
						assignedVars[ident.Name] = true
					}
				}
			}
		}
		return true
	})

	for _, assigned := range assignedVars {
		if !assigned {
			return false
		}
	}
	return true
}

// Returns a producer in the cases: 1) func call 2) literal nil 3) another global var 4) struct field/method.
// In all other cases, it returns nil.
func getGlobalProducer(pass *analysis.Pass, valspec *ast.ValueSpec, lid int, rid int) *annotation.ProduceTrigger {
	switch rhs := valspec.Values[rid].(type) {
	case *ast.CallExpr:
		if ident, ok := rhs.Fun.(*ast.Ident); ok {
			// We assume builtin functions do not return nil.
			if _, ok := pass.TypesInfo.ObjectOf(ident).(*types.Builtin); ok {
				return nil
			}
			return getProducerForFuncCall(pass, ident, lid, rid, rhs)
		}
		// Method call
		if methCall, ok := rhs.Fun.(*ast.SelectorExpr); ok {
			methName := methCall.Sel
			return getProducerForMethodCall(pass, methName, lid, rid, rhs)
		}
	case *ast.Ident:
		// if rhs is literal nil
		if rhs.Name == "nil" {
			return &annotation.ProduceTrigger{
				Annotation: &annotation.ConstNil{ProduceTriggerTautology: &annotation.ProduceTriggerTautology{}},
				Expr:       rhs,
			}
		}
		// if rhs is another global
		return getProducerForVar(pass, rhs)
	case *ast.SelectorExpr:
		// Struct field access
		return getProducerForField(pass, rhs.Sel)
	}

	return nil
}

func getProducerForVar(pass *analysis.Pass, rhs *ast.Ident) *annotation.ProduceTrigger {
	rhsVar, ok := pass.TypesInfo.ObjectOf(rhs).(*types.Var)
	if !ok || !annotation.VarIsGlobal(rhsVar) {
		// If rhs is not a global variable (e.g., a constant), we ignore it.
		return nil
	}

	return &annotation.ProduceTrigger{
		Annotation: &annotation.GlobalVarRead{
			TriggerIfNilable: &annotation.TriggerIfNilable{
				Ann: &annotation.GlobalVarAnnotationKey{
					VarDecl: rhsVar,
				}}},
		Expr: rhs,
	}
}

func getProducerForField(pass *analysis.Pass, rhs *ast.Ident) *annotation.ProduceTrigger {
	rhsVar, ok := pass.TypesInfo.ObjectOf(rhs).(*types.Var)
	if !ok {
		// If rhs is not a variable (e.g., a constant from an upstream package), we ignore it.
		return nil
	}
	return &annotation.ProduceTrigger{
		Annotation: &annotation.FldRead{
			TriggerIfNilable: &annotation.TriggerIfNilable{
				Ann: &annotation.FieldAnnotationKey{
					FieldDecl: rhsVar,
				}}},
		Expr: rhs,
	}
}

func getProducerForFuncCall(pass *analysis.Pass, methName *ast.Ident, lid int, rid int, rhs ast.Expr) *annotation.ProduceTrigger {
	fdecl, ok := pass.TypesInfo.ObjectOf(methName).(*types.Func)

	// We ignore if the method is anonymous
	if !ok {
		return nil
	}

	// We are interested in `lid-rid`-th return of the function
	// In single return function this is `0` and in multiple return function it is `lid`
	retKey := annotation.RetKeyFromRetNum(fdecl, lid-rid)

	prod := &annotation.ProduceTrigger{
		Annotation: &annotation.FuncReturn{
			TriggerIfNilable: &annotation.TriggerIfNilable{Ann: retKey, NeedsGuard: false},
		},
		Expr: rhs,
	}
	return prod
}

func getProducerForMethodCall(pass *analysis.Pass, methName *ast.Ident, lid int, rid int, rhs ast.Expr) *annotation.ProduceTrigger {
	mdecl, ok := pass.TypesInfo.ObjectOf(methName).(*types.Func)

	// We ignore if the method is anonymous
	if !ok {
		return nil
	}

	// We are interested in `lid-rid`-th return of the function
	// In single return function this is `0` and in multiple return function it is `lid`
	retKey := annotation.RetKeyFromRetNum(mdecl, lid-rid)

	prod := &annotation.ProduceTrigger{
		Annotation: &annotation.MethodReturn{
			TriggerIfNilable: &annotation.TriggerIfNilable{Ann: retKey},
		},
		Expr: rhs,
	}
	return prod
}
