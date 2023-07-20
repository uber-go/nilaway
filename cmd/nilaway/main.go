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

// main package makes it possible to build NilAway as a standalone code checker that can be
// independently invoked to check other packages. It also makes it possible to run cpu and mem
// profiles on NilAway through command line arguments when analyzing packages.
package main

import (
	"go.uber.org/nilaway"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(nilaway.Analyzer)
}
