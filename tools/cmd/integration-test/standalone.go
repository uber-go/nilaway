package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type StandaloneDriver struct {
	Dir string
}

func (d *StandaloneDriver) Run() (map[Position]string, error) {
	// Build NilAway first.
	if _, err := exec.Command("make", "build").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("build NilAway: %w", err)
	}

	// Run the NilAway binary on the integration test project, with redirects to an internal buffer.
	cmd := exec.Command("../../bin/nilaway", "-json", "-pretty-print=false", "./...")
	cmd.Dir = d.Dir
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("run nilaway: %w\n%s", err, buf.String())
	}

	// Parse the diagnostics.
	type diagnostic struct {
		Posn    string `json:"posn"`
		Message string `json:"message"`
	}
	// pkg name -> "nilaway" -> list of diagnostics.
	var result map[string]map[string][]diagnostic
	if err := json.NewDecoder(&buf).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode nilaway output: %w", err)
	}

	collected := make(map[Position]string)
	for _, m := range result {
		diagnostics, ok := m["nilaway"]
		if !ok {
			return nil, fmt.Errorf("expect \"nilaway\" key in result, got %v", m)
		}
		for _, d := range diagnostics {
			parts := strings.Split(d.Posn, ":")
			if len(parts) != 3 {
				return nil, fmt.Errorf("expect 3 parts in position string, got %v", d)
			}
			// Convert diagnostic output from NilAway to canonical form.
			line, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("convert line: %w", err)
			}
			pos := Position{Filename: parts[0], Line: line}
			if current, ok := collected[pos]; ok {
				return nil, fmt.Errorf("multiple diagnostics on the same line not supported, current: %q, got: %q", current, d.Message)
			}
			collected[pos] = d.Message
		}
	}

	return collected, nil
}
