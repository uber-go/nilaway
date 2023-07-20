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

// <nilaway no inference>
package require

type any interface{}

// these stubs simulate the real `github.com/stretchr/testify/require` package because we can't import it in tests

type Assertions struct {
	t TestingT
}

type TestingT interface {
	Errorf(format string, args ...interface{})
}

// nilable(object)
func NotNil(t TestingT, object interface{}, msgAndArgs ...interface{}) bool { return true }

// nilable(object)
func NotNilf(t TestingT, object interface{}, msg string, args ...interface{}) bool { return true }

// nilable(object)
func Nil(t TestingT, object interface{}, msgAndArgs ...interface{}) bool { return true }

// nilable(object)
func Nilf(t TestingT, object interface{}, msg string, args ...interface{}) bool { return true }

// nilable(object)
func NoError(t TestingT, object interface{}, msgAndArgs ...interface{}) bool { return true }

// nilable(object)
func NoErrorf(t TestingT, object interface{}, msg string, args ...interface{}) bool { return true }

// nilable(object)
func Error(t TestingT, object interface{}, msgAndArgs ...interface{}) bool { return true }

// nilable(object)
func Errorf(t TestingT, object interface{}, msg string, args ...interface{}) bool { return true }

func True(t TestingT, value bool, msgAndArgs ...interface{}) bool { return true }

func Truef(t TestingT, value bool, msg string, args ...interface{}) bool { return true }

func False(t TestingT, value bool, msgAndArgs ...interface{}) bool { return true }

func Falsef(t TestingT, value bool, msg string, args ...interface{}) bool { return true }

// nilable(expected, actual)
func Equal(t TestingT, expected interface{}, actual interface{}, msgAndArgs ...interface{}) {}

// nilable(expected, actual)
func Equalf(t TestingT, expected interface{}, actual interface{}, msg string, args ...interface{}) {}

// nilable(expected, actual)
func NotEqual(t TestingT, expected interface{}, actual interface{}, msgAndArgs ...interface{}) {}

// nilable(expected, actual)
func NotEqualf(t TestingT, expected interface{}, actual interface{}, msg string, args ...interface{}) {
}

// nilable(a, b)
func Greater(t TestingT, a interface{}, b interface{}, msgAndArgs ...interface{}) {}

// nilable(a, b)
func Greaterf(t TestingT, a interface{}, b interface{}, msg string, args ...interface{}) {}

// nilable(a, b)
func GreaterOrEqual(t TestingT, a interface{}, b interface{}, msgAndArgs ...interface{}) {}

// nilable(a, b)
func GreaterOrEqualf(t TestingT, a interface{}, b interface{}, msg string, args ...interface{}) {}

// nilable(a, b)
func Less(t TestingT, a interface{}, b interface{}, msgAndArgs ...interface{}) {}

// nilable(a, b)
func Lessf(t TestingT, a interface{}, b interface{}, msg string, args ...interface{}) {}

// nilable(a, b)
func LessOrEqual(t TestingT, a interface{}, b interface{}, msgAndArgs ...interface{}) {}

// nilable(a, b)
func LessOrEqualf(t TestingT, a interface{}, b interface{}, msg string, args ...interface{}) {}

// nilable(object)
func Len(t TestingT, object interface{}, length int, msgAndArgs ...interface{}) {}

// nilable(object)
func Lenf(t TestingT, object interface{}, length int, msg string, args ...interface{}) {}

// nilable(object)
func (*Assertions) NotNil(object interface{}, msgAndArgs ...interface{}) bool { return true }

// nilable(object)
func (*Assertions) NotNilf(object interface{}, msg string, args ...interface{}) bool { return true }

// nilable(object)
func (*Assertions) Nil(object interface{}, msgAndArgs ...interface{}) bool { return true }

// nilable(object)
func (*Assertions) Nilf(object interface{}, msg string, args ...interface{}) bool { return true }

// nilable(object)
func (*Assertions) NoError(object interface{}, msgAndArgs ...interface{}) bool { return true }

// nilable(object)
func (*Assertions) NoErrorf(object interface{}, msg string, args ...interface{}) bool { return true }

// nilable(object)
func (*Assertions) Error(object interface{}, msgAndArgs ...interface{}) bool { return true }

// nilable(object)
func (*Assertions) Errorf(object interface{}, msg string, args ...interface{}) bool { return true }

func (*Assertions) True(value bool, msgAndArgs ...interface{}) bool { return true }

func (*Assertions) Truef(value bool, msg string, args ...interface{}) bool { return true }

func (*Assertions) False(value bool, msgAndArgs ...interface{}) bool { return true }

func (*Assertions) Falsef(value bool, msg string, args ...interface{}) bool { return true }

// nilable(expected, actual)
func (*Assertions) Equal(expected interface{}, actual interface{}, msgAndArgs ...interface{}) {}

// nilable(expected, actual)
func (*Assertions) Equalf(expected interface{}, actual interface{}, msg string, args ...interface{}) {
}

// nilable(expected, actual)
func (*Assertions) NotEqual(expected interface{}, actual interface{}, msgAndArgs ...interface{}) {}

// nilable(expected, actual)
func (*Assertions) NotEqualf(expected interface{}, actual interface{}, msg string, args ...interface{}) {
}

// nilable(a, b)
func (*Assertions) Greater(a interface{}, b interface{}, msgAndArgs ...interface{}) {}

// nilable(a, b)
func (*Assertions) Greaterf(a interface{}, b interface{}, msg string, args ...interface{}) {}

// nilable(a, b)
func (*Assertions) GreaterOrEqual(a interface{}, b interface{}, msgAndArgs ...interface{}) {}

// nilable(a, b)
func (*Assertions) GreaterOrEqualf(a interface{}, b interface{}, msg string, args ...interface{}) {}

// nilable(a, b)
func (*Assertions) Less(a interface{}, b interface{}, msgAndArgs ...interface{}) {}

// nilable(a, b)
func (*Assertions) Lessf(a interface{}, b interface{}, msg string, args ...interface{}) {}

// nilable(a, b)
func (*Assertions) LessOrEqual(a interface{}, b interface{}, msgAndArgs ...interface{}) {}

// nilable(a, b)
func (*Assertions) LessOrEqualf(a interface{}, b interface{}, msg string, args ...interface{}) {}

// nilable(object)
func (*Assertions) Len(object interface{}, length int, msgAndArgs ...interface{}) {}

// nilable(object)
func (*Assertions) Lenf(object interface{}, length int, msg string, args ...interface{}) {}
