//	Copyright (c) 2023 Uber Technologies, Inc.
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
	"reflect"
	"strings"

	"github.com/stretchr/testify/suite"
)

type CopyTestSuite struct {
	suite.Suite
	initStructs   []any
	interfaceName string
	packagePath   string
}

type objInfo struct {
	addr      string
	numFields int
	typ       reflect.Type
}

func newObjInfo(addr string, numFields int, typ reflect.Type) objInfo {
	return objInfo{
		addr:      addr,
		numFields: numFields,
		typ:       typ,
	}
}

// getObjInfo is a helper function that returns a map of struct and field names to their objInfo.
// The key is in the format of `struct_<struct name>` or `fld_<struct name>.<field name>`.
func getObjInfo(obj any) map[string]objInfo {
	ptr := make(map[string]objInfo)

	val := reflect.ValueOf(obj).Elem()
	ptr[fmt.Sprintf("struct_%s", val.Type().Name())] = newObjInfo(fmt.Sprintf("%p", val.Addr().Interface()), val.NumField(), val.Type())
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		key := fmt.Sprintf("fld_%s.%s", val.Type().Name(), val.Type().Field(i).Name)
		if field.Kind() == reflect.Ptr {
			if !field.IsZero() {
				ptr[key] = newObjInfo(fmt.Sprintf("%p", field.Interface()), field.Elem().NumField(), field.Elem().Type())
			}
		} else if field.Kind() == reflect.Interface && !field.IsNil() {
			// %p cannot be used directly with a reflect.Value, so we need to extract the underlying value first.
			interfaceValue := field.Interface()
			underlyingValue := reflect.ValueOf(interfaceValue).Elem()
			ptr[key] = newObjInfo(fmt.Sprintf("%p", underlyingValue.Addr().Interface()), underlyingValue.NumField(), underlyingValue.Type())
		} else {
			ptr[key] = newObjInfo("", 0, field.Type())
		}
	}
	return ptr
}

// This test checks that the `Copy` method implementations perform a deep copy, i.e., copies the values but generates
// different pointer addresses for the copied struct and its fields.
// Note that here we cannot use `reflect.DeepEqual` to compare the original and copied structs because reflection
// does not work well with fields with nested struct pointers, giving incorrect results.
// Therefore, we compare the original and copied structs along with their fields for:
// - type
// - number of fields
// - pointer address (if the field is a struct and has at least one field)
func (s *CopyTestSuite) TestCopy() {
	var expectedObjs, actualObjs map[string]objInfo

	for _, initStruct := range s.initStructs {
		var copied any
		expectedObjs = getObjInfo(initStruct)

		switch t := initStruct.(type) {
		case ConsumingAnnotationTrigger:
			copied = t.Copy()
			actualObjs = getObjInfo(copied)
		case Key:
			copied = t.copy()
			actualObjs = getObjInfo(copied)
		default:
			s.Failf("unknown type", "unknown type %T", t)
		}

		for expectedKey, expectedObj := range expectedObjs {
			actualObj, ok := actualObjs[expectedKey]
			s.True(ok, "key `%s` should exist in copied struct object", expectedKey)
			s.Equal(expectedObj.typ, actualObj.typ, "key `%s` should have the same type after deep copying", expectedKey)
			s.Equal(expectedObj.numFields, actualObj.numFields, "key `%s` should have the same number of fields after deep copying", expectedKey)

			// Note that Go optimizes the memory allocation of pointers to structs. The pointer address for structs with
			// no fields will be the same. E.g., consider struct `S` with no fields, then `s1 := &S{}, s2 := &S{};
			// fmt.Printf("%p %p", s1, s2)` will print the same address. Therefore, we only add the pointer address of a struct
			// if it has at least one field. The reason for this being that currently, the use of this helper function is used only in
			// the `CopyTestSuite` to check that the `Copy` method implementations perform a deep copy, i.e., generates different
			// pointer addresses for the copied struct and its fields. We may want to modify this behavior in the future, if needed.
			if expectedObj.addr != "" && actualObj.addr != "" && expectedObj.numFields > 0 && actualObj.numFields > 0 {
				s.NotEqual(expectedObj.addr, actualObj.addr, "key `%s` should not have the same pointer value after deep copying", expectedKey)
			}
		}
	}
}

// Similar to EqualsTestSuite, this test serves as a sanity check to ensure that all the implemented consumer structs
// are tested in this file. The test fails if there are any structs that are found missing from the expected list.
func (s *CopyTestSuite) TestStructsChecked() {
	missedStructs := structsCheckedTestHelper(s.interfaceName, s.packagePath, s.initStructs)
	s.Equalf(0, len(missedStructs), "the following structs were not tested: [`%s`]", strings.Join(missedStructs, "`, `"))
}
