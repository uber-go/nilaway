// Package excludetestfiles is meant to check if the exclude-test-files flag has effect.
package excludetestfiles

// Deref dereferences the given pointer. When called with nil from a test file,
// the resulting diagnostic should be excluded since the nil flow involves a test file.
func Deref(p *int) {
	print(*p)
}

func main() {
	var a *int
	// Error in non-test file should still be reported even when exclude-test-files is enabled.
	print(*a) // want "unassigned variable `a`"
}
