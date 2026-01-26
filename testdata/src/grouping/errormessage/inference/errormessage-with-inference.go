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

// This package tests error messages for the inference mode.

package inference

// Below test checks error messages for single assertion conflicts when the producer expression is not authentic, i.e.,
// it is built from assertion nodes and hence not found in the original AST.

type S struct {
	f map[int]*S
	g string
}

type T struct {
	m map[string]S
}

// Here, although the two error messages are similar for test1 and test2, they should not be grouped together as they are
// from different functions.
func (t *T) test1(str string) {
	p := t.m[str]
	_ = *p.f[0] //want "dereferenced"
}

func (t *T) test2(str string) {
	p := t.m[str]
	_ = *p.f[0] //want "dereferenced"
}

// Here, the error messages for the two dereferences in test3 are similar and should be grouped together.
func (t *T) test3(str string) {
	p := t.m[str]
	_ = *p.f[0] //want "Same nil source could also cause potential nil panic"
	_ = *p.f[1]
}

// Here, the two error messages in test4 are similar, and ideally they should be grouped together.
// However, since producer position is unavailable, NilAway uses a heuristic combined of the producer and consumer
// error messages to group them, and in this case the consumer messages are different: "dereferenced" and "accessed field".
// Hence, they are not grouped together.
func (t *T) test4(str string) {
	p := t.m[str]
	_ = *p.f[0]  //want "dereferenced"
	_ = p.f[1].f //want "accessed field"
}

// Similar to test4, here the error messages are not grouped since the accessed fields in consumer messages are different.
func (t *T) test5(str string) {
	p := t.m[str]
	_ = p.f[0].f //want "accessed field `f`"
	_ = p.f[1].g //want "accessed field `g`"
}

// Here, although the two error messages are similar for the pairs test8-test9 and test10-test11,
// they should not be grouped together as they are from different functions.

func test8(mp map[int]*int) {
	_ = *mp[0] //want "dereferenced"
}

func test9(mp map[int]*int) {
	_ = *mp[0] //want "dereferenced"
}

func test10() {
	mp := make(map[int]*int)
	_ = *mp[0] //want "dereferenced"
}

func test11() {
	mp := make(map[int]*int)
	_ = *mp[0] //want "dereferenced"
}
