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

package nilaway

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/analysistest"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

// loadBenchPackages loads (and type-checks) the given testdata packages once, in the same GOPATH
// mode used by analysistest, so that the benchmark loop only measures the analysis itself.
func loadBenchPackages(b *testing.B, patterns ...string) []*packages.Package {
	b.Helper()

	testdata := analysistest.TestData()
	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax,
		Dir:   filepath.Join(testdata, "src"),
		Tests: false,
		Env: append(os.Environ(),
			"GOPATH="+testdata,
			"GO111MODULE=off",
			"GOWORK=off",
			"GOPROXY=off",
		),
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		b.Fatalf("failed to load packages %v: %v", patterns, err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		b.Fatalf("errors while loading packages %v", patterns)
	}
	if len(pkgs) == 0 {
		b.Fatalf("no packages matched %v", patterns)
	}
	return pkgs
}

// BenchmarkNilAway runs the full NilAway analyzer (including all of its sub-analyzers and fact
// computation for dependencies) on representative testdata packages.
func BenchmarkNilAway(b *testing.B) {
	benchmarks := []struct {
		name     string
		patterns []string
	}{
		{name: "HelloWorld", patterns: []string{"go.uber.org/helloworld"}},
		{name: "Inference", patterns: []string{"go.uber.org/inference"}},
		{name: "Contracts", patterns: []string{"go.uber.org/contracts/..."}},
		{name: "ErrorReturn", patterns: []string{"go.uber.org/errorreturn", "go.uber.org/errorreturn/typeswitch", "go.uber.org/errorreturn/typeswitch/shadownil"}},
		{name: "Maps", patterns: []string{"go.uber.org/maps"}},
		{name: "Slices", patterns: []string{"go.uber.org/slices"}},
		{name: "LoopFlow", patterns: []string{"go.uber.org/loopflow"}},
		{name: "Generics", patterns: []string{"go.uber.org/generics"}},
		{name: "MethodImplementation", patterns: []string{"go.uber.org/methodimplementation/..."}},
		{name: "TransitiveFacts", patterns: []string{"go.uber.org/transitivefacts/..."}},
		{name: "All", patterns: []string{"go.uber.org/..."}},
	}

	analyzers := []*analysis.Analyzer{Analyzer}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			pkgs := loadBenchPackages(b, bm.patterns...)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				graph, err := checker.Analyze(analyzers, pkgs, nil)
				if err != nil {
					b.Fatal(err)
				}
				for act := range graph.All() {
					if act.Err != nil {
						b.Fatalf("analysis of %s failed: %v", act.Package.PkgPath, act.Err)
					}
				}
			}
		})
	}
}
