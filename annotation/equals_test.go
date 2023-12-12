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
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"strings"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/nilaway/util"
	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/packages"
)

// This test file tests the implementation of the `equals` method defined for the interfaces `ConsumingAnnotationTrigger`,
// `ProducingAnnotationTrigger` and `Key`.

// Below are the helper utilities used in the tests, such as mock implementations of the interfaces and utility functions.

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

// EqualsTestSuite defines the test suite for the `equals` method.
type EqualsTestSuite struct {
	suite.Suite
	initStructs   []any
	interfaceName string
	packagePath   string
}

// This test checks that the `equals` method of all the implemented consumer structs when compared with themselves
// returns true. Although trivial, this test is important to ensure that the type assertion in `equals` method is
// implemented correctly.
func (s *EqualsTestSuite) TestEqualsTrue() {
	msg := "equals() of `%T` should return true when compared with object of same type"

	for _, initStruct := range s.initStructs {
		switch t := initStruct.(type) {
		case ConsumingAnnotationTrigger:
			s.Truef(t.equals(t), msg, t)
		case ProducingAnnotationTrigger:
			s.Truef(t.equals(t), msg, t)
		case Key:
			s.Truef(t.equals(t), msg, t)
		default:
			s.Failf("unknown type", "unknown type `%T`", t)
		}
	}
}

// This test checks that the `equals` method of all the implemented consumer structs when compared with any other consumer
// struct returns false. This test is important to ensure that the `equals` method is robust to differentiate between
// different consumer struct types.
func (s *EqualsTestSuite) TestEqualsFalse() {
	msg := "equals() of `%T` should return false when compared with object of different type `%T`"

	for _, s1 := range s.initStructs {
		for _, s2 := range s.initStructs {
			if s1 != s2 {
				switch t1 := s1.(type) {
				case ConsumingAnnotationTrigger:
					if t2, ok := s2.(ConsumingAnnotationTrigger); ok {
						s.Falsef(t1.equals(t2), msg, t1, t2)
					}
				case ProducingAnnotationTrigger:
					if t2, ok := s2.(ProducingAnnotationTrigger); ok {
						s.Falsef(t1.equals(t2), msg, t1, t2)
					}
				case Key:
					if t2, ok := s2.(Key); ok {
						s.Falsef(t1.equals(t2), msg, t1, t2)
					}
				default:
					s.Failf("unknown type", "unknown type `%T`", t1)
				}
			}
		}
	}
}

// This test serves as a sanity check to ensure that all the implemented consumer structs are tested in this file.
// Ideally, we would have liked to  programmatically parse all the consumer structs, instantiate them, and call their
// methods. However, this does not seem to be possible. Therefore, we rely on this not-so-ideal, but practical approach.
// It finds the expected list of consumer structs implementing the interface under test (e.g., `ConsumingAnnotationTrigger`)
// using `structsImplementingInterface()`, and finds the actual list of consumer structs that are tested in the
// governing test case. The test fails if there are any structs that are missing from the expected list.
func (s *EqualsTestSuite) TestStructsChecked() {
	expected := structsImplementingInterface(s.interfaceName, s.packagePath)
	s.NotEmpty(expected, "no structs found implementing `%s` interface", s.interfaceName)

	actual := make(map[string]bool)
	for _, initStruct := range s.initStructs {
		actual[reflect.TypeOf(initStruct).Elem().Name()] = true
	}

	// compare expected and actual, and find structs that were not tested
	var missedStructs []string
	for structName := range expected {
		if !actual[structName] {
			missedStructs = append(missedStructs, structName)
		}
	}
	// if there are any structs that were not tested, fail the test and print the list of structs
	s.Equalf(0, len(missedStructs), "the following structs were not tested: [`%s`]", strings.Join(missedStructs, "`, `"))
}
