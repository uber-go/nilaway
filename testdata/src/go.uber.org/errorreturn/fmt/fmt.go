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
package fmt

type any interface{}

// these stubs simulate the real `fmt` package because we can't import it in tests

type err struct{}

func (err) Error() string { return "err message" }

// nonnil(result 0)
func Errorf(format string, a ...any) error { return err{} }
