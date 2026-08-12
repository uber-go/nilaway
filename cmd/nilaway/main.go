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
// independently invoked to check other packages.
//
// NilAway ships as a modular binary that serves two roles from one executable:
//
//   - Direct invocation (e.g. `nilaway ./...`, `nilaway std`) delegates to `go vet
//     -vettool=<self>`. The go command enumerates the package universe and spawns one worker
//     process per compilation unit, which bounds peak memory.
//   - Worker invocation (the `-flags`, `-V=full`, and `*.cfg` protocol used by `go vet`) is
//     handled by the golang.org/x/tools unitchecker, which analyzes a single package per process
//     and preserves cross-package facts through the build cache.
//
// This split keeps cross-package facts and exact diagnostics while preventing the whole-program
// type information from accumulating in a single address space.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.uber.org/nilaway"
	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/unitchecker"
)

// Analyzer is identical to the one in nilaway.go, except that it overrides the run function for
// extra filtering of errors, since the unitchecker does not support error suppression like other
// popular linter drivers.
var Analyzer = &analysis.Analyzer{
	Name:       nilaway.Analyzer.Name,
	Doc:        nilaway.Analyzer.Doc,
	Run:        run,
	FactTypes:  nilaway.Analyzer.FactTypes,
	ResultType: nilaway.Analyzer.ResultType,
	Requires:   nilaway.Analyzer.Requires,
}

var (
	// includeErrorsInFiles is a driver flag for specifying the list of file prefixes to only
	// report errors.
	includeErrorsInFiles string
	// excludeErrorsInFiles is a driver flag for specifying the list of file prefixes to not report
	// errors.
	excludeErrorsInFiles string

	errChildWaitTimeout = errors.New("child process wait timed out")
)

func run(pass *analysis.Pass) (interface{}, error) {
	enhanced := analysishelper.NewEnhancedPass(pass)
	// NilAway by default analyzes all packages, including dependencies. Even if specified to
	// exclude packages from analysis via configurations, NilAway can still report errors on
	// packages that are not analyzed if the nilness flow happens within the analyzed package, but
	// the flow concerns a struct that is in an excluded package. The usual way to handle them is
	// to suppress them at the driver level, but unitchecker does not support that yet. Therefore,
	// here we add extra logic to filter the errors.

	includes, err := parseFilePrefixes(includeErrorsInFiles)
	if err != nil {
		return nil, fmt.Errorf("parse file prefixes for error inclusion: %w", err)
	}
	excludes, err := parseFilePrefixes(excludeErrorsInFiles)
	if err != nil {
		return nil, fmt.Errorf("parse file prefixes for error exclusion: %w", err)
	}

	report := enhanced.Report
	enhanced.Report = func(diagnostic analysis.Diagnostic) {
		path := pass.Fset.File(diagnostic.Pos).Name()
		for _, prefix := range excludes {
			if strings.HasPrefix(path, prefix) {
				return
			}
		}
		for _, prefix := range includes {
			if strings.HasPrefix(path, prefix) {
				report(diagnostic)
				return
			}
		}
	}

	// Delegate the real analysis run to the original nilaway analyzer.
	return nilaway.Analyzer.Run(pass)
}

// parseFilePrefixes parses the comma-separated list of file prefixes, converts them to absolute
// file paths, and returns them as a slice.
func parseFilePrefixes(value string) ([]string, error) {
	if value == "" {
		return nil, nil
	}
	prefixes := strings.Split(value, ",")
	for i, prefix := range prefixes {
		absolute, err := filepath.Abs(prefix)
		if err != nil {
			return nil, fmt.Errorf("convert %q to absolute path: %w", prefix, err)
		}
		prefixes[i] = absolute
	}
	return prefixes, nil
}

// isProtocolInvocation reports whether the command-line arguments match the `go vet` unitchecker
// protocol: the single `-flags`/`-V=full` descriptor calls, or exactly one positional argument
// ending in `.cfg` (the per-compilation-unit description handed to a worker process).
func isProtocolInvocation(args []string, flags *flag.FlagSet) bool {
	if len(args) == 1 && strings.HasPrefix(args[0], "-") {
		protocolFlag := strings.TrimLeft(args[0], "-")
		if protocolFlag == "flags" || protocolFlag == "V=full" {
			return true
		}
	}
	positional := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			if len(positional) > 1 {
				return false
			}
			continue
		}
		if len(positional) != 0 {
			return false
		}
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if hasValue {
			continue
		}
		if f := flags.Lookup(name); f != nil && !isBoolFlag(f) {
			i++
		}
	}
	return len(positional) == 1 && strings.HasSuffix(positional[0], ".cfg")
}

func isBoolFlag(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

// vetCommand builds the `go vet -vettool=<tool>` argument vector that delegates direct invocations
// back through the worker protocol.
func vetCommand(tool string, args []string) []string {
	return append([]string{"go", "vet", "-vettool=" + tool}, args...)
}

// protocolFlagSet returns the flag set the worker protocol accepts, combining the lifted NilAway
// config flags with the standard `go vet` driver flags.
func protocolFlagSet() *flag.FlagSet {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	config.Analyzer.Flags.VisitAll(func(f *flag.Flag) { flags.Var(f.Value, f.Name, f.Usage) })
	flags.Bool("flags", false, "print analyzer flags in JSON")
	flags.Bool("V", false, "print version and exit")
	flags.Bool("json", false, "emit JSON output")
	flags.Int("c", -1, "display offending line with this many lines of context")
	flags.Bool("fix", false, "apply all suggested fixes")
	flags.Bool("diff", false, "with -fix, don't update the files, but print a unified diff")
	flags.String("include-errors-in-files", "", "")
	flags.String("exclude-errors-in-files", "", "")
	return flags
}

func runVetCommand(command *exec.Cmd) error {
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	termination := make(chan os.Signal, 2)
	signal.Notify(termination, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(termination)
	if err := command.Start(); err != nil {
		return err
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	return waitForVetCommand(wait, termination, newGraceTimer, func(sig os.Signal) error {
		return forwardTermination(command.Process, sig)
	}, func() error {
		return command.Process.Kill()
	})
}

func newGraceTimer() (<-chan time.Time, func()) {
	grace := time.NewTimer(time.Second)
	return grace.C, func() {
		if !grace.Stop() {
			select {
			case <-grace.C:
			default:
			}
		}
	}
}

func waitForVetCommand(
	wait <-chan error,
	termination <-chan os.Signal,
	grace func() (<-chan time.Time, func()),
	forward func(os.Signal) error,
	force func() error,
) error {
	select {
	case err := <-wait:
		return err
	case sig := <-termination:
		forwardErr := forward(sig)
		if errors.Is(forwardErr, os.ErrProcessDone) {
			joined, childErr := waitForChild(wait, grace)
			return joinChildLifecycleErrors(nil, nil, childErr, joined)
		}
		if forwardErr != nil {
			return joinAfterForce(wait, grace, forwardErr, force())
		}
		graceWait, stopGrace := grace()
		select {
		case childErr := <-wait:
			stopGrace()
			return childErr
		case <-graceWait:
			stopGrace()
			return joinAfterForce(wait, grace, nil, force())
		}
	}
}

func joinAfterForce(
	wait <-chan error,
	grace func() (<-chan time.Time, func()),
	forwardErr, forceErr error,
) error {
	joined, childErr := waitForChild(wait, grace)
	return joinChildLifecycleErrors(forwardErr, forceErr, childErr, joined)
}

func joinChildLifecycleErrors(forwardErr, forceErr, childErr error, joined bool) error {
	if !joined {
		return errors.Join(forwardErr, forceErr, errChildWaitTimeout)
	}
	if errors.Is(forceErr, os.ErrProcessDone) {
		forceErr = nil
	}
	return errors.Join(forwardErr, forceErr, childErr)
}

func waitForChild(wait <-chan error, grace func() (<-chan time.Time, func())) (joined bool, err error) {
	waitDeadline, stopWait := grace()
	defer stopWait()
	select {
	case childErr := <-wait:
		return true, childErr
	case <-waitDeadline:
		return false, nil
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		if status, ok := exitError.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return 128 + int(status.Signal())
		}
		if code := exitError.ExitCode(); code >= 0 {
			return code
		}
	}
	return 1
}

func main() {
	protocolFlags := protocolFlagSet()
	// Lift the flags from config.Analyzer to the top level so that users can specify them without
	// having to specify the analyzer name ("nilaway_config"). See the config analyzer docs for
	// details.
	config.Analyzer.Flags.VisitAll(func(f *flag.Flag) { flag.Var(f.Value, f.Name, f.Usage) })
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	flag.StringVar(&includeErrorsInFiles, "include-errors-in-files", workingDirectory,
		"A comma-separated list of file prefixes to report errors, default is current working directory.")
	flag.StringVar(&excludeErrorsInFiles, "exclude-errors-in-files", "",
		"A comma-separated list of file prefixes to exclude from error reporting. This takes precedence over include-errors-in-files.")

	if isProtocolInvocation(os.Args[1:], protocolFlags) {
		unitchecker.Main(Analyzer)
		return
	}

	commandArgs := vetCommand(os.Args[0], os.Args[1:])
	command := exec.Command(commandArgs[0], commandArgs[1:]...)
	if err := runVetCommand(command); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitCode(err))
	}
}
