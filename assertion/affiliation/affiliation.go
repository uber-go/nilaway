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

// Package affiliation implements the affliation analyzer that tries to find the concrete
// implementation of an interface and create full triggers for them.
package affiliation

import (
	"go/ast"
	"go/types"
	"strings"

	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// Affiliation is used to track the association between an interface and its concrete implementations in the form of a map,
// where the key is a function where the affiliation was witnessed and value is an array of full triggers computed for
// the affiliation key
type Affiliation struct {
	conf     *config.Config
	triggers []annotation.FullTrigger
}

// Pair is a struct to store struct-interface affiliation pairs
type Pair struct {
	ImplementedID string
	DeclaredID    string
}

// ImplementedDeclaredTypesCache declares a map of a concrete implementation (e.g., struct) and its implemented interfaces
// in the form of their fully qualified paths (key: <structFQ>#<interfaceFQ>, value: true/false)
type ImplementedDeclaredTypesCache map[Pair]bool

// AffliliationCache stores the mapping between interfaces and their implementations that have been analyzed. This
// information can be used by downstream packages to avoid re-analysis of the same affiliations
type AffliliationCache struct {
	Cache ImplementedDeclaredTypesCache
}

// AFact enables use of the facts passing mechanism in Go's analysis framework
func (*AffliliationCache) AFact() {}

// extractAffiliations processes all affiliations (e.g., interface and its implementing struct) and returns map documenting
// the affiliations
func (a *Affiliation) extractAffiliations(pass *analysis.Pass) {
	// initialize
	upstreamCache := make(ImplementedDeclaredTypesCache, 0) // store entries passed from upstream packages
	currentCache := make(ImplementedDeclaredTypesCache, 0)  // store new entries witnessed in this current package

	// populate upstreamCache by importing entries passed from upstream packages
	facts := pass.AllPackageFacts()
	if len(facts) > 0 {
		for _, f := range facts {
			switch c := f.Fact.(type) {
			case *AffliliationCache:
				for k, v := range c.Cache {
					upstreamCache[k] = v
				}
			}
		}
	}

	a.computeTriggersForCastingSites(pass, upstreamCache, currentCache)

	// export upstreamCache from this package by adding new entries (if any)
	if len(currentCache) > 0 {
		pass.ExportPackageFact(&AffliliationCache{
			Cache: currentCache,
		})
	}
}

// computeTriggersForCastingSites analyzes all explicit and implicit sites of casts in the AST. For example, explicit casts,
// variable assignments, variable declaration and initialization, method returns, and method parameters.
func (a *Affiliation) computeTriggersForCastingSites(pass *analysis.Pass, upstreamCache ImplementedDeclaredTypesCache, currentCache ImplementedDeclaredTypesCache) {
	appendTypeToTypeTriggers := func(lhsType, rhsType types.Type) {
		a.triggers = append(a.triggers, a.computeTriggersForTypes(lhsType, rhsType, upstreamCache, currentCache)...)
	}

	for _, file := range pass.Files {
		if !a.conf.IsFileInScope(file) {
			continue
		}

		// identify sites of explicit or implicit casts
		for _, decl := range file.Decls {
			f, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			ast.Inspect(f, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.AssignStmt:
					// special case of n-to-1 assignment from a function with multiple returns: e.g., i1, i2 = foo(), where foo() return s1, s2
					// note that other n-to-1 assignments (e.g. v, ok := m[k]) are handled by the loop below, since only the first LHS element is
					// being directly assigned to in a way we care about
					if len(node.Rhs) == 1 && len(node.Lhs) > 1 {
						if rhsSig, ok := pass.TypesInfo.TypeOf(node.Rhs[0]).(*types.Tuple); ok && rhsSig.Len() == len(node.Lhs) {
							for i := range node.Lhs {
								lhsType := pass.TypesInfo.TypeOf(node.Lhs[i])
								rhsType := rhsSig.At(i).Type()
								appendTypeToTypeTriggers(lhsType, rhsType)
							}
							return true
						}
					}
					// e.g., var i I, var s *S, i = s, or more generally, i1, i2, i3 = s1, s2, s3
					for i := 0; i < len(node.Lhs) && i < len(node.Rhs); i++ {
						lhsType := pass.TypesInfo.TypeOf(node.Lhs[i])
						rhsType := pass.TypesInfo.TypeOf(node.Rhs[i])
						appendTypeToTypeTriggers(lhsType, rhsType)
					}
				case *ast.ValueSpec:
					// e.g., var i I = &S{}
					for i := 0; i < len(node.Values); i++ {
						lhsType := pass.TypesInfo.TypeOf(node.Type)
						rhsType := pass.TypesInfo.TypeOf(node.Values[i])
						appendTypeToTypeTriggers(lhsType, rhsType)
					}
				case *ast.CallExpr:
					// e.g., func foo(i I), foo(&S{})
					if ident := util.FuncIdentFromCallExpr(node); ident != nil {
						if declObj := pass.TypesInfo.Uses[ident]; declObj != nil {
							if fdecl, ok := declObj.(*types.Func); ok {
								fsig := fdecl.Type().(*types.Signature)
								for i := 0; i < fsig.Params().Len() && i < len(node.Args); i++ {
									lhsType := fsig.Params().At(i).Type()          // receiver param of method declaration
									rhsType := pass.TypesInfo.TypeOf(node.Args[i]) // caller param
									appendTypeToTypeTriggers(lhsType, rhsType)
								}
							}
						}
					}

					// slice is declared to be of interface type, and append function is used to add struct
					if sliceType, ok := util.IsSliceAppendCall(node, pass); ok {
						for i := 1; i < len(node.Args); i++ {
							lhsType := sliceType.Elem()
							rhsType := pass.TypesInfo.TypeOf(node.Args[i])
							appendTypeToTypeTriggers(lhsType, rhsType)
						}
					}

				case *ast.TypeAssertExpr:
					// e.g., v, ok := i.(*S)
					lhsType := pass.TypesInfo.TypeOf(node.X)
					rhsType := pass.TypesInfo.TypeOf(node.Type)
					appendTypeToTypeTriggers(lhsType, rhsType)

				case *ast.ReturnStmt:
					// function signature states interface return, but the actual return is a struct
					// e.g., m(x *A) I { return x }
					var results = f.Type.Results
					if results != nil {
						funcSigResultsList := results.List
						for i := range node.Results {
							if i < len(funcSigResultsList) {
								lhsType := pass.TypesInfo.TypeOf(funcSigResultsList[i].Type)
								rhsType := pass.TypesInfo.TypeOf(node.Results[i])
								appendTypeToTypeTriggers(lhsType, rhsType)
							}
						}
					}

				case *ast.CompositeLit:
					switch nodeType := node.Type.(type) {
					case *ast.ArrayType:
						// A slice (or array) declared of type interface, and initialized with a struct
						// e.g., _ = []I{&S{}}
						// TODO: currently, nested composite literal for ArrayType is not supported (e.g., _ = [][]I{{&A1{}}}).
						//  Tracked in issue #46.
						lhsType := pass.TypesInfo.TypeOf(nodeType.Elt)
						for _, elt := range node.Elts {
							appendTypeToTypeTriggers(lhsType, pass.TypesInfo.TypeOf(elt))
						}
					case *ast.MapType:
						// Key, value, or both of a map declared of type interface, and initialized with a struct
						// e.g., _ = map[int]I{0: &S{}}
						keyType := pass.TypesInfo.TypeOf(nodeType.Key)
						valueType := pass.TypesInfo.TypeOf(nodeType.Value)
						for _, elt := range node.Elts {
							if kv, ok := elt.(*ast.KeyValueExpr); ok {
								appendTypeToTypeTriggers(keyType, pass.TypesInfo.TypeOf(kv.Key))
								appendTypeToTypeTriggers(valueType, pass.TypesInfo.TypeOf(kv.Value))
							}
						}
					case *ast.Ident:
						// A struct field (embedded or explicit) declared of type interface, and initialized with a struct
						// e.g., var i I = S{t:&T{}}, where `type S struct { t J }`. (Here I and J are interfaces,
						// and S and T are structs implementing them, respectively.)
						// Similarly, embedding is also supported. E.g., var i I = &S{&T{}}, where `type S struct { J }`.
						for i, elt := range node.Elts {
							var lhsType, rhsType types.Type
							if kv, ok := elt.(*ast.KeyValueExpr); ok {
								// In this case the initialization is key-value based. E.g. s = &S{t: &T{}}
								lhsType = pass.TypesInfo.TypeOf(kv.Key)
								rhsType = pass.TypesInfo.TypeOf(kv.Value)
							} else {
								// In this case the initialization is serial. E.g. s = &S{&T{}}
								if sObj := util.TypeAsDeeplyStruct(pass.TypesInfo.TypeOf(node)); sObj != nil {
									lhsType = sObj.Field(i).Type()
									rhsType = pass.TypesInfo.TypeOf(elt)
								}
							}
							if lhsType != nil && rhsType != nil {
								appendTypeToTypeTriggers(lhsType, rhsType)
							}
						}
					}
				case *ast.FuncLit:
					// TODO: Nilability analysis support for anonymous functions is currently not
					//       implemented (tracked in issue #52), so here we completely skip
					//       the affiliation analysis for them.
					return false
				}
				return true
			})
		}
	}
}

// computeTriggersForTypes finds corresponding concrete implementation and their declared methods and populates them in a map
func (a *Affiliation) computeTriggersForTypes(lhsType types.Type, rhsType types.Type, upstreamCache ImplementedDeclaredTypesCache, currentCache ImplementedDeclaredTypesCache) []annotation.FullTrigger {
	if lhsType == nil || rhsType == nil {
		return nil
	}

	lhsObj, ok := lhsType.Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	rhsObj, ok := util.UnwrapPtr(rhsType).(*types.Named)
	if !ok {
		return nil
	}

	// Don't process if the affiliation is already analyzed in upstream packages' upstreamCache or
	// the current package's upstreamCache.
	key := computeAfflitiationCacheKey(lhsObj, rhsObj)
	if upstreamCache[key] {
		return nil
	}
	if currentCache[key] {
		return nil
	}
	// Add unvisited entry.
	currentCache[key] = true

	var triggers []annotation.FullTrigger
	// for each method declared in the interface, find its corresponding concrete implementation
	for i := 0; i < lhsObj.NumMethods(); i++ {
		interfaceMethod := lhsObj.Method(i)
		implementedMethodObj, _, _ := types.LookupFieldOrMethod(rhsType, false, rhsObj.Obj().Pkg(), interfaceMethod.Name())
		if implementedMethodObj == nil || !a.conf.IsPkgInScope(interfaceMethod.Pkg()) || !a.conf.IsPkgInScope(implementedMethodObj.Pkg()) {
			continue
		}
		if implementedMethod, ok := implementedMethodObj.(*types.Func); ok {
			triggers = append(triggers, createFunctionTriggers(implementedMethod, interfaceMethod)...)
		}
	}
	return triggers
}

func getFullyQualifiedName(t types.Type) string {
	s := ""
	switch n := t.(type) {
	case *types.Named:
		s = n.String()
	case *types.Interface:
		// interface has no exported field/method that can be used to get its fully qualified path directly. However,
		// its declared methods (*types.Func) have such exported methods. Therefore, the below logic extracts the
		// interface's fully qualified path from its method's FullName()
		if n.NumMethods() > 0 {
			s = n.Method(0).FullName()
			// funcName.FullName() returns a string of the form "(/path/to/interface).funcName". The below code strips
			// off the method name and parentheses to get only "/path/to/interface"
			i := strings.LastIndex(s, ".")
			if i > -1 {
				s = s[:i]
			}
			s = strings.ReplaceAll(s, "(", "")
			s = strings.ReplaceAll(s, ")", "")
		}
	}
	return s
}

func computeAfflitiationCacheKey(interfaceObj *types.Interface, concreteObj *types.Named) Pair {
	interfaceObjFQ := getFullyQualifiedName(interfaceObj)
	concreteObjFQ := getFullyQualifiedName(concreteObj)
	return Pair{
		ImplementedID: concreteObjFQ,
		DeclaredID:    interfaceObjFQ,
	}
}

// createFunctionTriggers verifies the nilability annotations of the concrete implementation of a method
// against its interface declaration for covariant return types and contravariant parameter types
func createFunctionTriggers(implementingMethod *types.Func, interfaceMethod *types.Func) []annotation.FullTrigger {
	triggers := make([]annotation.FullTrigger, 0)

	methodSig := implementingMethod.Type().(*types.Signature)
	affiliation := annotation.AffiliationPair{
		ImplementingMethod: implementingMethod,
		InterfaceMethod:    interfaceMethod,
	}

	// check for covariance in return types
	for i := 0; i < methodSig.Results().Len(); i++ {
		triggers = append(triggers, annotation.FullTriggerForInterfaceResultFlow(affiliation, i))
	}

	// check for contravariance in parameter types
	for i := 0; i < methodSig.Params().Len(); i++ {
		triggers = append(triggers, annotation.FullTriggerForInterfaceParamFlow(affiliation, i))

	}
	return triggers
}
