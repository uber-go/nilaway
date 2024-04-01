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

/*
This package aims to test nilability behavior surrounding maps

<nilaway no inference>
*/
package maps

// var nilableMap map[int]*int
//
// // nonnil(nonnilMap)
// var nonnilMap = *new(map[int]*int)
//
// // nonnil(result 1)
// func retsNilableNonnilMaps() (map[int]*int, map[int]*int) {
// 	switch 0 {
// 	case 1:
// 		return make(map[int]*int), make(map[int]*int)
// 	case 2:
// 		return nil, nil //want "returned"
// 	case 3:
// 		return nilableMap, nilableMap //want "returned"
// 	case 4:
// 		return nonnilMap, nilableMap //want "returned"
// 	case 5:
// 		return nilableMap, nonnilMap
// 	default:
// 		return nonnilMap, nonnilMap
// 	}
// }
//
// // nonnil(nonnilMapParam, nonnilMapParam[])
// func testMapNilability(nilableMapParam, nonnilMapParam map[int]*int) *int {
// 	nilableMapResult, nonnilMapResult := retsNilableNonnilMaps()
//
// 	i := 0
//
// 	nilableMap[0] = nil //want "assigned" "written to at an index"
// 	nilableMap[1] = &i  //want "written to at an index"
// 	nonnilMap[0] = nil  //want "assigned"
// 	nonnilMap[1] = &i
//
// 	nilableMapParam[0] = nil //want "assigned" "written to at an index"
// 	nilableMapParam[1] = &i  //want "written to at an index"
// 	nonnilMapParam[0] = nil  //want "assigned"
// 	nonnilMapParam[1] = &i
//
// 	nilableMapResult[0] = nil //want "written to at an index"
// 	nilableMapResult[1] = &i  //want "written to at an index"
// 	nonnilMapResult[0] = nil
// 	nonnilMapResult[1] = &i
//
// 	switch 0 {
// 	case 1:
// 		return nilableMap[0] //want "returned"
// 	case 2:
// 		return nilableMap[1]
// 	case 3:
// 		return nilableMap[2] //want "returned"
// 	case 4:
// 		return nonnilMap[0] //want "returned"
// 	case 5:
// 		return nonnilMap[1]
// 	case 6:
// 		return nonnilMap[2] //want "returned"
// 	case 7:
// 		return nilableMapParam[0] //want "returned"
// 	case 8:
// 		return nilableMapParam[1]
// 	case 9:
// 		return nilableMapParam[2] //want "returned"
// 	case 10:
// 		return nonnilMapParam[0] //want "returned"
// 	case 11:
// 		return nonnilMapParam[1]
// 	case 12:
// 		return nonnilMapParam[2] //want "returned"
// 	case 13:
// 		return nilableMapResult[0] //want "returned"
// 	case 14:
// 		return nilableMapResult[1]
// 	case 15:
// 		return nilableMapResult[2] //want "returned"
// 	case 16:
// 		return nonnilMapResult[0] //want "returned"
// 	case 17:
// 		return nonnilMapResult[1]
// 	case 18:
// 		return nonnilMapResult[2] //want "returned"
// 	}
// 	return &i
// }
//
// // the following three functions have identical bodies except for the first 2 lines of each
//
// var dummy bool
//
// // nilable(deepNilableMapParam[])
// func testOkCheckForParams(deepNilableMapParam, deepNonnilMapParam map[int]*int) *int {
// 	vNonnil, okNonnil := deepNonnilMapParam[0]
// 	vNilable, okNilable := deepNilableMapParam[0]
//
// 	if dummy {
// 		return vNonnil //want "returned"
// 	}
// 	if dummy {
// 		return vNilable //want "returned"
// 	}
//
// 	if okNonnil {
// 		if dummy {
// 			return vNonnil
// 		}
// 		if dummy {
// 			return vNilable //want "returned"
// 		}
// 	}
//
// 	if okNilable {
// 		if dummy {
// 			return vNonnil //want "returned"
// 		}
// 		if dummy {
// 			return vNilable //want "returned"
// 		}
// 	}
//
// 	if dummy {
// 		return vNonnil //want "returned"
// 	}
// 	if dummy {
// 		return vNilable //want "returned"
// 	}
//
// 	switch 0 {
// 	case 1:
// 		okNonnil = true
//
// 		if okNonnil {
// 			// this case tests that assignments to the rich bool invalidate the check properly
// 			return vNonnil //want "returned"
// 		}
// 	case 2:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			okNonnil = true
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests that assignments in branching of degree
// 			// greater than 2 is still handled properly
// 			return vNonnil //want "returned"
// 		}
// 	case 3:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			vNonnil, okNonnil = deepNonnilMapParam[0]
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests an identical re-assignment
// 			// of vNonNil and okNonNil
// 			return vNonnil
// 		}
// 	case 4:
// 		var ok2Nonnil bool
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			vNonnil, ok2Nonnil = deepNonnilMapParam[0]
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests a non-identical re-assignment
// 			// of vNonNil to make sure the check is invalidated
// 			return vNonnil //want "returned"
// 		}
//
// 		if ok2Nonnil {
// 			// without this ok2Nonnil is unused and throws a static error
// 		}
// 	case 5:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but the 3-way switch is all no-ops, so
// 			// the rich bool should still be in place
// 			return vNonnil
// 		}
// 	}
//
// 	i := 0
// 	return &i
// }
//
// // nilable(deepNilableMap[])
// var deepNilableMap map[int]*int
//
// var deepNonnilMap map[int]*int
//
// func testOkCheckForGlobals() *int {
// 	vNonnil, okNonnil := deepNonnilMap[0]
// 	vNilable, okNilable := deepNilableMap[0]
//
// 	if dummy {
// 		return vNonnil //want "returned"
// 	}
// 	if dummy {
// 		return vNilable //want "returned"
// 	}
//
// 	if okNonnil {
// 		if dummy {
// 			return vNonnil
// 		}
// 		if dummy {
// 			return vNilable //want "returned"
// 		}
// 	}
//
// 	if okNilable {
// 		if dummy {
// 			return vNonnil //want "returned"
// 		}
// 		if dummy {
// 			return vNilable //want "returned"
// 		}
// 	}
//
// 	if dummy {
// 		return vNonnil //want "returned"
// 	}
// 	if dummy {
// 		return vNilable //want "returned"
// 	}
//
// 	switch 0 {
// 	case 1:
// 		okNonnil = true
//
// 		if okNonnil {
// 			// this case tests that assignments to the rich bool invalidate the check properly
// 			return vNonnil //want "returned"
// 		}
// 	case 2:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			okNonnil = true
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests that assignments in branching of degree
// 			// greater than 2 is still handled properly
// 			return vNonnil //want "returned"
// 		}
// 	case 3:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			vNonnil, okNonnil = deepNonnilMap[0]
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests an identical re-assignment
// 			// of vNonNil and okNonNil
// 			return vNonnil
// 		}
// 	case 4:
// 		var ok2Nonnil bool
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			vNonnil, ok2Nonnil = deepNonnilMap[0]
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests a non-identical re-assignment
// 			// of vNonNil to make sure the check is invalidated
// 			return vNonnil //want "returned"
// 		}
//
// 		if ok2Nonnil {
// 			// without this ok2Nonnil is unused and throws a static error
// 		}
// 	case 5:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but the 3-way switch is all no-ops, so
// 			// the rich bool should still be in place
// 			return vNonnil
// 		}
// 	}
//
// 	i := 0
// 	return &i
// }
//
// func testOkCheckForLocals() *int {
// 	// without , no way to have a deeply nilable local map here
// 	var deepNonnilMap = make(map[int]*int)
// 	vNonnil, okNonnil := deepNonnilMap[0]
//
// 	if dummy {
// 		return vNonnil //want "returned"
// 	}
//
// 	if okNonnil {
// 		if dummy {
// 			return vNonnil
// 		}
// 	}
//
// 	if dummy {
// 		return vNonnil //want "returned"
// 	}
//
// 	switch 0 {
// 	case 1:
// 		okNonnil = true
//
// 		if okNonnil {
// 			// this case tests that assignments to the rich bool invalidate the check properly
// 			return vNonnil //want "returned"
// 		}
// 	case 2:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			okNonnil = true
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests that assignments in branching of degree
// 			// greater than 2 is still handled properly
// 			return vNonnil //want "returned"
// 		}
// 	case 3:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			vNonnil, okNonnil = deepNonnilMap[0]
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests an identical re-assignment
// 			// of vNonNil and okNonNil
// 			return vNonnil
// 		}
// 	case 4:
// 		var ok2Nonnil bool
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			vNonnil, ok2Nonnil = deepNonnilMap[0]
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests a non-identical re-assignment
// 			// of vNonNil to make sure the check is invalidated
// 			return vNonnil //want "returned"
// 		}
//
// 		if ok2Nonnil {
// 			// without this ok2Nonnil is unused and throws a static error
// 		}
// 	case 5:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but the 3-way switch is all no-ops, so
// 			// the rich bool should still be in place
// 			return vNonnil
// 		}
// 	}
//
// 	i := 0
// 	return &i
// }
//
// // nilable(result 0[])
// func retsDeepNilableNonnilMaps() (map[int]*int, map[int]*int) {
// 	return make(map[int]*int), make(map[int]*int)
// }
//
// func testOkCheckForResults() *int {
// 	deepNilableMapResult, deepNonnilMapResult := retsDeepNilableNonnilMaps()
// 	vNonnil, okNonnil := deepNonnilMapResult[0]
// 	vNilable, okNilable := deepNilableMapResult[0]
//
// 	if dummy {
// 		return vNonnil //want "returned"
// 	}
// 	if dummy {
// 		return vNilable //want "returned"
// 	}
//
// 	if okNonnil {
// 		if dummy {
// 			return vNonnil
// 		}
// 		if dummy {
// 			return vNilable //want "returned"
// 		}
// 	}
//
// 	if okNilable {
// 		if dummy {
// 			return vNonnil //want "returned"
// 		}
// 		if dummy {
// 			return vNilable //want "returned"
// 		}
// 	}
//
// 	if dummy {
// 		return vNonnil //want "returned"
// 	}
// 	if dummy {
// 		return vNilable //want "returned"
// 	}
//
// 	switch 0 {
// 	case 1:
// 		okNonnil = true
//
// 		if okNonnil {
// 			// this case tests that assignments to the rich bool invalidate the check properly
// 			return vNonnil //want "returned"
// 		}
// 	case 2:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			okNonnil = true
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests that assignments in branching of degree
// 			// greater than 2 is still handled properly
// 			return vNonnil //want "returned"
// 		}
// 	case 3:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			vNonnil, okNonnil = deepNonnilMapResult[0]
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests an identical re-assignment
// 			// of vNonNil and okNonNil
// 			return vNonnil
// 		}
// 	case 4:
// 		var ok2Nonnil bool
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 			vNonnil, ok2Nonnil = deepNonnilMapResult[0]
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but tests a non-identical re-assignment
// 			// of vNonNil to make sure the check is invalidated
// 			return vNonnil //want "returned"
// 		}
//
// 		if ok2Nonnil {
// 			// without this ok2Nonnil is unused and throws a static error
// 		}
// 	case 5:
// 		switch 0 {
// 		case 1:
// 		case 2:
// 		case 3:
// 		}
//
// 		if okNonnil {
// 			// this case is similar to above, but the 3-way switch is all no-ops, so
// 			// the rich bool should still be in place
// 			return vNonnil
// 		}
// 	}
//
// 	i := 0
// 	return &i
// }
//
// func takesNonnil(interface{}) {}
//
// func singleKeysEstablishNonnil(m map[int]*int) {
// 	v, ok := m[0]
//
// 	// here, m and v should be nilable
// 	takesNonnil(v) //want "passed"
// 	takesNonnil(m) //want "passed"
//
// 	switch 0 {
// 	case 1:
// 		if !ok {
// 			return
// 		}
//
// 		// here, we should know that BOTH v and m and nonnil
// 		takesNonnil(v)
// 		takesNonnil(m)
// 	case 4:
// 		ok = true
//
// 		if !ok {
// 			return
// 		}
//
// 		// here, neither v nor m should be nonnil
// 		takesNonnil(v) //want "passed"
// 		takesNonnil(m) //want "passed"
// 	case 5:
// 		v = nil
//
// 		if !ok {
// 			return
// 		}
//
// 		// here, JUST m should be nonnil
// 		takesNonnil(v) //want "passed"
// 		takesNonnil(m)
// 	case 6:
// 		m = nil
//
// 		if !ok {
// 			return
// 		}
//
// 		// here, JUST v should be nonnil
// 		takesNonnil(v)
// 		takesNonnil(m) //want "passed"
// 	}
// }
//
// func plainReflCheck(m map[any]any) any {
// 	if dummy {
// 		return m //want "returned"
// 	}
//
// 	_, ok := m[0]
//
// 	if ok {
// 		return m
// 	}
//
// 	return m //want "returned"
// }
//
// // tests for checking explicit boolean checks
// // nonnil(mp, mp[])
// func testExplicitBool(mp map[int]*int, i int) *int {
// 	switch i {
// 	case 0:
// 		if x, ok := mp[i]; ok == true {
// 			return x
// 		}
// 	case 1:
// 		if x, ok := mp[i]; ok != true {
// 			return x //want "returned"
// 		}
// 	case 2:
// 		if x, ok := mp[i]; ok != false {
// 			return x
// 		}
// 	case 3:
// 		if x, ok := mp[i]; true == ok {
// 			return x
// 		}
// 	case 4:
// 		if x, ok := mp[i]; true != ok {
// 			return x //want "returned"
// 		}
// 	case 5:
// 		var x *int
// 		var ok bool
// 		if x, ok = mp[0]; ok == false {
// 			x = &i
// 			mp[0] = x
// 		}
// 		return x
// 	case 6:
// 		if x, ok := mp[i]; ok != false {
// 			return x
// 		}
// 	case 7:
// 		if x, ok := mp[i]; ok != true {
// 			return x //want "returned"
// 		}
// 	case 8:
// 		if x, ok := mp[i]; false == ok {
// 			return x //want "returned"
// 		}
// 	case 9:
// 		if x, ok := mp[i]; false != ok {
// 			return x
// 		}
// 	case 10:
// 		if x, ok := mp[i]; true != ok {
// 			return x //want "returned"
// 		}
// 	case 11:
// 		if x, ok := mp[i]; !(!(!(!(true != ok) || ok == true))) {
// 			return x //want "returned"
// 		}
// 	case 12:
// 		x, ok1 := mp[0]
// 		y, ok2 := mp[1]
// 		if ok1 == true && ok2 != false {
// 			return x
// 		}
// 		if ok1 == true || ok2 == true {
// 			return y //want "returned"
// 		}
// 	case 13:
// 		if x, _ := mp[0]; true == true || true != false || false == false || false != true {
// 			return x //want "returned"
// 		}
// 	case 14:
// 		if x, ok := mp[i]; ok == true || i > 5 {
// 			return x //want "returned"
// 		}
// 	}
// 	return &i
// }

// nonnil(mp, mp[])
func testme(mp map[int]*int) {
	if _, ok := mp[0]; !ok {
		mp[0] = new(int)
	}
	_ = *mp[0]
}
