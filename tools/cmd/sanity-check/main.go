package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"go.uber.org/nilaway/tools/cmd/util/executil"
)

// CmdRunner is the interface for running commands.
type CmdRunner interface {
	// Run runs the program with the given arguments, and returns the stdout and any error occurred.
	// If the command starts but does not complete successfully, the error is of type *[exec.ExitError].
	// Other error types may be returned for other situations.
	Run(program string, args ...string) (string, error)
}

func run(runner CmdRunner, baseBranch, testBranch string, markDown bool) error {
	out, err := runner.Run("git", "status", "--porcelain=v1")
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if len(out) != 0 {
		return errors.New("git repository is not clean")
	}
	rootDir, err := runner.Run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("get root of git repository: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current working directory: %w", err)
	}
	if filepath.Clean(strings.TrimSpace(string(rootDir))) != filepath.Clean(wd) {
		return fmt.Errorf("this tool must be run at the root of the git repository: %q != %q", rootDir, wd)
	}

	out, err = runner.Run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}
	originalBranch := strings.TrimSpace(string(out))
	if testBranch == "" {
		testBranch = originalBranch
	}
	defer func() {
		_, err := runner.Run("git", "checkout", originalBranch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to checkout original branch %q: %v\n", originalBranch, err)
		}
	}()
	fmt.Printf("running sanity check on base branch %q and test branch %q\n", baseBranch, testBranch)

	var results [2][]byte
	for i, branch := range [2]string{baseBranch, testBranch} {
		commands := [][]string{
			{"git", "checkout", branch},
			{"make", "build"},
		}
		for _, command := range commands {
			_, err := runner.Run(command[0], command[1:]...)
			if err != nil {
				return fmt.Errorf("run command %q: %w", command, err)
			}
		}
		command := []string{"bin/nilaway", "-include-errors-in-files", "/", "-json", "-pretty-print", "false", "std"}
		out, err := runner.Run(command[0], command[1:]...)
		if err != nil {
			return fmt.Errorf("run command %q: %w", command, err)
		}
		results[i] = []byte(out)
	}

	base, err := parseDiagnostics(results[0])
	if err != nil {
		return fmt.Errorf("parse diagnostics: %w", err)
	}
	test, err := parseDiagnostics(results[1])
	if err != nil {
		return fmt.Errorf("parse diagnostics: %w", err)
	}

	if markDown {
		printMarkdown(base, test)
	} else {
		printDiff(base, test)
	}

	return nil
}

func parseDiagnostics(raw []byte) (map[string]bool, error) {
	// Package name -> "nilaway" -> slice of diagnostics.
	var output map[string]map[string][]struct {
		Posn    string `json:"posn"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		return nil, fmt.Errorf("marshal diagnostics: %w", err)
	}

	allDiagnostics := make(map[string]bool)
	for _, packages := range output {
		diagnostics, ok := packages["nilaway"]
		if !ok {
			continue
		}
		for _, d := range diagnostics {
			allDiagnostics[fmt.Sprintf("%s:%s", d.Posn, d.Message)] = true
		}
	}

	return allDiagnostics, nil
}

func printDiff(base, test map[string]bool) {
	fmt.Printf("base: %d diagnostics\n", len(base))
	fmt.Printf("test: %d diagnostics\n", len(test))
	fmt.Printf("detailed diff:\n")

	for d := range base {
		if !test[d] {
			lines := strings.Split(strings.TrimSpace(d), "\n")
			for i := range lines {
				lines[i] = "--- " + lines[i]
			}
			color.Red(strings.Join(lines, "\n"))
		}
	}
	for d := range test {
		if !base[d] {
			lines := strings.Split(strings.TrimSpace(d), "\n")
			for i := range lines {
				lines[i] = "+++ " + lines[i]
			}
			color.Green(strings.Join(lines, "\n"))
		}
	}
}

func printMarkdown(base, test map[string]bool) {
	var pluses, minuses []string
	for d := range base {
		if !test[d] {
			minuses = append(minuses, d)
		}
	}
	for d := range test {
		if !base[d] {
			pluses = append(pluses, d)
		}
	}

	if len(pluses) == 0 && len(minuses) == 0 {
		fmt.Printf(":white_check_mark: NilAway errors reported on stdlib are equal")
	}

	fmt.Printf("| Branch | Errors\n")
	fmt.Printf("| --- | ---\n")
	fmt.Printf("| base | %d\n", len(base))
	fmt.Printf("| test | %d\n", len(test))
}

func main() {
	fset := flag.NewFlagSet("sanity-check", flag.ExitOnError)
	baseBranch := fset.String("base-branch", "main", "the base branch to compare against")
	testBranch := fset.String("test-branch", "", "the test branch to run sanity check (default current branch)")
	markDown := fset.Bool("markdown", false, "output in markdown format")
	err := fset.Parse(os.Args[1:])
	if err != nil {
		fmt.Printf("failed to parse flags: %v\n", err)
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Create a concrete command runner.
	runner := &executil.CmdRunner{}
	if err := run(runner, *baseBranch, *testBranch, *markDown); err != nil {
		fmt.Printf("failed to run sanity check: %v\n", err)
		var e *exec.ExitError
		if errors.As(err, &e) {
			fmt.Printf("failed command output: %v", string(e.Stderr))
		}
		os.Exit(1)
	}
}
