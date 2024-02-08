package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/tools/go/packages"
)

type Position struct {
	Filename string
	Line     int
}

type Driver interface {
	Run() (map[Position]string, error)
}

func CollectGroundTruths(dir string, wd string) (map[Position]*regexp.Regexp, error) {
	if err := os.Chdir(dir); err != nil {
		return nil, fmt.Errorf("chdir: %w", err)
	}
	defer func() {
		// Switch back to the original directory.
		os.Chdir(wd)
	}()

	// First load all packages.
	config := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedFiles | packages.NeedTypes}
	pkgs, err := packages.Load(config, "./...")
	if err != nil {
		return nil, fmt.Errorf("load packages: %w", err)
	}

	// Traverse all comment nodes and collect corresponding comments with "want" strings.
	truths := make(map[Position]*regexp.Regexp)
	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			for _, group := range f.Comments {
				for _, comment := range group.List {
					text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
					if !strings.HasPrefix(text, "want ") {
						continue
					}
					text = strings.Trim(text[5:], "\"")
					pos := pkg.Fset.Position(group.Pos())
					p := Position{Filename: pos.Filename, Line: pos.Line}
					truths[p] = regexp.MustCompile(text)
				}
			}
		}
	}

	return truths, nil
}

func CompareDiagnostics(truth map[Position]*regexp.Regexp, collected map[Position]string) error {
	// Errors will be joined together.
	var err error

	// Keep track of the positions that we have seen.
	hit := make(map[Position]bool, len(truth))
	for pos, got := range collected {
		want, ok := truth[pos]
		if !ok {
			err = errors.Join(err, fmt.Errorf("unexpected diagnostic at %s:%d:\n\tgot :%q", pos.Filename, pos.Line, got))
			continue
		}
		hit[pos] = true
		if !want.MatchString(got) {
			err = errors.Join(err, fmt.Errorf("diagnostic mismatch at %s:%d:\n\twant: %q\n\tgot : %q", pos.Filename, pos.Line, want, got))
			continue
		}
	}

	// Check for missing diagnostics.
	for pos, want := range truth {
		if hit[pos] {
			continue
		}
		err = errors.Join(err, fmt.Errorf("missing diagnostic at %s:%d:\n\twant: %q", pos.Filename, pos.Line, want))
	}

	return err
}

func Run() error {
	// Make sure we are at the root of the git repository.
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return fmt.Errorf("get root of git repository: %w", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	if dir := strings.TrimSpace(string(out)); dir != wd {
		return fmt.Errorf("not at the root of the git repository: %q != %q", dir, wd)
	}
	// Set up the root directory for the integration test project.
	dir := filepath.Join(wd, "testdata", "integration")

	// Collect ground truths first.
	truths, err := CollectGroundTruths(dir, wd)
	if err != nil {
		return fmt.Errorf("collect want strings: %w", err)
	}

	drivers := []Driver{
		&StandaloneDriver{Dir: dir},
	}
	for _, driver := range drivers {
		name := reflect.TypeOf(driver).Elem().Name()
		fmt.Printf("--- Running integration tests using %q driver\n", name)
		collected, err := driver.Run()
		if err != nil {
			return fmt.Errorf("%q driver: %w", name, err)
		}
		if err := CompareDiagnostics(truths, collected); err != nil {
			return fmt.Errorf("diagnostics mismatch: \n%w", err)
		}
	}

	return nil
}

func main() {
	if err := Run(); err != nil {
		fmt.Printf("Integration test failed: %s\n", err)
		os.Exit(1)
	}
	fmt.Println("PASSED")
}
