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

// nilawaytestvettool is the modular analysis worker used by nilawaytest.
package main

import (
	"flag"

	"go.uber.org/nilaway"
	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	// Lift NilAway's configuration flags to the vettool's top-level flag set so the test harness
	// can pass the same configuration used by the in-process analyzer.
	config.Analyzer.Flags.VisitAll(func(f *flag.Flag) {
		flag.Var(f.Value, f.Name, f.Usage)
	})
	unitchecker.Main(nilaway.Analyzer)
}
