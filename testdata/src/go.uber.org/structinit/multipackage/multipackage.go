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

// Package multipackage test functions as a sanity check for multi-file handling, just making sure that in the
// case of linked files no errors are thrown
package multipackage

import (
	"go.uber.org/structinit/multipackage/packageone"
)

func m() *int {
	var a = packageone.GiveEmptyA()
	return a.Aptr.Ptr
}

func callF12() {
	t := &packageone.A{Aptr: &packageone.A{}}
	packageone.F12(t)
}

type B struct {
	packageone.A
}

func m2() *B {
	var b = &B{
		A: packageone.A{},
	}
	return b
}
