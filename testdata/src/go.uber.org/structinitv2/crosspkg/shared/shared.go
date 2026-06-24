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

// Package shared defines a struct type returned by both sibling constructors p1.New and p2.New.
// See package app.
package shared

// A is returned by both p1.New and p2.New.
type A struct {
	Ptr  *int
	Aptr *Leaf
}

// Leaf is the dereference target of A.Aptr.
type Leaf struct {
	Ptr *int
}
