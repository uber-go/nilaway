// Package excludetestfilesbaseline is the baseline for the exclude-test-files test.
// It verifies that diagnostics ARE produced for test file flows when the flag is disabled.
package excludetestfilesbaseline

func main() {
	var a *int
	print(*a) // want "dereferenced"
}
