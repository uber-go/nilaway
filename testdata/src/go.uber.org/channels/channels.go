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
These tests aim to ensure that nilaway properly handles channels

<nilaway no inference>
*/
package channels

// BELOW TESTS CHECK DEEP NILABILITY OF CHANNELS

// nilable(<-nilableChan)
var nilableChan = make(chan *int)

var nonNilChan = make(chan *int)

var dummyBool = true

// nonnil(nilableChanArg, nonNilChanArg)
// nilable(<-nilableChanArg, nilableArg)
func testChans(nilableChanArg, nonNilChanArg chan *int, nilableArg, nonNilArg *int) *int {
	switch 0 {
	case 1:
		return <-nilableChanArg //want "returned"
	case 2:
		return <-nonNilChanArg
	case 3:
		return <-nilableChan //want "returned"
	case 4:
		return <-nonNilChan
	case 5:
		for i := range nilableChanArg {
			return i //want "returned"
		}
	case 6:
		for i := range nonNilChanArg {
			return i
		}
	case 7:
		for i := range nilableChan {
			return i //want "returned"
		}
	case 8:
		for i := range nonNilChan {
			return i
		}
	case 9:
		nilableChanArg <- nilableArg
		nilableChanArg <- nonNilArg
		nonNilChanArg <- nilableArg //want "assigned"
		nonNilChanArg <- nonNilArg

		nilableChan <- nilableArg
		nilableChan <- nonNilArg
		nonNilChan <- nilableArg //want "assigned"
		nonNilChan <- nonNilArg
	case 10:
		nilableChan <- <-nilableChan
		nilableChan <- <-nonNilChan
		nonNilChan <- <-nilableChan //want "assigned"
		nonNilChan <- <-nonNilChan

		nilableChanArg <- <-nilableChan
		nilableChanArg <- <-nonNilChan
		nonNilChanArg <- <-nilableChan //want "assigned"
		nonNilChanArg <- <-nonNilChan

		nilableChan <- <-nilableChanArg
		nilableChan <- <-nonNilChanArg
		nonNilChan <- <-nilableChanArg //want "assigned"
		nonNilChan <- <-nonNilChanArg

		nilableChan <- <-nilableChanArg
		nilableChan <- <-nonNilChanArg
		nonNilChan <- <-nilableChanArg //want "assigned"
		nonNilChan <- <-nonNilChanArg
	}

	return nonNilArg
}

// nonnil(recvOnlyNilable, sendOnlyNilable, sendOnly, recvOnly)
// nilable(<-sendOnlyNilable, <-recvOnlyNilable, nilable)
type T struct {
	sendOnly        chan<- *int
	recvOnly        <-chan *int
	sendOnlyNilable chan<- *int
	recvOnlyNilable <-chan *int
	nilable         *int
	nonnil          *int
}

func testRestrictedChans(t T) {
	t.sendOnly <- t.nilable //want "assigned"
	t.sendOnlyNilable <- t.nilable
	t.sendOnly <- t.nonnil
	t.sendOnlyNilable <- t.nonnil

	t.nilable = <-t.recvOnly
	t.nilable = <-t.recvOnlyNilable
	t.nonnil = <-t.recvOnly
	t.nonnil = <-t.recvOnlyNilable //want "assigned"
}

type I interface {
	// nonnil(sendOnly, recvOnly, sendOnlyNilable, recvOnlyNilable)
	// nilable(<-sendOnlyNilable, <-recvOnlyNilable)
	retsChans() (
		sendOnly chan<- *int,
		recvOnly <-chan *int,
		sendOnlyNilable chan<- *int,
		recvOnlyNilable <-chan *int)

	// nonnil(result 0)
	retsSendOnly() chan<- *int
	// nonnil(result 0)
	retsRecvOnly() <-chan *int
	// nonnil(result 0)
	// nilable(<-result 0)
	retsSendOnlyNilable() chan<- *int
	// nonnil(result 0)
	// nilable(<-result 0)
	retsRecvOnlyNilable() <-chan *int
}

func testRets(t T, i I) {
	i.retsSendOnly() <- t.nilable //want "assigned"
	i.retsSendOnlyNilable() <- t.nilable
	i.retsSendOnly() <- t.nonnil
	i.retsSendOnlyNilable() <- t.nonnil

	t.nilable = <-i.retsRecvOnly()
	t.nilable = <-i.retsRecvOnlyNilable()
	t.nonnil = <-i.retsRecvOnly()
	t.nonnil = <-i.retsRecvOnlyNilable() //want "assigned"
}

func testIndirectRets(t T, i I) {
	sendOnly, recvOnly, sendOnlyNilable, recvOnlyNilable := i.retsChans()

	sendOnly <- t.nilable //want "assigned"
	// TODO: remove the diagnostic on next line, blocked on
	sendOnlyNilable <- t.nilable //want "assigned"
	sendOnly <- t.nonnil
	sendOnlyNilable <- t.nonnil

	t.nilable = <-recvOnly
	t.nilable = <-recvOnlyNilable
	t.nonnil = <-recvOnly
	t.nonnil = <-recvOnlyNilable // TODO: want "assigned", blocked on
}

var dummy bool

// nonnil(nilableChan, nonnilChan)
// nilable(<-nilableChan)
func testOkChecksForParams(nilableChan chan *int, nonnilChan chan *int) *int {
	vNonnil, okNonnil := <-nonnilChan
	vNilable, okNilable := <-nilableChan

	if dummy {
		return vNonnil //want "returned"
	}
	if dummy {
		return vNilable //want "returned"
	}

	if okNonnil {
		if dummy {
			return vNonnil
		}
		if dummy {
			return vNilable //want "returned"
		}
	}

	if okNilable {
		if dummy {
			return vNonnil //want "returned"
		}
		if dummy {
			return vNilable //want "returned"
		}
	}

	if dummy {
		return vNonnil //want "returned"
	}
	if dummy {
		return vNilable //want "returned"
	}

	switch 0 {
	case 1:
		okNonnil = true

		if okNonnil {
			// this case tests that assignments to the rich bool invalidate the check properly
			return vNonnil //want "returned"
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
			return vNonnil //want "returned"
		}
	case 3:
		switch 0 {
		case 1:
		case 2:
		case 3:
			vNonnil, okNonnil = <-nonnilChan
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
			vNonnil, ok2Nonnil = <-nonnilChan
		}

		if okNonnil {
			// this case is similar to above, but tests a non-identical re-assignment
			// of vNonNil to make sure the check is invalidated
			return vNonnil //want "returned"
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

// nonnil(result 0)
// nilable(<-result 0)
func retsNilableChans() <-chan *int {
	return make(chan *int)
}

// nonnil(result 0)
func retsNonnilChans() <-chan *int {
	return make(chan *int)
}

func testOkChecksForResults() *int {
	vNonnil, okNonnil := <-retsNonnilChans()
	vNilable, okNilable := <-retsNilableChans()

	if dummy {
		return vNonnil //want "returned"
	}
	if dummy {
		return vNilable //want "returned"
	}

	if okNonnil {
		if dummy {
			return vNonnil
		}
		if dummy {
			return vNilable //want "returned"
		}
	}

	if okNilable {
		if dummy {
			return vNonnil //want "returned"
		}
		if dummy {
			return vNilable //want "returned"
		}
	}

	if dummy {
		return vNonnil //want "returned"
	}
	if dummy {
		return vNilable //want "returned"
	}

	switch 0 {
	case 1:
		okNonnil = true

		if okNonnil {
			// this case tests that assignments to the rich bool invalidate the check properly
			return vNonnil //want "returned"
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
			return vNonnil //want "returned"
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
			return vNonnil //want "returned"
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

func testOkChecksForGlobals() *int {
	vNonnil, okNonnil := <-nonNilChan
	vNilable, okNilable := <-nilableChan

	if dummy {
		return vNonnil //want "returned"
	}
	if dummy {
		return vNilable //want "returned"
	}

	if okNonnil {
		if dummy {
			return vNonnil
		}
		if dummy {
			return vNilable //want "returned"
		}
	}

	if okNilable {
		if dummy {
			return vNonnil //want "returned"
		}
		if dummy {
			return vNilable //want "returned"
		}
	}

	if dummy {
		return vNonnil //want "returned"
	}
	if dummy {
		return vNilable //want "returned"
	}

	switch 0 {
	case 1:
		okNonnil = true

		if okNonnil {
			// this case tests that assignments to the rich bool invalidate the check properly
			return vNonnil //want "returned"
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
			return vNonnil //want "returned"
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
			return vNonnil //want "returned"
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

// nonnil(a, b) nilable(<-b, <-d)
func testRangeOverChans(a, b, c, d chan *int) *int {
	switch 0 {
	case 1:
		for a_elem := range a {
			return a_elem
		}
	case 2:
		for b_elem := range b {
			return b_elem //want "returned"
		}
	case 3:
		for c_elem := range c {
			return c_elem
		}
	case 4:
		for d_elem := range d {
			return d_elem //want "returned"
		}
	}
	i := 0
	return &i
}

func takesNonnil(interface{}) {}

func singleKeysEstablishNonnil(ch chan *int) {
	v, ok := <-ch //want "of uninitialized channel"

	// here, ch and v should be nilable
	takesNonnil(v)  //want "passed"
	takesNonnil(ch) //want "passed"

	switch 0 {
	case 1:
		if !ok {
			return
		}

		// here, we should know that BOTH v and ch and nonnil
		takesNonnil(v)
		takesNonnil(ch)
	case 4:
		ok = true

		if !ok {
			return
		}

		// here, neither v nor ch should be nonnil
		takesNonnil(v)  //want "passed"
		takesNonnil(ch) //want "passed"
	case 5:
		v = nil

		if !ok {
			return
		}

		// here, JUST ch should be nonnil
		takesNonnil(v) //want "passed"
		takesNonnil(ch)
	case 6:
		ch = nil

		if !ok {
			return
		}

		// here, JUST v should be nonnil
		takesNonnil(v)
		takesNonnil(ch) //want "passed"
	}
}

func plainReflCheck(ch chan any) any {
	if dummy {
		return ch //want "returned"
	}

	_, ok := <-ch //want "of uninitialized channel"

	if ok {
		return ch
	}

	return ch //want "returned"
}

// BELOW TESTS CHECK SHALLOW NILABILITY OF CHANNELS :: SEND AND RECEIVE ON NIL CHANNELS
var nilChanGlobal chan string
var nonnilChanGlobal = make(chan string)

func testSendToGlobalChan() {
	nilChanGlobal <- "xyz" //want "of uninitialized channel"
	nonnilChanGlobal <- "xyz"
}

// nonnil(nonnilChanParam)
func testSendToParamChan(nilChanParam chan string, nonnilChanParam chan string) {
	nilChanParam <- "xyz" //want "of uninitialized channel"
	nonnilChanParam <- "xyz"
}

func testSendToLocalChan() {
	var nilChanLocal chan string
	nilChanLocal <- "xyz" //want "of uninitialized channel"

	var nonnilChanLocal = make(chan string)
	nonnilChanLocal <- "xyz"
}

func testRecvFromGlobalChan() (string, string) {
	return <-nilChanGlobal, <-nonnilChanGlobal //want "of uninitialized channel"
}

// nonnil(nonnilChanParam)
func testRecvFromParamChan(nilChanParam chan string, nonnilChanParam chan string) {
	v1 := <-nilChanParam //want "of uninitialized channel"
	v2 := <-nonnilChanParam
	func(...any) {}(v1, v2)
}

func testRecvFromLocalChan() {
	var nilChanLocal chan string
	nilChanLocal <- "xyz" //want "of uninitialized channel"
	v1 := <-nilChanLocal  //want "of uninitialized channel"

	var nonnilChanLocal = make(chan string)
	nonnilChanLocal <- "xyz"
	v2 := <-nonnilChanLocal

	func(...any) {}(v1, v2)
}

func retNilChan() chan string {
	var nilChan chan string
	return nilChan
}

// nonnil(result 0)
func retNonNilChan() chan string {
	return make(chan string)
}

func testSendRecvFuncRet() {
	nilChanLocal := retNilChan()
	nilChanLocal <- "xyz" //want "of uninitialized channel"
	v1 := <-nilChanLocal  //want "of uninitialized channel"

	nonnilChanLocal := retNonNilChan()
	nonnilChanLocal <- "xyz"
	v2 := <-nonnilChanLocal

	nilChanLocal <- <-nonnilChanGlobal //want "of uninitialized channel"
	nonnilChanLocal <- <-nonnilChanGlobal

	func(...any) {}(v1, v2)
}
