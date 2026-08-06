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
*/
package maps

var dummy bool

// nilableMap is deeply nilable purely by inference: feedMaps below stores nil into it. nonnilMap
// only ever stores nonnil values, so its deep nilability stays unconstrained and guarded reads
// from it can be safely consumed.
var nilableMap = map[int]*int{}
var nonnilMap = map[int]*int{}

func feedMaps(i int) {
	nilableMap[0] = nil
	nonnilMap[0] = &i
}

// BELOW TESTS CHECK SHALLOW NILABILITY OF MAPS :: WRITING AT AN INDEX OF A NIL MAP PANICS.
// Each nil map here is a dedicated source: NilAway reports only the first conflict flowing out of
// a non-force-determined nilable site, and flows from the same source into multiple in-place
// consumers are grouped under a single diagnostic, so sharing a source across sections would
// silently swallow the other conflicts.

var indexWriteGlobalMap map[int]*int

func testIndexWriteGlobal(i int) {
	// Both writes below flow from the same nil source (the uninitialized global above), so
	// NilAway reports a single diagnostic for them (the other write is mentioned in a
	// "same nil source" note inside it).
	indexWriteGlobalMap[0] = &i
	indexWriteGlobalMap[1] = &i //want "written to at an index"
}

func testIndexWriteParam(m map[int]*int, i int) {
	m[0] = &i //want "written to at an index"
}

func callTestIndexWriteParam() {
	testIndexWriteParam(nil, 0)
}

func retsNilMap() map[int]*int {
	if dummy {
		return nil
	}
	return make(map[int]*int)
}

func testIndexWriteResult(i int) {
	r := retsNilMap()
	r[0] = &i //want "written to at an index"
}

// BELOW TESTS CHECK DEEP READS OF MAPS :: `m[k]` IS NIL WHEN THE STORED VALUE IS NIL OR THE KEY
// IS MISSING.
// Each function stores nil at key 0 and a nonnil value at key 1, then returns the reads: the read
// at key 0 yields the stored nil, the read at key 1 is safe (the write establishes both presence
// and a nonnil value), and the read at key 2 may miss and yield the zero value. The consuming
// caller is declared before the function so that each unsafe flow is reported individually at the
// caller's dereference.

var presenceGlobalMap = map[int]*int{}

func callTestDeepReadsGlobal() {
	print(*testDeepReadsGlobal(0)) //want "returned from `testDeepReadsGlobal.*` in position 0" "returned from `testDeepReadsGlobal.*` in position 0"
}

func testDeepReadsGlobal(i int) *int {
	presenceGlobalMap[0] = nil
	presenceGlobalMap[1] = &i

	switch i {
	case 1:
		return presenceGlobalMap[0] // stored value is nil
	case 2:
		return presenceGlobalMap[1] // safe: stored value is nonnil and the key is present
	case 3:
		return presenceGlobalMap[2] // the key may be missing
	}
	return &i
}

func callTestDeepReadsParam() {
	print(*testDeepReadsParam(map[int]*int{}, 0)) //want "returned from `testDeepReadsParam.*` in position 0" "returned from `testDeepReadsParam.*` in position 0"
}

func testDeepReadsParam(m map[int]*int, i int) *int {
	m[0] = nil
	m[1] = &i

	switch i {
	case 1:
		return m[0] // stored value is nil
	case 2:
		return m[1] // safe: stored value is nonnil and the key is present
	case 3:
		return m[2] // the key may be missing
	}
	return &i
}

func retsFreshMap() map[int]*int {
	return make(map[int]*int)
}

func callTestDeepReadsResult() {
	print(*testDeepReadsResult(0)) //want "returned from `testDeepReadsResult.*` in position 0" "returned from `testDeepReadsResult.*` in position 0"
}

func testDeepReadsResult(i int) *int {
	m := retsFreshMap()
	m[0] = nil
	m[1] = &i

	switch i {
	case 1:
		return m[0] // stored value is nil
	case 2:
		return m[1] // safe: stored value is nonnil and the key is present
	case 3:
		return m[2] // the key may be missing
	}
	return &i
}

// BELOW TESTS CHECK RETURNING NIL MAPS FROM A MULTI-RESULT FUNCTION.
// The caller writes into result 1 at an index, so every nilable return in position 1 is reported
// there; position 0 is never consumed, so its nilable returns are silent under inference. The two
// `nil` literals are distinct sources and report separately, while shallowNilMap is a single
// source: returning it in position 1 twice would be deduplicated (one conflict per pair of
// inference sites), hence it is returned in position 1 only once.

var shallowNilMap map[int]*int

func callRetsNilableNonnilMaps(i int) {
	_, m := retsNilableNonnilMaps()
	m[0] = &i //want "returned from `retsNilableNonnilMaps.*` in position 1" "returned from `retsNilableNonnilMaps.*` in position 1" "returned from `retsNilableNonnilMaps.*` in position 1"
}

func retsNilableNonnilMaps() (map[int]*int, map[int]*int) {
	switch 0 {
	case 1:
		return make(map[int]*int), make(map[int]*int)
	case 2:
		return nil, nil
	case 3:
		return shallowNilMap, shallowNilMap
	default:
		return make(map[int]*int), nil
	}
}

// BELOW TESTS CHECK DEEP WRITES OF NILABLE VALUES INTO MAPS WHOSE READS ARE CONSUMED AS NONNIL.
// The mixed* maps below have their (ok-guarded) reads dereferenced but receive exactly one
// nilable store each: once a site is determined nilable, further nilable stores into it agree
// with the existing value and do not create additional conflicts, so a one-store-one-deref
// pairing keeps every diagnostic attributable to a specific store. The reads must be ok-guarded:
// an unguarded read would already be reported for the key possibly missing.

var mixedMapFromArg = map[int]*int{}
var mixedMapFromRead = map[int]*int{}

func drainMixedMapFromArg() {
	if v, ok := mixedMapFromArg[0]; ok {
		print(*v) //want "assigned deeply into global variable `mixedMapFromArg`"
	}
}

func drainMixedMapFromRead() {
	if v, ok := mixedMapFromRead[0]; ok {
		print(*v) //want "assigned deeply into global variable `mixedMapFromRead`"
	}
}

// testMapDeepWrites checks that storing nilable values into maps whose reads are consumed as
// nonnil is reported, while storing nonnil values (or storing into the deeply nilable map) is
// safe. The two unsafe global stores use different nilable sources (a nil argument and a guarded
// read from the deeply nilable map); the unsafe store into the parameter conflicts with the
// in-function dereference in case 3. Note that nilableArgA and nilableArgB must be separate
// parameters: once a nilable pointer site is determined via its first conflicting flow, edges
// from that site into other maps are discarded as redundant, so a shared parameter would yield
// only one conflict.
func testMapDeepWrites(mixedMapParam map[int]*int, nilableArgA, nilableArgB, nonnilArg *int) {
	switch 0 {
	case 1:
		nilableMap[1] = nilableArgA // safe: nilableMap is deeply nilable already
		nilableMap[2] = nonnilArg
		mixedMapFromArg[0] = nilableArgA
		mixedMapFromArg[1] = nonnilArg
	case 2:
		if v, ok := nilableMap[0]; ok {
			mixedMapFromRead[0] = v // the deeply nilable map legitimately stores nil
		}
		if v, ok := nonnilMap[0]; ok {
			mixedMapFromRead[1] = v // safe: nonnilMap only ever stores nonnil values
		}
	case 3:
		if v, ok := mixedMapParam[0]; ok {
			print(*v) //want "assigned deeply into parameter arg `mixedMapParam`"
		}
	case 4:
		mixedMapParam[0] = nilableArgB
		mixedMapParam[1] = nonnilArg
	}
}

// paramMixedMap backs mixedMapParam above; it is otherwise unused so that the conflict counts of
// the mixed maps stay independent.
var paramMixedMap = map[int]*int{}

func callTestMapDeepWrites(i int) {
	testMapDeepWrites(paramMixedMap, nil, nil, &i)
}

// The callTestOk* consumers below are declared before the functions they consume: NilAway
// determines each inference site once, and only observations opposing an already-determined site
// create conflicts. With the consuming dereference processed first (source order), every unsafe
// return in the function that follows is reported individually at that dereference.
//
// The rich-bool (`v, ok := m[k]`) checks are split into four scenario functions — unguarded /
// wrong guard / guard scope / guard invalidation — per map source (parameters, function results,
// globals, locals), so that each dereference line carries only the diagnostics of its own
// scenario. In all of them, a value read from the deeply nilable map is never safe (`ok` only
// means the key is present, and the map legitimately stores nil), while a value read from the
// never-nil map is safe exactly when guarded by its own `ok`.

func callTestOkUnguardedParams() {
	print(*testOkUnguardedParams(nilableMap, nonnilMap)) //want "returned from `testOkUnguardedParams.*` in position 0" "returned from `testOkUnguardedParams.*` in position 0"
}

// testOkUnguardedParams checks that a value read in the `v, ok := m[k]` form is nilable when
// returned without checking `ok`, regardless of the map's deep nilability: a read of a missing
// key yields the zero value.
func testOkUnguardedParams(deepNilableMapParam, deepNonnilMapParam map[int]*int) *int {
	vNonnil, okNonnil := deepNonnilMapParam[0]
	vNilable, okNilable := deepNilableMapParam[0]

	if dummy {
		return vNonnil
	}
	if dummy {
		return vNilable
	}

	if okNonnil || okNilable {
		// no-op: the `ok`s are deliberately left unchecked before the returns above
	}

	i := 0
	return &i
}

func callTestOkWrongGuardParams() {
	print(*testOkWrongGuardParams(nilableMap, nonnilMap)) //want "returned from `testOkWrongGuardParams.*` in position 0" "returned from `testOkWrongGuardParams.*` in position 0" "returned from `testOkWrongGuardParams.*` in position 0"
}

// testOkWrongGuardParams checks which `ok` guard protects which value: only the value's own `ok`
// establishes anything, and even that is not enough for a value read from the deeply nilable map.
func testOkWrongGuardParams(deepNilableMapParam, deepNonnilMapParam map[int]*int) *int {
	vNonnil, okNonnil := deepNonnilMapParam[0]
	vNilable, okNilable := deepNilableMapParam[0]

	if okNonnil {
		if dummy {
			return vNonnil // safe: matching guard on a value from the never-nil map
		}
		if dummy {
			return vNilable // the guard matches, but the map is deeply nilable
		}
	}

	if okNilable {
		if dummy {
			return vNonnil // guarded by the wrong `ok`
		}
		if dummy {
			return vNilable // the map is deeply nilable
		}
	}

	i := 0
	return &i
}

func callTestOkGuardScopeParams() {
	print(*testOkGuardScopeParams(nilableMap, nonnilMap)) //want "returned from `testOkGuardScopeParams.*` in position 0" "returned from `testOkGuardScopeParams.*` in position 0"
}

// testOkGuardScopeParams checks that an `ok` guard only protects returns inside its own block:
// the return that is safe inside the guarded block is unsafe when repeated after it.
func testOkGuardScopeParams(deepNilableMapParam, deepNonnilMapParam map[int]*int) *int {
	vNonnil, okNonnil := deepNonnilMapParam[0]
	vNilable, okNilable := deepNilableMapParam[0]

	if okNonnil {
		if dummy {
			return vNonnil // safe: inside the guarded block
		}
	}
	if okNilable {
		// no-op: this block only exists so that the returns below sit after both guard blocks
	}

	if dummy {
		return vNonnil // the guard above does not extend past its block
	}
	return vNilable
}

func callTestOkGuardInvalidationParams() {
	print(*testOkGuardInvalidationParams(nonnilMap)) //want "returned from `testOkGuardInvalidationParams.*` in position 0" "returned from `testOkGuardInvalidationParams.*` in position 0" "returned from `testOkGuardInvalidationParams.*` in position 0"
}

// testOkGuardInvalidationParams checks that assignments to the rich bool (or to the read value)
// invalidate the `ok` guard, while no-op branching leaves it intact.
func testOkGuardInvalidationParams(deepNonnilMapParam map[int]*int) *int {
	vNonnil, okNonnil := deepNonnilMapParam[0]

	switch 0 {
	case 1:
		okNonnil = true

		if okNonnil {
			// this case tests that assignments to the rich bool invalidate the check properly
			return vNonnil
		}
	case 2:
		switch 0 {
		case 1:
		case 2:
		case 3:
			okNonnil = true
		}

		if okNonnil {
			// this case is similar to above, but tests that assignments in branching of degree
			// greater than 2 is still handled properly
			return vNonnil
		}
	case 3:
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, okNonnil = deepNonnilMapParam[0]
		}

		if okNonnil {
			// this case is similar to above, but tests an identical re-assignment
			// of vNonNil and okNonNil
			return vNonnil
		}
	case 4:
		var ok2Nonnil bool
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, ok2Nonnil = deepNonnilMapParam[0]
		}

		if okNonnil {
			// this case is similar to above, but tests a non-identical re-assignment
			// of vNonNil to make sure the check is invalidated
			return vNonnil
		}

		if ok2Nonnil {
			// without this ok2Nonnil is unused and throws a static error
		}
	case 5:
		switch 0 {
		case 1:
		case 2:
		case 3:
		}

		if okNonnil {
			// this case is similar to above, but the 3-way switch is all no-ops, so
			// the rich bool should still be in place
			return vNonnil
		}
	}

	i := 0
	return &i
}

// retsDeepNilableMap returns a deeply nilable map (the global that feedMaps stores nil into),
// while retsDeepNonnilMap returns a fresh map that never stores nil.
func retsDeepNilableMap() map[int]*int {
	return nilableMap
}

func retsDeepNonnilMap() map[int]*int {
	return make(map[int]*int)
}

// The four scenario functions below mirror the *Params variants, with the maps coming from
// function results instead of parameters.

func callTestOkUnguardedResults() {
	print(*testOkUnguardedResults()) //want "returned from `testOkUnguardedResults.*` in position 0" "returned from `testOkUnguardedResults.*` in position 0"
}

func testOkUnguardedResults() *int {
	vNonnil, okNonnil := retsDeepNonnilMap()[0]
	vNilable, okNilable := retsDeepNilableMap()[0]

	if dummy {
		return vNonnil
	}
	if dummy {
		return vNilable
	}

	if okNonnil || okNilable {
		// no-op: the `ok`s are deliberately left unchecked before the returns above
	}

	i := 0
	return &i
}

func callTestOkWrongGuardResults() {
	print(*testOkWrongGuardResults()) //want "returned from `testOkWrongGuardResults.*` in position 0" "returned from `testOkWrongGuardResults.*` in position 0" "returned from `testOkWrongGuardResults.*` in position 0"
}

func testOkWrongGuardResults() *int {
	vNonnil, okNonnil := retsDeepNonnilMap()[0]
	vNilable, okNilable := retsDeepNilableMap()[0]

	if okNonnil {
		if dummy {
			return vNonnil // safe: matching guard on a value from the never-nil map
		}
		if dummy {
			return vNilable // the guard matches, but the map is deeply nilable
		}
	}

	if okNilable {
		if dummy {
			return vNonnil // guarded by the wrong `ok`
		}
		if dummy {
			return vNilable // the map is deeply nilable
		}
	}

	i := 0
	return &i
}

func callTestOkGuardScopeResults() {
	print(*testOkGuardScopeResults()) //want "returned from `testOkGuardScopeResults.*` in position 0" "returned from `testOkGuardScopeResults.*` in position 0"
}

func testOkGuardScopeResults() *int {
	vNonnil, okNonnil := retsDeepNonnilMap()[0]
	vNilable, okNilable := retsDeepNilableMap()[0]

	if okNonnil {
		if dummy {
			return vNonnil // safe: inside the guarded block
		}
	}
	if okNilable {
		// no-op: this block only exists so that the returns below sit after both guard blocks
	}

	if dummy {
		return vNonnil // the guard above does not extend past its block
	}
	return vNilable
}

func callTestOkGuardInvalidationResults() {
	print(*testOkGuardInvalidationResults()) //want "returned from `testOkGuardInvalidationResults.*` in position 0" "returned from `testOkGuardInvalidationResults.*` in position 0" "returned from `testOkGuardInvalidationResults.*` in position 0"
}

func testOkGuardInvalidationResults() *int {
	vNonnil, okNonnil := retsDeepNonnilMap()[0]

	switch 0 {
	case 1:
		okNonnil = true

		if okNonnil {
			// this case tests that assignments to the rich bool invalidate the check properly
			return vNonnil
		}
	case 2:
		switch 0 {
		case 1:
		case 2:
		case 3:
			okNonnil = true
		}

		if okNonnil {
			// this case is similar to above, but tests that assignments in branching of degree
			// greater than 2 is still handled properly
			return vNonnil
		}
	case 3:
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, okNonnil = retsDeepNonnilMap()[0]
		}

		if okNonnil {
			// this case is similar to above, but tests an identical re-assignment
			// of vNonNil and okNonNil
			return vNonnil
		}
	case 4:
		var ok2Nonnil bool
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, ok2Nonnil = retsDeepNonnilMap()[0]
		}

		if okNonnil {
			// this case is similar to above, but tests a non-identical re-assignment
			// of vNonNil to make sure the check is invalidated
			return vNonnil
		}

		if ok2Nonnil {
			// without this ok2Nonnil is unused and throws a static error
		}
	case 5:
		switch 0 {
		case 1:
		case 2:
		case 3:
		}

		if okNonnil {
			// this case is similar to above, but the 3-way switch is all no-ops, so
			// the rich bool should still be in place
			return vNonnil
		}
	}

	i := 0
	return &i
}

// The four scenario functions below mirror the *Params variants, with the maps being the globals
// themselves.

func callTestOkUnguardedGlobals() {
	print(*testOkUnguardedGlobals()) //want "returned from `testOkUnguardedGlobals.*` in position 0" "returned from `testOkUnguardedGlobals.*` in position 0"
}

func testOkUnguardedGlobals() *int {
	vNonnil, okNonnil := nonnilMap[0]
	vNilable, okNilable := nilableMap[0]

	if dummy {
		return vNonnil
	}
	if dummy {
		return vNilable
	}

	if okNonnil || okNilable {
		// no-op: the `ok`s are deliberately left unchecked before the returns above
	}

	i := 0
	return &i
}

func callTestOkWrongGuardGlobals() {
	print(*testOkWrongGuardGlobals()) //want "returned from `testOkWrongGuardGlobals.*` in position 0" "returned from `testOkWrongGuardGlobals.*` in position 0" "returned from `testOkWrongGuardGlobals.*` in position 0"
}

func testOkWrongGuardGlobals() *int {
	vNonnil, okNonnil := nonnilMap[0]
	vNilable, okNilable := nilableMap[0]

	if okNonnil {
		if dummy {
			return vNonnil // safe: matching guard on a value from the never-nil map
		}
		if dummy {
			return vNilable // the guard matches, but the map is deeply nilable
		}
	}

	if okNilable {
		if dummy {
			return vNonnil // guarded by the wrong `ok`
		}
		if dummy {
			return vNilable // the map is deeply nilable
		}
	}

	i := 0
	return &i
}

func callTestOkGuardScopeGlobals() {
	print(*testOkGuardScopeGlobals()) //want "returned from `testOkGuardScopeGlobals.*` in position 0" "returned from `testOkGuardScopeGlobals.*` in position 0"
}

func testOkGuardScopeGlobals() *int {
	vNonnil, okNonnil := nonnilMap[0]
	vNilable, okNilable := nilableMap[0]

	if okNonnil {
		if dummy {
			return vNonnil // safe: inside the guarded block
		}
	}
	if okNilable {
		// no-op: this block only exists so that the returns below sit after both guard blocks
	}

	if dummy {
		return vNonnil // the guard above does not extend past its block
	}
	return vNilable
}

func callTestOkGuardInvalidationGlobals() {
	print(*testOkGuardInvalidationGlobals()) //want "returned from `testOkGuardInvalidationGlobals.*` in position 0" "returned from `testOkGuardInvalidationGlobals.*` in position 0" "returned from `testOkGuardInvalidationGlobals.*` in position 0"
}

func testOkGuardInvalidationGlobals() *int {
	vNonnil, okNonnil := nonnilMap[0]

	switch 0 {
	case 1:
		okNonnil = true

		if okNonnil {
			// this case tests that assignments to the rich bool invalidate the check properly
			return vNonnil
		}
	case 2:
		switch 0 {
		case 1:
		case 2:
		case 3:
			okNonnil = true
		}

		if okNonnil {
			// this case is similar to above, but tests that assignments in branching of degree
			// greater than 2 is still handled properly
			return vNonnil
		}
	case 3:
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, okNonnil = nonnilMap[0]
		}

		if okNonnil {
			// this case is similar to above, but tests an identical re-assignment
			// of vNonNil and okNonNil
			return vNonnil
		}
	case 4:
		var ok2Nonnil bool
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, ok2Nonnil = nonnilMap[0]
		}

		if okNonnil {
			// this case is similar to above, but tests a non-identical re-assignment
			// of vNonNil to make sure the check is invalidated
			return vNonnil
		}

		if ok2Nonnil {
			// without this ok2Nonnil is unused and throws a static error
		}
	case 5:
		switch 0 {
		case 1:
		case 2:
		case 3:
		}

		if okNonnil {
			// this case is similar to above, but the 3-way switch is all no-ops, so
			// the rich bool should still be in place
			return vNonnil
		}
	}

	i := 0
	return &i
}

// The three scenario functions below mirror the *Params variants, with the map being a local.
// A local map made with make() never stores nil here, so only the never-nil-map scenarios apply.

func callTestOkUnguardedLocals() {
	print(*testOkUnguardedLocals()) //want "returned from `testOkUnguardedLocals.*` in position 0"
}

func testOkUnguardedLocals() *int {
	deepNonnilMap := make(map[int]*int)
	vNonnil, okNonnil := deepNonnilMap[0]

	if dummy {
		return vNonnil
	}

	if okNonnil {
		// no-op: the `ok` is deliberately left unchecked before the return above
	}

	i := 0
	return &i
}

func callTestOkGuardScopeLocals() {
	print(*testOkGuardScopeLocals()) //want "returned from `testOkGuardScopeLocals.*` in position 0"
}

func testOkGuardScopeLocals() *int {
	deepNonnilMap := make(map[int]*int)
	vNonnil, okNonnil := deepNonnilMap[0]

	if okNonnil {
		if dummy {
			return vNonnil // safe: inside the guarded block
		}
	}

	if dummy {
		return vNonnil // the guard above does not extend past its block
	}

	i := 0
	return &i
}

func callTestOkGuardInvalidationLocals() {
	print(*testOkGuardInvalidationLocals()) //want "returned from `testOkGuardInvalidationLocals.*` in position 0" "returned from `testOkGuardInvalidationLocals.*` in position 0" "returned from `testOkGuardInvalidationLocals.*` in position 0"
}

func testOkGuardInvalidationLocals() *int {
	deepNonnilMap := make(map[int]*int)
	vNonnil, okNonnil := deepNonnilMap[0]

	switch 0 {
	case 1:
		okNonnil = true

		if okNonnil {
			// this case tests that assignments to the rich bool invalidate the check properly
			return vNonnil
		}
	case 2:
		switch 0 {
		case 1:
		case 2:
		case 3:
			okNonnil = true
		}

		if okNonnil {
			// this case is similar to above, but tests that assignments in branching of degree
			// greater than 2 is still handled properly
			return vNonnil
		}
	case 3:
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, okNonnil = deepNonnilMap[0]
		}

		if okNonnil {
			// this case is similar to above, but tests an identical re-assignment
			// of vNonNil and okNonNil
			return vNonnil
		}
	case 4:
		var ok2Nonnil bool
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, ok2Nonnil = deepNonnilMap[0]
		}

		if okNonnil {
			// this case is similar to above, but tests a non-identical re-assignment
			// of vNonNil to make sure the check is invalidated
			return vNonnil
		}

		if ok2Nonnil {
			// without this ok2Nonnil is unused and throws a static error
		}
	case 5:
		switch 0 {
		case 1:
		case 2:
		case 3:
		}

		if okNonnil {
			// this case is similar to above, but the 3-way switch is all no-ops, so
			// the rich bool should still be in place
			return vNonnil
		}
	}

	i := 0
	return &i
}

// The singleKey* scenarios below check what the `v, ok := m[k]` rich bool establishes for the
// read value v and for the map operand m itself. Each scenario is a separate function with its
// own map read and its own nil-fed parameter: once a nilable site is determined via its first
// conflicting flow, further edges from it are discarded as redundant, and dereferences sharing a
// single read are grouped under one diagnostic — per-scenario reads and sources keep every want
// on its own line. Consumption is in place (a dereference of v, an index write to m), except for
// singleKeyUnguarded, which routes v through the generic helper below to retain argument-pass
// (and generic-instantiation) consumption coverage.

func takesNonnilPtr[T any](x *T) {
	_ = *x //want "passed as arg"
}

// singleKeyUnguarded checks that without consulting `ok`, neither the read value nor the map
// operand may be consumed as nonnil.
func singleKeyUnguarded(m map[int]*int) {
	v, ok := m[0]

	takesNonnilPtr(v)
	m[1] = new(int) //want "written to at an index"

	_ = ok
}

// singleKeyGuarded checks that the `ok` guard establishes BOTH the read value and the map operand
// as nonnil: the key being present implies the map itself is nonnil.
func singleKeyGuarded(m map[int]*int) {
	v, ok := m[0]

	if !ok {
		return
	}

	print(*v)
	m[1] = new(int)
}

// singleKeyInvalidatedOk checks that overwriting `ok` invalidates the guard for both the read
// value and the map operand.
func singleKeyInvalidatedOk(m map[int]*int) {
	v, ok := m[0]

	ok = true

	if !ok {
		return
	}

	print(*v)       //want "dereferenced"
	m[1] = new(int) //want "written to at an index"
}

// singleKeyInvalidatedValue checks that overwriting the read value invalidates the guard for JUST
// the value; the map operand stays established.
func singleKeyInvalidatedValue(m map[int]*int) {
	v, ok := m[0]

	v = nil

	if !ok {
		return
	}

	print(*v) //want "dereferenced"
	m[1] = new(int)
}

// singleKeyInvalidatedMap checks that overwriting the map operand invalidates the guard for JUST
// the map; the read value stays established.
func singleKeyInvalidatedMap(m map[int]*int) {
	v, ok := m[0]

	m = nil

	if !ok {
		return
	}

	print(*v)
	m[1] = new(int) //want "written to at an index"
}

func callSingleKeys() {
	singleKeyUnguarded(nil)
	singleKeyGuarded(nil)
	singleKeyInvalidatedOk(nil)
	singleKeyInvalidatedValue(nil)
	singleKeyInvalidatedMap(make(map[int]*int))
}

// plainReflCheck exercises the rich-bool form on a `map[any]any`: the `ok` check establishes the
// map operand itself as nonnil for the return that it guards. The map must be a global: NilAway
// does not track the shallow nilability of a map-typed parameter into a `return` of that
// parameter, so a parameter-based variant would be silent. Note that only the first unguarded
// return is reported: both unguarded returns flow between the same pair of inference sites
// (reflMap and result 0), and NilAway deduplicates conflicts per site pair.
var reflMap map[any]any

func callPlainReflCheck() {
	r := plainReflCheck()
	r[0] = 0 //want "returned from `plainReflCheck.*` in position 0"
}

func plainReflCheck() map[any]any {
	if dummy {
		return reflMap
	}

	_, ok := reflMap[0]

	if ok {
		return reflMap // safe: `ok` establishes reflMap itself as nonnil
	}

	return reflMap // deduplicated against the first unguarded return above (same site pair)
}

// BELOW TESTS CHECK EXPLICIT BOOLEAN COMPARISONS WITH THE RICH BOOL (`ok == true` AND FRIENDS).
// The original single function is split by scenario so that each consuming caller carries only
// the diagnostics of its own comparisons. All flows here are guard-based (a read of a possibly
// missing key), so the maps need no nilable feeder; the callers pass fresh maps.

func callTestExplicitBoolSafeGuards() {
	print(*testExplicitBoolSafeGuards(make(map[int]*int), 0))
}

// testExplicitBoolSafeGuards groups the comparisons that correctly establish presence: no errors.
func testExplicitBoolSafeGuards(mp map[int]*int, i int) *int {
	switch i {
	case 0:
		if x, ok := mp[i]; ok == true {
			return x
		}
	case 1:
		if x, ok := mp[i]; ok != false {
			return x
		}
	case 2:
		if x, ok := mp[i]; true == ok {
			return x
		}
	case 3:
		if x, ok := mp[i]; false != ok {
			return x
		}
	case 4:
		var x *int
		var ok bool
		if x, ok = mp[0]; ok == false {
			x = &i
			mp[0] = x
		}
		return x
	}
	return &i
}

func callTestExplicitBoolNegatedTrue() {
	print(*testExplicitBoolNegatedTrue(make(map[int]*int), 0)) //want "returned from `testExplicitBoolNegatedTrue.*` in position 0" "returned from `testExplicitBoolNegatedTrue.*` in position 0"
}

// testExplicitBoolNegatedTrue groups the `!= true` comparisons: the guarded block is entered
// exactly when the key is missing.
func testExplicitBoolNegatedTrue(mp map[int]*int, i int) *int {
	switch i {
	case 0:
		if x, ok := mp[i]; ok != true {
			return x
		}
	case 1:
		if x, ok := mp[i]; true != ok {
			return x
		}
	}
	return &i
}

func callTestExplicitBoolFalseComparisons() {
	print(*testExplicitBoolFalseComparisons(make(map[int]*int), 0)) //want "returned from `testExplicitBoolFalseComparisons.*` in position 0" "returned from `testExplicitBoolFalseComparisons.*` in position 0"
}

// testExplicitBoolFalseComparisons groups comparisons against `false` and a deeply nested
// negation that reduces to `!ok`.
func testExplicitBoolFalseComparisons(mp map[int]*int, i int) *int {
	switch i {
	case 0:
		if x, ok := mp[i]; false == ok {
			return x
		}
	case 1:
		if x, ok := mp[i]; !(!(!(!(true != ok) || ok == true))) {
			return x
		}
	}
	return &i
}

func callTestExplicitBoolMultiKey() {
	print(*testExplicitBoolMultiKey(make(map[int]*int), 0)) //want "returned from `testExplicitBoolMultiKey.*` in position 0"
}

// testExplicitBoolMultiKey checks compound conditions over two rich bools: `&&` establishes both
// keys, while `||` establishes neither.
func testExplicitBoolMultiKey(mp map[int]*int, i int) *int {
	x, ok1 := mp[0]
	y, ok2 := mp[1]
	if ok1 == true && ok2 != false {
		return x // safe: both keys established
	}
	if ok1 == true || ok2 == true {
		return y // either key may be missing
	}
	return &i
}

func callTestExplicitBoolNonGuards() {
	print(*testExplicitBoolNonGuards(make(map[int]*int), 0)) //want "returned from `testExplicitBoolNonGuards.*` in position 0" "returned from `testExplicitBoolNonGuards.*` in position 0"
}

// testExplicitBoolNonGuards groups conditions that do not consult the rich bool at all (a
// tautology) or consult it in a disjunction that can be entered without it, so nothing is
// established.
func testExplicitBoolNonGuards(mp map[int]*int, i int) *int {
	switch i {
	case 0:
		if x, _ := mp[0]; true == true || true != false || false == false || false != true {
			return x
		}
	case 1:
		if x, ok := mp[i]; ok == true || i > 5 {
			return x
		}
	}
	return &i
}

// BELOW TESTS CHECK CONSECUTIVE ACCESSES OF THE SAME MAP KEY: AN `ok` CHECK ON `mp[k]` ALSO
// GUARDS A LATER `mp[k]` ACCESS. All flows are guard-based, split into a literal-key and a
// non-literal-key variant.

func callTestConsequentMapAccessesLiteral() {
	print(*testConsequentMapAccessesLiteral(make(map[int]*int), 0)) //want "returned from `testConsequentMapAccessesLiteral.*` in position 0" "returned from `testConsequentMapAccessesLiteral.*` in position 0" "returned from `testConsequentMapAccessesLiteral.*` in position 0"
}

func testConsequentMapAccessesLiteral(mp map[int]*int, i int) *int {
	switch i {
	case 0:
		if _, ok := mp[0]; !ok {
			mp[0] = new(int)
		}
		return mp[0] // safe: the key is either present or was just assigned

	case 1:
		if _, ok := mp[0]; ok {
			return mp[0] // safe: presence established by the check on the same key
		}

	case 2:
		if _, ok := mp[0]; !ok {
		}
		return mp[0] // the empty branch established nothing

	case 3:
		if _, ok := mp[0]; ok {
		}
		return mp[0] // the check does not guard accesses after its block

	case 4:
		v, ok := mp[0]
		v2, ok2 := mp[0]
		if ok && !ok2 {
			v2 = v
		}
		return v2 // both reads may have missed

	case 5:
		if v, ok := mp[0]; ok {
			if dummy {
				return v
			}
			return mp[0] // safe: same key, still under the check
		}

	case 6:
		const j = 0
		if _, ok := mp[j]; !ok {
			mp[j] = new(int)
		}
		return mp[j] // safe: constant keys are tracked like literals
	}
	return &i
}

func callTestConsequentMapAccessesNonLiteral() {
	print(*testConsequentMapAccessesNonLiteral(make(map[int]*int), 0)) //want "returned from `testConsequentMapAccessesNonLiteral.*` in position 0" "returned from `testConsequentMapAccessesNonLiteral.*` in position 0" "returned from `testConsequentMapAccessesNonLiteral.*` in position 0"
}

func testConsequentMapAccessesNonLiteral(mp map[int]*int, i int) *int {
	switch i {
	case 0:
		if _, ok := mp[i]; !ok {
			mp[i] = new(int)
		}
		return mp[i] // safe: the key is either present or was just assigned

	case 1:
		if _, ok := mp[i]; ok {
			return mp[i] // safe: presence established by the check on the same key
		}

	case 2:
		if _, ok := mp[i]; !ok {
		}
		return mp[i] // the empty branch established nothing

	case 3:
		if _, ok := mp[i]; ok {
		}
		return mp[i] // the check does not guard accesses after its block

	case 4:
		v, ok := mp[i]
		v2, ok2 := mp[i]
		if ok && !ok2 {
			v2 = v
		}
		return v2 // both reads may have missed

	case 5:
		if v, ok := mp[i]; ok {
			if dummy {
				return v
			}
			return mp[i] // safe: same key, still under the check
		}
	}
	return &i
}

// Below tests check the behavior in presence of two rich check effects: ok-returning function,
// and map access. We should be able to handle both correctly.

type S struct {
	m map[string]*int
}

func retPtrBool() (*S, bool) {
	if dummy {
		return &S{m: make(map[string]*int)}, true
	}
	return nil, false
}

func callTestMixedRichCheckEffects() {
	print(*testMixedRichCheckEffects(0)) //want "returned from `testMixedRichCheckEffects.*` in position 0" "returned from `testMixedRichCheckEffects.*` in position 0"
}

func testMixedRichCheckEffects(i int) *int {
	switch i {
	case 0:
		// Here the ok-returning function is correctly guarded, but not the map access, for which error should be reported.
		s, ok := retPtrBool()
		if !ok {
			return new(int)
		}
		return s.m["abc"]

	case 1:
		// Here the map access is correctly guarded, but not the ok-returning function, for which error should be reported.
		s, _ := retPtrBool()
		if v, ok := s.m["abc"]; ok { //want "accessed field"
			return v
		}

	case 2:
		// Here both the ok-returning function and the map access are not guarded, so error should be reported for both.
		s, ok := retPtrBool()
		_ = ok
		return s.m["abc"] //want "accessed field"

	case 3:
		// Here both the ok-returning function and the map access are correctly guarded, so no error should be reported.
		s, ok := retPtrBool()
		if !ok {
			return new(int)
		}
		if v, ok := s.m["abc"]; ok {
			return v
		}

	case 4:
		// This test case checks the behavior with consequent map accesses.
		// Here both the ok-returning function and the map access are correctly guarded, so no error should be reported.
		s, ok := retPtrBool()
		if !ok {
			return new(int)
		}
		if _, ok := s.m["abc"]; !ok {
			s.m["abc"] = new(int)
		}
		return s.m["abc"]
	}
	return &i
}

// tests for checking non-literal map accesses

func retInt() int {
	return 0
}

type A struct {
	f int
	g int
}

type mapType map[string][]*string

func testNonLiteralMapAccess(mp map[int]*int, i, j int) {
	switch i {
	case 0:
		if mp[i] != nil {
			print(*mp[i])
		}

	case 1:
		if mp[i] == nil {
			return
		}
		print(*mp[i])

	case 3:
		if mp[i] != nil {
			i := 10
			print(*mp[i]) //want "lacking guarding"
		}

	case 4:
		if mp[i] != nil {
			print(*mp[j]) //want "lacking guarding"
		}

	case 5:
		localVar := 0
		if mp[localVar] != nil {
			print(mp[localVar])
		}

	case 6:
		a := &A{}
		if mp[a.f] != nil {
			print(*mp[a.f])
		}

	case 7:
		a1 := &A{}
		a2 := &A{}
		if mp[a1.f] != nil {
			print(*mp[a2.f]) //want "lacking guarding"
		}

	case 8:
		a := &A{}
		if mp[a.f] != nil {
			print(*mp[a.g]) //want "lacking guarding"
		}

	case 9:
		var sl []*int
		if mp[len(sl)-1] != nil {
			print(*mp[len(sl)-1])
		}

	case 10:
		// NilAway does not consider user-defined functions as stable, and hence reports an error here. It could be
		// considered a false positive from a user perspective, but NilAway cannot guarantee the stability of the function
		// without a more complex analysis. We are currently not choosing to do this since we believe this to be a rare
		// case and also an anti-pattern since users should ideally create a local variable and use that instead.
		if mp[retInt()] != nil {
			print(*mp[retInt()]) //want "lacking guarding"
		}

		localVar := retInt()
		if mp[localVar] != nil {
			print(*mp[localVar])
		}

	case 11:
		// TODO: This case is currently a false negative since NilAway does not track the value of integers (`i`).
		//  However, this is not expected to be a common pattern, hence we plan to add support for this in a follow-up PR.
		i = 0
		if mp[i] != nil {
			i = 100
			print(*mp[i]) // TODO: report error here
		}

	case 12:
		// TODO: Similar as above, this case is currently a false negative since NilAway does not track the value of integers (`i`).
		//  However, this is not expected to be a common pattern, hence we plan to add support for this in a follow-up PR.
		i = len(mp) - 1
		if mp[i] != nil {
			i = len(mp)
			print(*mp[i]) // TODO: report error here
		}

	case 13:
		// TODO: Similar as above, this case is currently a false negative since NilAway does not track the value of integers (`i`).
		//  However, this is not expected to be a common pattern, hence we plan to add support for this in a follow-up PR.
		a := &A{}
		i = a.f
		if mp[i] != nil {
			i = a.g
			print(*mp[i]) // TODO: report error here
		}

	case 14:
		// test case for checking with map type
		m := mapType{}
		key := "key"
		vs := m[key]
		if len(vs) == 0 {
			return
		}
		print(*vs[0])

	case 15:
		var v uint8
		m := make(map[string]*int)
		if m[string(v)] != nil {
			_ = *m[string(v)]
		}

		if v, ok := m[string(v)]; ok {
			_ = *v
		}

	case 16:
		var v uint8
		m := make(map[string]*int)
		_ = *m[string(v)] //want "lacking guarding"
	}
}

type Node struct {
	children map[rune]*Node
}

func testNestedMaps(mapOfMap map[string]map[string]*int, mapOfmapOfMap map[string]map[string]map[string]*int, root *Node, i int) {
	k1, k2, k3 := "key1", "key2", "key3"

	switch i {
	case 0:
		if _, ok := mapOfMap[k1]; !ok {
			mapOfMap[k1] = map[string]*int{}
		}
		mapOfMap[k1][k2] = new(int)

	case 1:
		// same as case 0, but with for loop
		for _, s := range []string{"a", "b", "c"} {
			if _, ok := mapOfMap[s]; !ok {
				mapOfMap[s] = map[string]*int{}
			}
			mapOfMap[s][k2] = new(int)
		}

	case 2:
		if mapOfmapOfMap[k1] == nil {
			mapOfmapOfMap[k1] = make(map[string]map[string]*int)
		}
		if _, ok := mapOfmapOfMap[k1][k2]; !ok {
			mapOfmapOfMap[k1][k2] = make(map[string]*int)
		}
		mapOfmapOfMap[k1][k2][k3] = new(int)

	case 3:
		// test case simulated from issue #84
		for _, s := range []string{"a", "b", "c"} {
			if mapOfmapOfMap[s] == nil {
				mapOfmapOfMap[s] = make(map[string]map[string]*int)
			}
			for _, t := range []string{"x", "y", "z"} {
				if _, ok := mapOfmapOfMap[s][t]; !ok {
					mapOfmapOfMap[s][t] = make(map[string]*int)
				}
				mapOfmapOfMap[s][t][k3] = new(int)
			}
		}

	case 4:
		if _, ok := mapOfMap[k1]; !ok {
		}
		mapOfMap[k1][k2] = new(int) //want "lacking guarding"

	case 5:
		if _, ok := mapOfmapOfMap[k1][k2]; !ok {
			mapOfmapOfMap[k1][k2] = make(map[string]*int) //want "lacking guarding"
		}
		mapOfmapOfMap[k1][k2][k3] = new(int)

	case 6:
		// test case simulated from issue #206
		if root == nil {
			return
		}
		current := root
		for _, v := range k1 {
			if current.children == nil {
				current.children = make(map[rune]*Node)
			}
			if current.children[v] == nil {
				current.children[v] = &Node{
					children: make(map[rune]*Node),
				}
			}

			current = current.children[v]
		}
	}
}
