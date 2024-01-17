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

package annotation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"

	"github.com/stretchr/testify/mock"
	"go.uber.org/nilaway/util"
	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/packages"
)

// mockKey is a mock implementation of the Key interface
type mockKey struct {
	mock.Mock
}

func (m *mockKey) Lookup(m2 Map) (Val, bool) {
	args := m.Called(m2)
	return args.Get(0).(Val), args.Bool(1)
}

func (m *mockKey) Object() types.Object {
	args := m.Called()
	return args.Get(0).(types.Object)
}

func (m *mockKey) equals(other Key) bool {
	args := m.Called(other)
	return args.Bool(0)
}

func (m *mockKey) copy() Key {
	args := m.Called()
	return args.Get(0).(Key)
}

func newMockKey() *mockKey {
	mockedKey := new(mockKey)
	mockedKey.ExpectedCalls = nil
	mockedKey.On("equals", mock.Anything).Return(true)

	copiedMockKey := new(mockKey)
	mockedKey.ExpectedCalls = nil
	mockedKey.On("equals", mock.Anything).Return(true)

	mockedKey.On("copy").Return(copiedMockKey)
	return mockedKey
}

// mockProducingAnnotationTrigger is a mock implementation of the ProducingAnnotationTrigger interface
type mockProducingAnnotationTrigger struct {
	mock.Mock
}

func (m *mockProducingAnnotationTrigger) CheckProduce(m2 Map) bool {
	args := m.Called(m2)
	return args.Bool(0)
}

func (m *mockProducingAnnotationTrigger) NeedsGuardMatch() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockProducingAnnotationTrigger) SetNeedsGuard(b bool) {
	m.Called(b)
}

func (m *mockProducingAnnotationTrigger) Prestring() Prestring {
	args := m.Called()
	return args.Get(0).(Prestring)
}

func (m *mockProducingAnnotationTrigger) Kind() TriggerKind {
	args := m.Called()
	return args.Get(0).(TriggerKind)
}

func (m *mockProducingAnnotationTrigger) UnderlyingSite() Key {
	args := m.Called()
	return args.Get(0).(Key)
}

func (m *mockProducingAnnotationTrigger) equals(other ProducingAnnotationTrigger) bool {
	args := m.Called(other)
	return args.Bool(0)
}

// getImplementedMethods is a helper function that returns all the methods implemented by the struct "t"
func getImplementedMethods(t *types.Named) []*types.Func {
	visitedMethods := make(map[string]*types.Func) // helps in only storing the latest overridden implementation of a method
	visitedStructs := make(map[*types.Struct]bool) // helps in avoiding infinite recursion if there is a cycle in the struct embedding
	collectMethods(t, visitedMethods, visitedStructs)
	return maps.Values(visitedMethods)
}

// collectMethods is a helper function that recursively collects all `methods` implemented by the struct `t`.
// Methods inherited from the embedded and anonymous fields of `t` are collected in a DFS manner. In case of overriding,
// only the overridden implementation of the method is stored with the help of `visitedMethodNames`. For example,
// consider the following illustrative example, and the collected methods at different casting sites.
// ```
// type S struct { ... }			func (s *S) foo() { ... }		s := &S{} // methods = [foo()]
// type T struct { S }				func (t *T) bar() { ... }		t := &T{} // methods = [bar()]
// type U struct { T }												u := &U{} // methods = [foo(), bar()]
// ```
func collectMethods(t *types.Named, visitedMethods map[string]*types.Func, visitedStructs map[*types.Struct]bool) {
	for i := 0; i < t.NumMethods(); i++ {
		m := t.Method(i)
		if _, ok := visitedMethods[m.Name()]; !ok {
			visitedMethods[m.Name()] = m
		}
	}

	// collect methods from embedded fields
	if s := util.TypeAsDeeplyStruct(t); s != nil && !visitedStructs[s] {
		visitedStructs[s] = true
		for i := 0; i < s.NumFields(); i++ {
			f := s.Field(i)
			if f.Embedded() {
				if n, ok := util.UnwrapPtr(f.Type()).(*types.Named); ok {
					collectMethods(n, visitedMethods, visitedStructs)
				}
			}
		}
	}
}

// structsImplementingInterface is a helper function that returns all the struct names implementing the given interface
// in the given package recursively
func structsImplementingInterface(interfaceName string, packageName ...string) map[string]bool {
	structs := make(map[string]bool)

	// if no package name is provided, default to using the current directory
	if len(packageName) == 0 {
		packageName = []string{"."}
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
	}

	for _, p := range packageName {
		pkgs, err := packages.Load(cfg, p)
		if err != nil {
			panic(err)
		}
		if len(pkgs) == 0 {
			panic("no packages found")
		}

		for _, pkg := range pkgs {
			// scan the packages to find the interface and get its *types.Interface object
			obj := pkgs[0].Types.Scope().Lookup(interfaceName)
			if obj == nil {
				continue
			}
			interfaceObj, ok := obj.Type().Underlying().(*types.Interface)
			if !ok {
				continue
			}

			// iterate over all Go files in the package to find the structs implementing the interface
			for _, filepath := range pkg.GoFiles {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, filepath, nil, parser.AllErrors)
				if err != nil {
					panic(err)
				}

				ast.Inspect(node, func(n ast.Node) bool {
					if typeSpec, ok := n.(*ast.TypeSpec); ok {
						if _, ok := typeSpec.Type.(*ast.StructType); ok {
							sObj := pkg.Types.Scope().Lookup(typeSpec.Name.Name)
							if sObj == nil {
								return true
							}
							sType, ok := sObj.Type().(*types.Named)
							if !ok {
								return true
							}

							structMethods := getImplementedMethods(sType)
							if interfaceObj.NumMethods() > len(structMethods) {
								return true
							}

							// compare the methods of the interface and the struct, increment `match` if the method names match
							match := 0
							for i := 0; i < interfaceObj.NumMethods(); i++ {
								iMethod := interfaceObj.Method(i)
								for _, sMethod := range structMethods {
									if iMethod.Name() == sMethod.Name() {
										match++
									}
								}
							}
							if match == interfaceObj.NumMethods() {
								// we have found a struct that implements the interface
								structs[typeSpec.Name.Name] = true
							}
						}
					}
					return true
				})
			}
		}
	}
	return structs
}

func structsCheckedTestHelper(interfaceName string, packagePath string, initStructs []any) []string {
	expected := structsImplementingInterface(interfaceName, packagePath)
	if len(expected) == 0 {
		panic(fmt.Sprintf("no structs found implementing `%s` interface", interfaceName))
	}

	actual := make(map[string]bool)
	for _, initStruct := range initStructs {
		actual[reflect.TypeOf(initStruct).Elem().Name()] = true
	}

	// compare expected and actual, and find structs that were not tested
	var missedStructs []string
	for structName := range expected {
		if !actual[structName] {
			missedStructs = append(missedStructs, structName)
		}
	}
	return missedStructs
}
