package executil

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// CmdRunner is the command runner that calls [exec] package to run commands.
type CmdRunner struct{}

// Run delegates the command to [exec.Command] and returns the stdout of the command. It also
// prints the stdout and stderr of the command.
func (*CmdRunner) Run(program string, args ...string) (string, error) {
	var stdout, stderr, combined bytes.Buffer
	cmd := exec.Command(program, args...)
	cmd.Stdout = io.MultiWriter(&stdout, &combined)
	cmd.Stderr = io.MultiWriter(&stderr, &combined)

	fmt.Printf("Running `%s %s`\n", program, strings.Join(args, " "))
	defer func() {
		fmt.Printf(combined.String() + "\n")
	}()

	err := cmd.Run()
	return stdout.String(), err
}
