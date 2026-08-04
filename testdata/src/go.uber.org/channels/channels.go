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

// Package channels tests that nilaway properly handles channels.
package channels

// nilableChan is deeply nilable purely by inference: feedChans below sends nil into it.
// nonNilChan only ever receives nonnil values, so its deep nilability stays unconstrained
// and reads from it can be safely consumed.
var nilableChan = make(chan *int)
var nonNilChan = make(chan *int)

// nilableChanDirect, nilableChanRange, and nilableChanRangeArg are dedicated to the in-place
// dereferences in testChanRecvs: in-place dereferences that share a nil source are grouped into
// a single diagnostic (and a parameter site only supports one conflicting flow), so each in-place
// consumer gets its own deeply nilable channel.
var nilableChanDirect = make(chan *int)
var nilableChanRange = make(chan *int)
var nilableChanRangeArg = make(chan *int)

func feedChans(i int) {
	nilableChan <- nil
	nonNilChan <- &i
	nilableChanDirect <- nil
	nilableChanRange <- nil
	nilableChanRangeArg <- nil
}

// takesNonnilRecv keeps one unsafe receive consumed through an argument pass; the other unsafe
// receives in testChanRecvs are dereferenced in place so that each diagnostic appears right at
// the case that exercises it.
func takesNonnilRecv(v *int) {
	_ = *v //want "dereferenced"
}

// testChanRecvs checks the receive matrix: values received (directly or via range) from a deeply
// nilable channel must not be consumed as nonnil, while values from a channel that never carries
// nil are safe. The channel parameters are backed by the globals at the call site below.
func testChanRecvs(nilableChanArg, rangeNilableChanArg, nonNilChanArg chan *int) {
	switch 0 {
	case 1:
		takesNonnilRecv(<-nilableChanArg)
	case 2:
		takesNonnilRecv(<-nonNilChanArg)
	case 3:
		print(*<-nilableChanDirect) //want "assigned deeply into global variable `nilableChanDirect`"
	case 4:
		print(*<-nonNilChan)
	case 5:
		for v := range rangeNilableChanArg {
			print(*v) //want "passed deeply as arg `rangeNilableChanArg`"
		}
	case 6:
		for v := range nonNilChanArg {
			print(*v)
		}
	case 7:
		for v := range nilableChanRange {
			print(*v) //want "assigned deeply into global variable `nilableChanRange`"
		}
	case 8:
		for v := range nonNilChan {
			print(*v)
		}
	}
}

func callTestChanRecvs() {
	testChanRecvs(nilableChan, nilableChanRangeArg, nonNilChan)
}

// The mixed* channels below have their reads dereferenced but receive exactly one nilable send
// each: once a site is determined nilable, further nilable sends into it agree with the existing
// value and do not create additional conflicts, so a one-send-one-deref pairing keeps every
// diagnostic attributable to a specific send.
var mixedChanFromArg = make(chan *int)
var mixedChanFromRecv = make(chan *int)

func drainMixedChanFromArg() {
	print(*<-mixedChanFromArg) //want "assigned deeply into global variable `mixedChanFromArg`"
}

func drainMixedChanFromRecv() {
	print(*<-mixedChanFromRecv) //want "assigned deeply into global variable `mixedChanFromRecv`"
}

// testChanSends checks that sending nilable values into channels whose reads are consumed as
// nonnil is reported, while sends of nonnil values (or sends into the deeply nilable channel)
// are safe. The two unsafe global sends use different nilable sources (a nil argument and a read
// from the deeply nilable channel); the unsafe send into the parameter conflicts with the
// in-function dereference in case 4. Note that nilableArgA and nilableArgB must be separate
// parameters: once a nilable pointer site is determined via its first conflicting flow, edges
// from that site into other channels are discarded as redundant, so a shared parameter would
// yield only one conflict.
func testChanSends(mixedChanArg chan *int, nilableArgA, nilableArgB, nonNilArg *int) {
	switch 0 {
	case 1:
		nilableChan <- nilableArgA
		nilableChan <- nonNilArg
		mixedChanFromArg <- nilableArgA
		mixedChanFromArg <- nonNilArg
	case 2:
		nilableChan <- <-nilableChan
		mixedChanFromRecv <- <-nilableChan
		mixedChanFromRecv <- <-nonNilChan
	case 3:
		mixedChanArg <- nilableArgB
		mixedChanArg <- nonNilArg
		mixedChanArg <- <-nonNilChan
	case 4:
		print(*<-mixedChanArg) //want "assigned deeply into parameter arg `mixedChanArg`"
	}
}

// paramMixedChan backs mixedChanArg above; it is otherwise unused so that the conflict counts of
// the mixed channels stay independent.
var paramMixedChan = make(chan *int)

func callTestChanSends(i int) {
	testChanSends(paramMixedChan, nil, nil, &i)
}

// The T struct and I interface below exercise directional (send-only/receive-only) channels as
// struct fields and interface method results. NilAway does not currently infer deep nilability
// through struct fields (struct field tracking is suppressed until object sensitivity is added,
// issue #339) or through interface method results for channels, and shallow channel nilability
// has no panicking consumer to infer from (send/receive on a nil channel block instead of
// panicking, and close() is not modeled). These functions are therefore retained purely as
// negative coverage: NilAway must process them without reporting false positives.
type T struct {
	sendOnly        chan<- *int
	recvOnly        <-chan *int
	sendOnlyNilable chan<- *int
	recvOnlyNilable <-chan *int
	nilable         *int
	nonnil          *int
}

func testRestrictedChans(t T) {
	t.sendOnly <- t.nilable
	t.sendOnlyNilable <- t.nilable
	t.sendOnly <- t.nonnil
	t.sendOnlyNilable <- t.nonnil

	t.nilable = <-t.recvOnly
	t.nilable = <-t.recvOnlyNilable
	t.nonnil = <-t.recvOnly
	t.nonnil = <-t.recvOnlyNilable
}

type I interface {
	retsChans() (
		sendOnly chan<- *int,
		recvOnly <-chan *int,
		sendOnlyNilable chan<- *int,
		recvOnlyNilable <-chan *int)

	retsSendOnly() chan<- *int
	retsRecvOnly() <-chan *int
	retsSendOnlyNilable() chan<- *int
	retsRecvOnlyNilable() <-chan *int
}

func testRets(t T, i I) {
	i.retsSendOnly() <- t.nilable
	i.retsSendOnlyNilable() <- t.nilable
	i.retsSendOnly() <- t.nonnil
	i.retsSendOnlyNilable() <- t.nonnil

	t.nilable = <-i.retsRecvOnly()
	t.nilable = <-i.retsRecvOnlyNilable()
	t.nonnil = <-i.retsRecvOnly()
	t.nonnil = <-i.retsRecvOnlyNilable()
}

func testIndirectRets(t T, i I) {
	sendOnly, recvOnly, sendOnlyNilable, recvOnlyNilable := i.retsChans()

	sendOnly <- t.nilable
	sendOnlyNilable <- t.nilable
	sendOnly <- t.nonnil
	sendOnlyNilable <- t.nonnil

	t.nilable = <-recvOnly
	t.nilable = <-recvOnlyNilable
	t.nonnil = <-recvOnly
	t.nonnil = <-recvOnlyNilable
}

var dummy bool

// The callTestOk* consumers below are declared before the functions they consume: NilAway
// determines each inference site once, and only observations opposing an already-determined site
// create conflicts. With the consuming dereference processed first (source order), every unsafe
// return in the function that follows is reported individually at that dereference.
//
// The rich-bool (`v, ok := <-ch`) checks are split into four scenario functions — unguarded /
// wrong guard / guard scope / guard invalidation — per channel source (parameters, function
// results, globals), so that each dereference line carries only the diagnostics of its own
// scenario. In all of them, a value from the deeply nilable channel is never safe (`ok` only
// means the channel is open, and the channel legitimately carries nil), while a value from the
// never-nil channel is safe exactly when guarded by its own `ok`.

func callTestOkUnguardedParams() {
	print(*testOkUnguardedParams(nilableChan, nonNilChan)) //want "returned from `testOkUnguardedParams.*` in position 0" "returned from `testOkUnguardedParams.*` in position 0"
}

// testOkUnguardedParams checks that a value received in the `v, ok := <-ch` form is nilable when
// returned without checking `ok`, regardless of the channel's deep nilability: a receive from a
// closed channel yields the zero value.
func testOkUnguardedParams(nilableChanParam, nonnilChanParam chan *int) *int {
	vNonnil, okNonnil := <-nonnilChanParam
	vNilable, okNilable := <-nilableChanParam

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
	print(*testOkWrongGuardParams(nilableChan, nonNilChan)) //want "returned from `testOkWrongGuardParams.*` in position 0" "returned from `testOkWrongGuardParams.*` in position 0" "returned from `testOkWrongGuardParams.*` in position 0"
}

// testOkWrongGuardParams checks which `ok` guard protects which value: only the value's own `ok`
// establishes anything, and even that is not enough for a value from the deeply nilable channel.
func testOkWrongGuardParams(nilableChanParam, nonnilChanParam chan *int) *int {
	vNonnil, okNonnil := <-nonnilChanParam
	vNilable, okNilable := <-nilableChanParam

	if okNonnil {
		if dummy {
			return vNonnil // safe: matching guard on a value from the never-nil channel
		}
		if dummy {
			return vNilable // the guard matches, but the channel is deeply nilable
		}
	}

	if okNilable {
		if dummy {
			return vNonnil // guarded by the wrong `ok`
		}
		if dummy {
			return vNilable // the channel is deeply nilable
		}
	}

	i := 0
	return &i
}

func callTestOkGuardScopeParams() {
	print(*testOkGuardScopeParams(nilableChan, nonNilChan)) //want "returned from `testOkGuardScopeParams.*` in position 0" "returned from `testOkGuardScopeParams.*` in position 0"
}

// testOkGuardScopeParams checks that an `ok` guard only protects returns inside its own block:
// the return that is safe inside the guarded block is unsafe when repeated after it.
func testOkGuardScopeParams(nilableChanParam, nonnilChanParam chan *int) *int {
	vNonnil, okNonnil := <-nonnilChanParam
	vNilable, okNilable := <-nilableChanParam

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
	print(*testOkGuardInvalidationParams(nonNilChan)) //want "returned from `testOkGuardInvalidationParams.*` in position 0" "returned from `testOkGuardInvalidationParams.*` in position 0" "returned from `testOkGuardInvalidationParams.*` in position 0"
}

// testOkGuardInvalidationParams checks that assignments to the rich bool (or to the received
// value) invalidate the `ok` guard, while no-op branching leaves it intact.
func testOkGuardInvalidationParams(nonnilChanParam chan *int) *int {
	vNonnil, okNonnil := <-nonnilChanParam

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
			vNonnil, okNonnil = <-nonnilChanParam
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
			vNonnil, ok2Nonnil = <-nonnilChanParam
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

// retsNilableChans returns a deeply nilable channel (the global fed with nil), while
// retsNonnilChans returns a fresh channel that never carries nil.
func retsNilableChans() <-chan *int {
	return nilableChan
}

func retsNonnilChans() <-chan *int {
	return make(chan *int)
}

// The four scenario functions below mirror the *Params variants, with the channels coming from
// function results instead of parameters.

func callTestOkUnguardedResults() {
	print(*testOkUnguardedResults()) //want "returned from `testOkUnguardedResults.*` in position 0" "returned from `testOkUnguardedResults.*` in position 0"
}

func testOkUnguardedResults() *int {
	vNonnil, okNonnil := <-retsNonnilChans()
	vNilable, okNilable := <-retsNilableChans()

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
	vNonnil, okNonnil := <-retsNonnilChans()
	vNilable, okNilable := <-retsNilableChans()

	if okNonnil {
		if dummy {
			return vNonnil // safe: matching guard on a value from the never-nil channel
		}
		if dummy {
			return vNilable // the guard matches, but the channel is deeply nilable
		}
	}

	if okNilable {
		if dummy {
			return vNonnil // guarded by the wrong `ok`
		}
		if dummy {
			return vNilable // the channel is deeply nilable
		}
	}

	i := 0
	return &i
}

func callTestOkGuardScopeResults() {
	print(*testOkGuardScopeResults()) //want "returned from `testOkGuardScopeResults.*` in position 0" "returned from `testOkGuardScopeResults.*` in position 0"
}

func testOkGuardScopeResults() *int {
	vNonnil, okNonnil := <-retsNonnilChans()
	vNilable, okNilable := <-retsNilableChans()

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
	vNonnil, okNonnil := <-retsNonnilChans()

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
			vNonnil, okNonnil = <-retsNonnilChans()
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
			vNonnil, ok2Nonnil = <-retsNonnilChans()
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
	case 6:
		var vNonnil, okNonnil = <-retsNonnilChans()
		if !okNonnil {
			panic(vNonnil)
		}
		return vNonnil
	}

	i := 0
	return &i
}

// The four scenario functions below mirror the *Params variants, with the channels being the
// globals themselves.

func callTestOkUnguardedGlobals() {
	print(*testOkUnguardedGlobals()) //want "returned from `testOkUnguardedGlobals.*` in position 0" "returned from `testOkUnguardedGlobals.*` in position 0"
}

func testOkUnguardedGlobals() *int {
	vNonnil, okNonnil := <-nonNilChan
	vNilable, okNilable := <-nilableChan

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
	vNonnil, okNonnil := <-nonNilChan
	vNilable, okNilable := <-nilableChan

	if okNonnil {
		if dummy {
			return vNonnil // safe: matching guard on a value from the never-nil channel
		}
		if dummy {
			return vNilable // the guard matches, but the channel is deeply nilable
		}
	}

	if okNilable {
		if dummy {
			return vNonnil // guarded by the wrong `ok`
		}
		if dummy {
			return vNilable // the channel is deeply nilable
		}
	}

	i := 0
	return &i
}

func callTestOkGuardScopeGlobals() {
	print(*testOkGuardScopeGlobals()) //want "returned from `testOkGuardScopeGlobals.*` in position 0" "returned from `testOkGuardScopeGlobals.*` in position 0"
}

func testOkGuardScopeGlobals() *int {
	vNonnil, okNonnil := <-nonNilChan
	vNilable, okNilable := <-nilableChan

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
	vNonnil, okNonnil := <-nonNilChan

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
			vNonnil, okNonnil = <-nonNilChan
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
			vNonnil, ok2Nonnil = <-nonNilChan
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

// singleKeysEstablishNonnil checks that the `v, ok := <-ch` guard establishes the received value
// as nonnil, and that assignments to `v` or `ok` invalidate the guard. Each case performs its own
// receive and dereferences in place: dereferences of the same receive share a nil source and
// would be grouped into a single diagnostic, but with one receive per case every unsafe
// dereference reports individually, right at the case that exercises it. Note that the guard's
// effect on the channel operand `ch` itself (shallow nilability) is no longer checkable: send and
// receive on a nil channel block instead of panicking, so under inference there is no consumer
// that would require `ch` to be nonnil (close() is not modeled).
func singleKeysEstablishNonnil(ch chan *int) {
	switch 0 {
	case 1:
		v, ok := <-ch

		// before the check, v is nilable
		print(*v) //want "lacking guarding"

		if !ok {
			return
		}

		// after the check, v is nonnil
		print(*v)
	case 2:
		v, ok := <-ch
		ok = true

		if !ok {
			return
		}

		// here, v should not be nonnil: the check on the overwritten `ok` establishes nothing
		print(*v) //want "lacking guarding"
	case 3:
		v, ok := <-ch
		v = nil

		if !ok {
			return
		}

		// here, v should not be nonnil: it was overwritten after the receive
		print(*v) //want "literal `nil` dereferenced"
	case 4:
		v, ok := <-ch

		if !ok {
			return
		}

		// here, v should be nonnil
		print(*v)
	}
}

func callSingleKeysEstablishNonnil() {
	singleKeysEstablishNonnil(make(chan *int))
}

// plainReflCheck exercises the rich-bool form on a `chan any` that is returned directly. The
// shallow nilability of `ch` itself is not checkable under inference (see
// singleKeysEstablishNonnil), so this is negative coverage only.
func plainReflCheck(ch chan any) any {
	if dummy {
		return ch
	}

	_, ok := <-ch

	if ok {
		return ch
	}

	return ch
}

// BELOW TESTS CHECK SHALLOW NILABILITY OF CHANNELS :: SEND AND RECEIVE ON NIL CHANNELS
var nilChanGlobal chan string
var nonnilChanGlobal = make(chan string)

func testSendToGlobalChan() {
	nilChanGlobal <- "xyz"
	nonnilChanGlobal <- "xyz"
}

func testSendToParamChan(nilChanParam chan string, nonnilChanParam chan string) {
	nilChanParam <- "xyz"
	nonnilChanParam <- "xyz"
}

func testSendToLocalChan() {
	var nilChanLocal chan string
	nilChanLocal <- "xyz"

	var nonnilChanLocal = make(chan string)
	nonnilChanLocal <- "xyz"
}

func testRecvFromGlobalChan() (string, string) {
	return <-nilChanGlobal, <-nonnilChanGlobal
}

func testRecvFromParamChan(nilChanParam chan string, nonnilChanParam chan string) {
	v1 := <-nilChanParam
	v2 := <-nonnilChanParam
	func(...any) {}(v1, v2)
}

func testRecvFromLocalChan() {
	var nilChanLocal chan string
	nilChanLocal <- "xyz"
	v1 := <-nilChanLocal

	var nonnilChanLocal = make(chan string)
	nonnilChanLocal <- "xyz"
	v2 := <-nonnilChanLocal

	func(...any) {}(v1, v2)
}

func retNilChan() chan string {
	var nilChan chan string
	return nilChan
}

func retNonNilChan() chan string {
	return make(chan string)
}

func testSendRecvFuncRet() {
	nilChanLocal := retNilChan()
	nilChanLocal <- "xyz"
	v1 := <-nilChanLocal

	nonnilChanLocal := retNonNilChan()
	nonnilChanLocal <- "xyz"
	v2 := <-nonnilChanLocal

	nilChanLocal <- <-nonnilChanGlobal
	nonnilChanLocal <- <-nonnilChanGlobal

	func(...any) {}(v1, v2)
}
