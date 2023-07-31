// Package ignoredpkg1 tests NilAway's ability to ignore packages that are configured to be ignored.
package ignoredpkg1

var GlobalVar *int

func main() {
	// Directly de-referencing a nil pointer, but it is OK since this package is ignored.
	print(*GlobalVar)
}
