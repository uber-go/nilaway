// This packages tests the behavior of nilaway on constants.
//
// <nilaway no inference>
package consts

import (
	"math"

	"go.uber.org/consts/lib"
)

var dummy bool

// tests for checking index expressions as constants both built-in and user defined (declared locally and in another package)
// nonnil(mp, mp[])
func testConst(mp map[string]*string, i int) string {
	switch i {
	case 0:
		// local const
		const key = "key"
		if mp[key] == nil || *mp[key] == "" {
			return "nil"
		} else {
			return *mp[key]
		}
	case 1:
		// local const created from a another package const
		const key = lib.MyStrConst
		if mp[key] == nil || *mp[key] == "" {
			return "nil"
		} else {
			return *mp[key]
		}
	case 2:
		// another package const
		if mp == nil || mp[lib.MyStrConst] == nil || *mp[lib.MyStrConst] == "" {
			return "nil"
		} else {
			return *mp[lib.MyStrConst]
		}
	case 3:
		var v = lib.MyStrConst
		if mp[v] == nil || *mp[v] == "" {
			return "nil"
		}
	case 4:
		// write and read from the same map
		if dummy {
			mp[lib.MyStrConst] = new(string)
			return *mp[lib.MyStrConst]
		}
		mp[lib.MyStrConst] = nil   //want "assigned"
		return *mp[lib.MyStrConst] //want "dereferenced"
	case 5:
		// built-in
		mp2 := make(map[float64]*string)
		if mp2[math.Pi] == nil || *mp2[math.Pi] == "" {
			return "nil"
		} else {
			return *mp2[math.Pi]
		}
	}
	return ""
}

var unexportedGlobalVar string = "local"

// tests for checking the behavior of indexing with a global variable.
// nonnil(mp, mp[])
func testGlobalVar(mp map[string]*string, i int) string {
	switch i {
	case 0:
		// locally defined unexported global variable
		if mp[unexportedGlobalVar] == nil || *mp[unexportedGlobalVar] == "" {
			return "nil"
		} else {
			return *mp[unexportedGlobalVar]
		}
	case 2:
		// global variable defined in another package
		if mp == nil || mp[lib.MyGlobalVar] == nil || *mp[lib.MyGlobalVar] == "" {
			return "nil"
		} else {
			return *mp[lib.MyGlobalVar]
		}
	}
	return ""
}
