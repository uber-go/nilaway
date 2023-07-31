// Package ignoredpkg2 tests NilAway's ability to ignore packages that are configured to be ignored.
package ignoredpkg2

var GlobalVar *int

func main() {
	// Directly de-referencing a nil pointer, but it is OK since this package is ignored.
	print(*GlobalVar)
}
