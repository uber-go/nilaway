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
These tests aim to ensure that nilness information gained in an earlier node of short circuited
logic is correctly propagate to later nodes. All the checks here are structured safely, but if
the nilflow engine were not aware of short circuiting it would not realize that.

<nilaway no inference>
*/
package nilcheck

func noop() {}

/*
nilable(f)
*/
type ralph struct {
	f *ralph
}

func nonNil() *ralph {
	return new(ralph)
}

// nilable(x)
func noDeep0(x *ralph) *ralph {
	if x == nil {
		x = nonNil()
	}
	return x
}

// nilable(x)
func noDeep1(x *ralph) *ralph {
	if x != nil {
		return x
	}
	return nonNil()
}

// nilable(x)
func noDeep2(x *ralph) *ralph {
	if nil == x {
		x = nonNil()
	}
	return x
}

// nilable(x)
func noDeep3(x *ralph) *ralph {
	if nil != x {
		return x
	}
	return nonNil()
}

// this test used to fail when nil-checking was implemented wrong
// nilable(x)
func noDeep4(x *ralph) *ralph {
	if x == nil {
		x = x
	}
	return x //want "returned from the function `noDeep4`"
}

// nilable(x)
func oneDeep(x *ralph) *ralph {
	if x != nil && x.f != nil {
		return x.f
	}
	return noDeep0(x)
}

// nilable(x)
func twoDeep(x *ralph) *ralph {
	if x != nil && x.f != nil && x.f.f != nil {
		return x.f.f
	}
	return oneDeep(x)
}

func posNilCheckPreservesNonNil(x *ralph) *ralph {
	if x == nil {
		noop()
	}
	return x
}

func negNilCheckPreservesNonNil(x *ralph) *ralph {
	if x != nil {
		noop()
	}
	return x
}

// nilable(x)
func posNilCheckPreservesNilable(x *ralph) *ralph {
	if x == nil {
		noop()
	}
	return x //want "returned"
}

// nilable(x)
func negNilCheckPreservesNilable(x *ralph) *ralph {
	if x != nil {
		noop()
	}
	return x //want "returned"
}

func posNilCheckDoesntTriggerConsumption(x *ralph) *ralph {
	if x == nil {
		// this is an interesting case
		// if this return were ever hit then we would know that it returns nil where a non-nil
		// value is expected, and thus should be an error, but indeed we can statically conclude
		// it never will be hit! so we don't expect an error. For subtle reasons, passing of this
		// test case is tied to the above 4
		return x
	}
	return x
}
