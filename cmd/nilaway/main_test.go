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

package main

import (
	"reflect"
	"testing"

	"go.uber.org/goleak"
	"go.uber.org/nilaway"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestIsProtocolInvocation(t *testing.T) {
	t.Parallel()

	flags := protocolFlagSet()
	for _, test := range []struct {
		name string
		args []string
		want bool
	}{
		{name: "flags", args: []string{"-flags"}, want: true},
		{name: "double-dash-flags", args: []string{"--flags"}, want: true},
		{name: "version", args: []string{"-V=full"}, want: true},
		{name: "double-dash-version", args: []string{"--V=full"}, want: true},
		{name: "worker", args: []string{"unit.cfg"}, want: true},
		{name: "worker-with-flags", args: []string{"-json", "unit.cfg"}, want: true},
		{name: "worker-with-all-protocol-flags", args: []string{"-json", "-flags=false", "-V=full", "--", "unit.cfg"}, want: true},
		{name: "worker-with-string-flag-value", args: []string{"-include-errors-in-files", "report.cfg", "unit.cfg"}, want: true},
		{name: "worker-with-equals-flag-value", args: []string{"-include-errors-in-files=report.cfg", "unit.cfg"}, want: true},
		{name: "worker-with-double-dash-string-flag-value", args: []string{"--include-errors-in-files", "report.cfg", "unit.cfg"}, want: true},
		{name: "worker-with-double-dash-equals-flag-value", args: []string{"--include-errors-in-files=report.cfg", "unit.cfg"}, want: true},
		{name: "direct-flag-value-looks-like-cfg", args: []string{"-include-errors-in-files", "report.cfg", "./..."}, want: false},
		{name: "direct-flag-value-with-equals-looks-like-cfg", args: []string{"-include-errors-in-files=report.cfg", "./..."}, want: false},
		{name: "direct", args: []string{"./..."}, want: false},
		{name: "direct-stdlib", args: []string{"std"}, want: false},
		{name: "multiple-positionals", args: []string{"unit.cfg", "./..."}, want: false},
		{name: "flag-after-positional", args: []string{"unit.cfg", "-json"}, want: false},
	} {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isProtocolInvocation(tt.args, flags); got != tt.want {
				t.Fatalf("isProtocolInvocation(%q) = %t, want %t", tt.args, got, tt.want)
			}
		})
	}
}

func TestVetCommandPreservesDirectArguments(t *testing.T) {
	t.Parallel()

	got := vetCommand("/tmp/nilaway", []string{"-exclude-test-files=false", "std"})
	want := []string{"go", "vet", "-vettool=/tmp/nilaway", "-exclude-test-files=false", "std"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("vetCommand() = %q, want %q", got, want)
	}
}

func TestAnalyzerPreservesMetadata(t *testing.T) {
	t.Parallel()

	if Analyzer.Name != nilaway.Analyzer.Name {
		t.Fatalf("Name = %q, want %q", Analyzer.Name, nilaway.Analyzer.Name)
	}
	if Analyzer.Doc != nilaway.Analyzer.Doc {
		t.Fatalf("Doc mismatch")
	}
	if !reflect.DeepEqual(Analyzer.FactTypes, nilaway.Analyzer.FactTypes) {
		t.Fatalf("FactTypes = %v, want %v", Analyzer.FactTypes, nilaway.Analyzer.FactTypes)
	}
	if Analyzer.ResultType != nilaway.Analyzer.ResultType {
		t.Fatalf("ResultType mismatch")
	}
	if !reflect.DeepEqual(Analyzer.Requires, nilaway.Analyzer.Requires) {
		t.Fatalf("Requires = %v, want %v", Analyzer.Requires, nilaway.Analyzer.Requires)
	}
	if Analyzer.Run == nil {
		t.Fatal("Run must be set")
	}
}

func TestParseFilePrefixes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty", input: "", wantErr: false},
		{name: "single", input: "/tmp", wantErr: false},
		{name: "multiple", input: "/tmp,/usr/local", wantErr: false},
	} {
		tt := test
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			prefixes, err := parseFilePrefixes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseFilePrefixes(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.input == "" && len(prefixes) != 0 {
				t.Fatalf("expected nil prefixes for empty input, got %v", prefixes)
			}
		})
	}
}
