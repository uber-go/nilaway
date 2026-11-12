//  Copyright (c) 2026 Uber Technologies, Inc.
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

package globalvars

import "sync"

// sync.Once.Do inside init()

var _onceInit *int
var _onceInitMulti1, _onceInitMulti2 *int
var _once sync.Once
var _onceMulti sync.Once

func init() {
	_once.Do(func() {
		_onceInit = new(int)
	})
}

func init() {
	_onceMulti.Do(func() {
		_onceInitMulti1 = new(int)
		_onceInitMulti2 = new(int)
	})
}

// sync.OnceFunc / OnceValue / OnceValues as global initializers

var _onceFunc = sync.OnceFunc(func() {})

var _onceValue = sync.OnceValue(func() *int {
	return new(int)
})

type onceConfig struct{ Name string }

var _onceValueStruct = sync.OnceValue(func() *onceConfig {
	return &onceConfig{Name: "test"}
})

var _onceValues = sync.OnceValues(func() (*int, error) {
	return new(int), nil
})

func useSyncOnceGlobals() {
	print(*_onceInit)
	print(*_onceInitMulti1)
	print(*_onceInitMulti2)

	_onceFunc()

	v := _onceValue()
	print(*v)

	s := _onceValueStruct()
	print(s.Name)

	p, _ := _onceValues()
	print(*p)
}
