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

// Package main implements the golden tests for NilAway to ensure that the errors reported on the
// stdlib are equal between the base branch and the test branch for preventing functionality
// regressions during development.
package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/fatih/color"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/tools/internal/pathutil"
)

// Diagnostic is the diagnostic reported by NilAway.
type Diagnostic struct {
	// Posn is the position string of the diagnostic.
	Posn string `json:"posn"`
	// Message is the message reported by NilAway.
	Message string `json:"message"`
}

// IsInternalPanic returns whether this diagnostic is an internal panic.
func (d *Diagnostic) IsInternalPanic() bool {
	return strings.Contains(d.Message, config.InternalPanicPrefix)
}

// BranchResult stores the information about a branch, and the diagnostics reported on that branch.
type BranchResult struct {
	// Name is the friendly name of the branch (if available and not "HEAD", otherwise it is equal
	// to its ShortSHA).
	Name string
	// ShortSHA is the short SHA of the branch.
	ShortSHA string
	// Result is the set of diagnostics NilAway reported on the branch.
	Result map[Diagnostic]bool
}

// Run runs the golden tests on the base branch and the test branch and writes the summary and
// diff to the writer. The standard library analysis is split into at most `shards` groups, each
// analyzed by a separate NilAway process to bound peak memory usage (a value <= 1 disables
// sharding and analyzes the whole standard library in a single process).
func Run(writer io.Writer, baseBranch, testBranch string, shards int) error {
	// First verify that the git repository is clean.
	out, err := exec.Command("git", "status", "--porcelain=v1").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if len(out) != 0 {
		return errors.New("git repository is not clean")
	}

	// Then verify that we are at the root of the git project.
	gitRoot, err := pathutil.GitRoot()
	if err != nil {
		return err
	}

	wd, err := pathutil.WorkingDirectory()
	if err != nil {
		return err
	}

	if gitRoot != wd {
		return fmt.Errorf("not at the root of the git repository: %q != %q", gitRoot, wd)
	}

	// Get the current branch name and switch back to it after the golden test.
	out, err = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		return fmt.Errorf("get current branch name: %w", err)
	}
	originalBranch := strings.TrimSpace(string(out))
	if originalBranch == "" || originalBranch == "HEAD" {
		out, err = exec.Command("git", "rev-parse", "--short", "HEAD").CombinedOutput()
		if err != nil {
			return fmt.Errorf("get short commit hash of HEAD: %w, output: %q", err, out)
		}
		originalBranch = strings.TrimSpace(string(out))
	}
	defer func() {
		_, err := exec.Command("git", "checkout", originalBranch).CombinedOutput()
		if err != nil {
			log.Fatalf("failed to checkout original branch %q: %q", originalBranch, err)
		}
	}()

	// If test branch is not specified, use the current branch.
	if testBranch == "" {
		log.Printf("test branch is not specified, using current branch %q", originalBranch)
		testBranch = originalBranch
	}

	// Initialize the base and test branch SHAs.
	branches := [2]*BranchResult{{Name: baseBranch}, {Name: testBranch}}
	for _, branch := range branches {
		out, err = exec.Command("git", "rev-parse", "--short", branch.Name).CombinedOutput()
		if err != nil {
			return fmt.Errorf("get short commit hash of branch %q: %w, output: %q", branch.Name, err, out)
		}
		branch.ShortSHA = strings.TrimSpace(string(out))
	}

	// Now the golden test starts. From here on, we should use the `branches` variable to refer to
	// the base and test branches.
	log.Printf("running golden test on base branch %q (%s) and test branch %q (%s)\n",
		branches[0].Name, branches[0].ShortSHA, branches[1].Name, branches[1].ShortSHA,
	)

	// Compute the package groups once so that both branches analyze the exact same set of packages.
	groups, err := packageGroups(shards)
	if err != nil {
		return err
	}
	if len(groups) > 1 {
		log.Printf("splitting the standard library analysis into %d shards", len(groups))
	}

	for _, branch := range branches {
		commands := [][]string{
			{"git", "checkout", branch.ShortSHA},
			{"make", "build"},
		}
		for _, command := range commands {
			_, err := exec.Command(command[0], command[1:]...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("run command %q: %w", command, err)
			}
		}

		// Run the built NilAway binary on the stdlib and parse the diagnostics.
		start := time.Now()
		diagnostics, err := runNilAwaySharded(groups)
		if err != nil {
			return fmt.Errorf("branch %q: %w", branch.Name, err)
		}
		elapsed := time.Since(start)
		log.Printf("NilAway execution time for branch \"%s\": %s", branch.Name, elapsed)
		branch.Result = diagnostics
	}

	WriteDiff(writer, branches)
	return CheckInternalPanics(branches)
}

// packageGroups returns the groups of packages that NilAway should be run on. When shards <= 1 it
// returns a single group with the `std` pattern (analyzing the whole standard library in one
// process). Otherwise it enumerates the standard library packages and deterministically partitions
// them into at most `shards` groups.
func packageGroups(shards int) ([][]string, error) {
	if shards <= 1 {
		return [][]string{{"std"}}, nil
	}
	packages, err := listStdPackages()
	if err != nil {
		return nil, err
	}
	return partitionPackages(packages, shards), nil
}

// listStdPackages returns the import paths of all standard library packages for the current Go
// toolchain, as reported by `go list std`.
func listStdPackages() ([]string, error) {
	out, err := exec.Command("go", "list", "std").Output()
	if err != nil {
		return nil, fmt.Errorf("list standard library packages: %w", err)
	}
	var packages []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			packages = append(packages, line)
		}
	}
	return packages, nil
}

// partitionPackages deterministically partitions the given package import paths into at most
// `shards` groups. Packages that share the same first two import path components (e.g. `net/http`,
// `crypto/tls`) are kept in the same group so that each group forms a connected subtree of the
// dependency graph, which keeps the per-process memory footprint small. Groups are assigned to the
// currently smallest shard (largest first) to balance the number of packages per shard.
func partitionPackages(packages []string, shards int) [][]string {
	if shards <= 1 || len(packages) == 0 {
		return [][]string{packages}
	}

	grouped := make(map[string][]string)
	for _, pkg := range packages {
		key := pkg
		if parts := strings.SplitN(pkg, "/", 3); len(parts) >= 2 {
			key = parts[0] + "/" + parts[1]
		}
		grouped[key] = append(grouped[key], pkg)
	}

	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if n := cmp.Compare(len(grouped[b]), len(grouped[a])); n != 0 {
			return n
		}
		return cmp.Compare(a, b)
	})

	buckets := make([][]string, shards)
	for _, key := range keys {
		target := 0
		for i := range buckets {
			if len(buckets[i]) < len(buckets[target]) {
				target = i
			}
		}
		buckets[target] = append(buckets[target], grouped[key]...)
	}

	var result [][]string
	for _, bucket := range buckets {
		if len(bucket) == 0 {
			continue
		}
		slices.Sort(bucket)
		result = append(result, bucket)
	}
	return result
}

// runNilAwaySharded runs NilAway on each of the given package groups and merges the diagnostics
// reported across all groups. The diagnostics are keyed by their position and message, so merging
// the per-shard sets yields the same result as a single run over all packages.
func runNilAwaySharded(groups [][]string) (map[Diagnostic]bool, error) {
	if len(groups) == 1 {
		return runNilAway(groups[0])
	}

	merged := make(map[Diagnostic]bool)
	for i, group := range groups {
		start := time.Now()
		diagnostics, err := runNilAway(group)
		if err != nil {
			return nil, fmt.Errorf("shard %d/%d: %w", i+1, len(groups), err)
		}
		log.Printf("shard %d/%d: %d packages, %d diagnostics, %s",
			i+1, len(groups), len(group), len(diagnostics), time.Since(start))
		for diagnostic := range diagnostics {
			merged[diagnostic] = true
		}
	}
	log.Printf("finished running %d shards: %d unique diagnostics", len(groups), len(merged))
	return merged, nil
}

// runNilAway runs the built NilAway binary on the given packages and parses the reported
// diagnostics.
func runNilAway(packages []string) (map[Diagnostic]bool, error) {
	args := []string{"-include-errors-in-files", "/", "-json", "-pretty-print=false", "-group-error-messages=true"}
	args = append(args, packages...)

	var buf bytes.Buffer
	cmd := exec.Command("bin/nilaway", args...)
	cmd.Stdout = &buf
	// Inherit env vars such that users can control the resource usages via GOMEMLIMIT, GOGC
	// etc. env vars. Route stderr to the tool's stderr so that GODEBUG output (e.g. gctrace)
	// and analyzer errors remain visible in CI logs.
	cmd.Env = os.Environ()
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("run NilAway on %d packages: %w", len(packages), err)
	}
	return ParseDiagnostics(&buf)
}

// ParseDiagnostics parses the diagnostics from the raw JSON output of NilAway and returns the
// set of diagnostics.
func ParseDiagnostics(reader io.Reader) (map[Diagnostic]bool, error) {
	// Package name -> "nilaway" -> slice of diagnostics.
	var output map[string]map[string][]Diagnostic
	if err := json.NewDecoder(reader).Decode(&output); err != nil {
		return nil, fmt.Errorf("decoding diagnostics: %w", err)
	}

	allDiagnostics := make(map[Diagnostic]bool)
	for _, packages := range output {
		diagnostics, ok := packages["nilaway"]
		if !ok {
			continue
		}
		for _, d := range diagnostics {
			allDiagnostics[d] = true
		}
	}

	return allDiagnostics, nil
}

// CheckInternalPanics returns an error if NilAway reported an internal panic on either branch.
func CheckInternalPanics(branches [2]*BranchResult) error {
	var affectedBranches []string
	for _, branch := range branches {
		count := 0
		for diagnostic := range branch.Result {
			if diagnostic.IsInternalPanic() {
				count++
			}
		}
		if count > 0 {
			affectedBranches = append(affectedBranches,
				fmt.Sprintf("%s (%s): %d", branch.Name, branch.ShortSHA, count))
		}
	}
	if len(affectedBranches) == 0 {
		return nil
	}
	return fmt.Errorf("%s diagnostic(s) reported on %s",
		config.InternalPanicPrefix, strings.Join(affectedBranches, ", "))
}

// WriteDiff writes the summary and the diff (if the base and test are different) between the base
// and test diagnostics to the writer. If the writer is os.Stdout, it will write the diff in color.
func WriteDiff(writer io.Writer, branches [2]*BranchResult) {
	// Compute the diagnostic differences between base and test branches.
	minuses, pluses := Diff(branches[0].Result, branches[1].Result), Diff(branches[1].Result, branches[0].Result)

	// Write the summary lines first.
	MustFprint(fmt.Fprintf(writer, "## Golden Test\n\n"))
	if len(pluses) == 0 && len(minuses) == 0 {
		MustFprint(fmt.Fprint(writer, "> [!NOTE]  \n"))
		MustFprint(fmt.Fprintf(writer, "> ✅ NilAway errors reported on standard libraries are **identical**.\n"))
	} else {
		MustFprint(fmt.Fprintf(writer, "> [!WARNING]  \n"))
		MustFprint(fmt.Fprintf(writer, "> ❌ NilAway errors reported on stdlib are **different**"))
		// Optionally write the direction of the change (if present).
		if len(branches[0].Result) < len(branches[1].Result) {
			MustFprint(fmt.Fprintf(writer, " 📈"))
		} else if len(branches[0].Result) > len(branches[1].Result) {
			MustFprint(fmt.Fprintf(writer, " 📉"))
		}
		MustFprint(fmt.Fprint(writer, ".\n"))
	}
	// Now write the statistics of the diagnostics in each branch.
	MustFprint(fmt.Fprint(writer, "> \n"))
	for i, branch := range branches {
		MustFprint(fmt.Fprintf(writer, "> **%d** errors on ", len(branch.Result)))
		if i == 1 {
			MustFprint(fmt.Fprintf(writer, "test branch"))
		} else {
			MustFprint(fmt.Fprintf(writer, "base branch"))
		}
		if branch.Name != branch.ShortSHA {
			MustFprint(fmt.Fprintf(writer, " (%s, %s)", branch.Name, branch.ShortSHA))
		} else {
			MustFprint(fmt.Fprintf(writer, " (%s)", branch.ShortSHA))
		}
		MustFprint(fmt.Fprint(writer, "\n"))
	}

	// Early return if there is no diff to write.
	if len(pluses) == 0 && len(minuses) == 0 {
		return
	}

	// If the writer is os.Stdout, we will write the diff in color.
	color.NoColor = true
	if f, ok := writer.(*os.File); ok && f == os.Stdout {
		color.NoColor = false
	}

	MustFprint(fmt.Fprintf(writer, "\n<details>\n"))
	MustFprint(fmt.Fprintf(writer, "<summary>Diffs</summary>\n\n"))
	MustFprint(fmt.Fprintf(writer, "```diff\n"))
	for i, diff := range [...][]Diagnostic{pluses, minuses} {
		prefix, c := "+", color.FgGreen
		if i == 1 {
			prefix, c = "-", color.FgRed
		}
		for _, d := range diff {
			lines := strings.Split(strings.TrimSpace(d.Message), "\n")
			// Add Posn to the first line and prefix to each line for diff formatting.
			lines[0] = d.Posn + ": " + lines[0]
			for i := range lines {
				lines[i] = prefix + " " + lines[i]
			}
			output := strings.Join(lines, "\n") + "\n"
			MustFprint(color.New(c).Fprint(writer, output))
		}
	}
	MustFprint(fmt.Fprintf(writer, "```\n\n"))
	MustFprint(fmt.Fprintf(writer, "</details>\n"))
}

// Diff computes the diff between the first and second diagnostics and returns the diff in
// alphabetical order.
func Diff(first, second map[Diagnostic]bool) []Diagnostic {
	var diff []Diagnostic
	for d := range first {
		if !second[d] {
			diff = append(diff, d)
		}
	}
	// Sort the diff such that we have stable ordering for the same runs.
	slices.SortFunc(diff, func(i, j Diagnostic) int {
		if iPanic, jPanic := i.IsInternalPanic(), j.IsInternalPanic(); iPanic != jPanic {
			if iPanic {
				return -1
			}
			return 1
		}
		if n := cmp.Compare(i.Posn, j.Posn); n != 0 {
			return n
		}
		return cmp.Compare(i.Message, j.Message)
	})
	return diff
}

// MustFprint is a helper function that takes the result of the family of Fprint functions and
// panics if the error is nonnil.
func MustFprint(_ int, err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	fset := flag.NewFlagSet("golden-test", flag.ExitOnError)
	baseBranch := fset.String("base-branch", "main", "the base branch to compare against")
	testBranch := fset.String("test-branch", "", "the test branch to run golden tests (default current branch)")
	resultFile := fset.String("result-file", "", "the file to write the diff to, default stdout")
	shards := fset.Int("shards", 1, "number of shards to split the standard library analysis into (<= 1 disables sharding)")
	if err := fset.Parse(os.Args[1:]); err != nil {
		log.Printf("failed to parse flags: %v\n", err)
		flag.PrintDefaults()
		os.Exit(1)
	}

	writer := os.Stdout
	if *resultFile != "" {
		w, err := os.OpenFile(*resultFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Fatalf("failed to open file %q: %v", *resultFile, err)
		}
		writer = w
	}

	if err := Run(writer, *baseBranch, *testBranch, *shards); err != nil {
		log.Printf("failed to run golden test: %v", err)
		var e *exec.ExitError
		if errors.As(err, &e) {
			log.Printf("failed command output: %v", e.Stderr)
		}
		os.Exit(1)
	}
}
