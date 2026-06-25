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

// Package p1 has a constructor New that always initializes the Aptr field. See package app.
package p1

import "go.uber.org/structinitv2/crosspkg/shared"

// New always initializes Aptr.
func New() *shared.A {
	return &shared.A{Aptr: &shared.Leaf{}}
}
