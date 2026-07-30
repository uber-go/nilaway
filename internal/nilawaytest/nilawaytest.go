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

// Package nilawaytest holds test-utility code shared across NilAway's packages.
// It lives under internal so external users cannot depend on it.
package nilawaytest

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"maps"
	"reflect"
	"slices"

	"go.uber.org/nilaway/util/typeshelper"
	"golang.org/x/tools/go/packages"
)

// StructsImplementingInterface returns the names of all structs implementing
// interfaceName in the provided packages. If no package pattern, or an empty
// package pattern, is provided, it defaults to the current package.
func StructsImplementingInterface(interfaceName string, packagePatterns ...string) map[string]bool {
	structs := make(map[string]bool)

	if len(packagePatterns) == 0 || (len(packagePatterns) == 1 && packagePatterns[0] == "") {
		packagePatterns = []string{"."}
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
	}

	for _, pattern := range packagePatterns {
		pkgs, err := packages.Load(cfg, pattern)
		if err != nil {
			panic(err)
		}
		if len(pkgs) == 0 {
			panic("no packages found")
		}

		for _, pkg := range pkgs {
			obj := pkg.Types.Scope().Lookup(interfaceName)
			if obj == nil {
				continue
			}
			interfaceObj, ok := obj.Type().Underlying().(*types.Interface)
			if !ok {
				continue
			}

			for _, filePath := range pkg.GoFiles {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
				if err != nil {
					panic(err)
				}

				ast.Inspect(node, func(n ast.Node) bool {
					typeSpec, ok := n.(*ast.TypeSpec)
					if !ok {
						return true
					}
					if _, ok := typeSpec.Type.(*ast.StructType); !ok {
						return true
					}

					sObj := pkg.Types.Scope().Lookup(typeSpec.Name.Name)
					if sObj == nil {
						return true
					}
					sType, ok := types.Unalias(sObj.Type()).(*types.Named)
					if !ok {
						return true
					}

					structMethods := getImplementedMethods(sType)
					if interfaceObj.NumMethods() > len(structMethods) {
						return true
					}

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
						structs[typeSpec.Name.Name] = true
					}
					return true
				})
			}
		}
	}

	return structs
}

// ObjInfo holds the address, field count, and type of a struct or struct field
// captured by GetObjInfo.
type ObjInfo struct {
	Addr      string
	NumFields int
	Typ       reflect.Type
}

// GetObjInfo returns a map from struct and field names to their ObjInfo. The
// keys are formatted as `struct_<struct name>` or `fld_<struct name>.<field name>`.
func GetObjInfo(obj any) map[string]ObjInfo {
	ptr := make(map[string]ObjInfo)

	val := reflect.ValueOf(obj).Elem()
	ptr[fmt.Sprintf("struct_%s", val.Type().Name())] = ObjInfo{
		Addr:      fmt.Sprintf("%p", val.Addr().Interface()),
		NumFields: val.NumField(),
		Typ:       val.Type(),
	}
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		key := fmt.Sprintf("fld_%s.%s", val.Type().Name(), val.Type().Field(i).Name)
		if field.Kind() == reflect.Ptr {
			if !field.IsZero() {
				ptr[key] = ObjInfo{
					Addr:      fmt.Sprintf("%p", field.Interface()),
					NumFields: field.Elem().NumField(),
					Typ:       field.Elem().Type(),
				}
			}
		} else if field.Kind() == reflect.Interface && !field.IsNil() {
			interfaceValue := field.Interface()
			underlyingValue := reflect.ValueOf(interfaceValue).Elem()
			ptr[key] = ObjInfo{
				Addr:      fmt.Sprintf("%p", underlyingValue.Addr().Interface()),
				NumFields: underlyingValue.NumField(),
				Typ:       underlyingValue.Type(),
			}
		} else {
			ptr[key] = ObjInfo{Typ: field.Type()}
		}
	}
	return ptr
}

// MissingStructs returns the names of interface implementations that exist in
// packagePath but are not represented in testedStructs.
func MissingStructs[T any](interfaceName, packagePath string, testedStructs []T) []string {
	expected := StructsImplementingInterface(interfaceName, packagePath)
	if len(expected) == 0 {
		panic(fmt.Sprintf("no structs found implementing `%s` interface", interfaceName))
	}

	actual := make(map[string]bool, len(testedStructs))
	for _, s := range testedStructs {
		actual[reflect.TypeOf(s).Elem().Name()] = true
	}

	var missedStructs []string
	for structName := range expected {
		if !actual[structName] {
			missedStructs = append(missedStructs, structName)
		}
	}
	return missedStructs
}

// getImplementedMethods returns all methods implemented by t.
func getImplementedMethods(t *types.Named) []*types.Func {
	visitedMethods := make(map[string]*types.Func)
	visitedStructs := make(map[*types.Struct]bool)
	collectMethods(t, visitedMethods, visitedStructs)
	return slices.AppendSeq(make([]*types.Func, 0, len(visitedMethods)), maps.Values(visitedMethods))
}

// collectMethods recursively collects methods implemented by t, including
// methods promoted from embedded fields.
func collectMethods(t *types.Named, visitedMethods map[string]*types.Func, visitedStructs map[*types.Struct]bool) {
	for i := 0; i < t.NumMethods(); i++ {
		m := t.Method(i)
		if _, ok := visitedMethods[m.Name()]; !ok {
			visitedMethods[m.Name()] = m
		}
	}

	if s := typeshelper.AsDeeplyStruct(t); s != nil && !visitedStructs[s] {
		visitedStructs[s] = true
		for i := 0; i < s.NumFields(); i++ {
			f := s.Field(i)
			if f.Embedded() {
				if n, ok := typeshelper.UnwrapPtr(types.Unalias(f.Type())).(*types.Named); ok {
					collectMethods(n, visitedMethods, visitedStructs)
				}
			}
		}
	}
}
