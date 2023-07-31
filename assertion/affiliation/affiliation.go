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
						if rhsSig, ok := util.TypeOf(pass, node.Rhs[0]).(*types.Tuple); ok && rhsSig.Len() == len(node.Lhs) {
							for i := range node.Lhs {
								lhsType := util.TypeOf(pass, node.Lhs[i])
								rhsType := rhsSig.At(i).Type()
								appendTypeToTypeTriggers(lhsType, rhsType)
							}
							return true
						}
					}
					// e.g., var i I, var s *S, i = s, or more generally, i1, i2, i3 = s1, s2, s3
					for i := 0; i < len(node.Lhs) && i < len(node.Rhs); i++ {
						lhsType := util.TypeOf(pass, node.Lhs[i])
						rhsType := util.TypeOf(pass, node.Rhs[i])
						appendTypeToTypeTriggers(lhsType, rhsType)
					}
				case *ast.ValueSpec:
					// e.g., var i I = &S{}
					for i := 0; i < len(node.Values); i++ {
						lhsType := util.TypeOf(pass, node.Type)
						rhsType := util.TypeOf(pass, node.Values[i])
						appendTypeToTypeTriggers(lhsType, rhsType)
					}
				case *ast.CallExpr:
					// e.g., func foo(i I), foo(&S{})
					if ident := util.FuncIdentFromCallExpr(node); ident != nil {
						if declObj := pass.TypesInfo.Uses[ident]; declObj != nil {
							if fdecl, ok := declObj.(*types.Func); ok {
								fsig := fdecl.Type().(*types.Signature)
								for i := 0; i < fsig.Params().Len() && i < len(node.Args); i++ {
									lhsType := fsig.Params().At(i).Type()      // receiver param of method declaration
									rhsType := util.TypeOf(pass, node.Args[i]) // caller param
									appendTypeToTypeTriggers(lhsType, rhsType)
								}
							}
						}
					}

					// slice is declared to be of interface type, and append function is used to add struct
					if sliceType, ok := util.IsSliceAppendCall(node, pass); ok {
						for i := 1; i < len(node.Args); i++ {
							lhsType := sliceType.Elem()
							rhsType := util.TypeOf(pass, node.Args[i])
							appendTypeToTypeTriggers(lhsType, rhsType)
						}
					}

				case *ast.TypeAssertExpr:
					// e.g., v, ok := i.(*S)
					lhsType := util.TypeOf(pass, node.X)
					rhsType := util.TypeOf(pass, node.Type)
					appendTypeToTypeTriggers(lhsType, rhsType)

				case *ast.ReturnStmt:
					// function signature states interface return, but the actual return is a struct
					// e.g., m(x *A) I { return x }
					var results = f.Type.Results
					if results != nil {
						funcSigResultsList := results.List
						for i := range node.Results {
							if i < len(funcSigResultsList) {
								lhsType := util.TypeOf(pass, funcSigResultsList[i].Type)
								rhsType := util.TypeOf(pass, node.Results[i])
								appendTypeToTypeTriggers(lhsType, rhsType)
							}
						}
					}

				case *ast.CompositeLit:
					nodeType := util.TypeOf(pass, node.Type)

					// If the composite is initializing a map, then check for possible pseudo-assignments
					// for keys and values. For instance, if the type of value of the map is an interface
					// but a struct is added to the map.
					if mpType, ok := nodeType.(*types.Map); ok {
						elemType := mpType.Elem()
						keyType := mpType.Key()
						// Iterate through each element in the composite
						for _, elt := range node.Elts {
							// This should be true as we have already checked that the composite is for a map
							if kv, ok := elt.(*ast.KeyValueExpr); ok {
								appendTypeToTypeTriggers(elemType, util.TypeOf(pass, kv.Value))
								appendTypeToTypeTriggers(keyType, util.TypeOf(pass, kv.Key))
							}
						}
					}

					// If the composite is used for initializing an array or a slice, then check for possible
					// pseudo-assignments through the initialized values of the elements in arrays/slice
					var elemType types.Type

					if slcType, ok := nodeType.(*types.Slice); ok {
						elemType = slcType.Elem()
					}

					if arrType, ok := nodeType.(*types.Array); ok {
						elemType = arrType.Elem()
					}

					if elemType != nil {
						// Iterate through each element in the composite
						for _, elt := range node.Elts {
							// This should be true as we have already checked that the composite is for an array/slice
							if un, ok := elt.(*ast.UnaryExpr); ok {
								appendTypeToTypeTriggers(elemType, util.TypeOf(pass, un))
							}
						}
					}
				case *ast.FuncLit:
					// TODO: Nilability analysis support for anonymous functions is currently not
					//       implemented (tracked in , so here we completely skip
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
	if !ok || rhsObj.NumMethods() <= 0 {
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
	declaredMethods := a.getDeclaredMethods(lhsObj)
	implementedMethods := a.getImplementedMethods(rhsObj)
	for _, dm := range declaredMethods {
		for _, im := range implementedMethods {
			// early return if the interface and its implementation is out of scope
			if (dm.Pkg() != nil && !a.conf.IsPkgInScope(dm.Pkg())) && (im.Pkg() != nil && !a.conf.IsPkgInScope(im.Pkg())) {
				return triggers
			}

			if dm.Name() == im.Name() {
				triggers = append(triggers, a.createFunctionTriggers(im, dm)...)
			}
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

// getDeclaredMethods returns all the methods declared by the interface "t"
func (*Affiliation) getDeclaredMethods(t *types.Interface) []*types.Func {
	methods := make([]*types.Func, 0)
	for i := 0; i < t.NumMethods(); i++ {
		methods = append(methods, t.Method(i))
	}
	return methods
}

// getImplementedMethods returns all the methods implemented by the struct "t"
func (*Affiliation) getImplementedMethods(t *types.Named) []*types.Func {
	methods := make([]*types.Func, 0)
	for i := 0; i < t.NumMethods(); i++ {
		methods = append(methods, t.Method(i))
	}
	return methods
}

// createFunctionTriggers verifies the nilability annotations of the concrete implementation of a method
// against its interface declaration for covariant return types and contravariant parameter types
func (*Affiliation) createFunctionTriggers(implementingMethod *types.Func, interfaceMethod *types.Func) []annotation.FullTrigger {
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
