// Package prettyprint is meant to check if our pretty-print flag has effect.
package prettyprint

func main() {
	var a *int
	// Ensure that the ASCII escape code is in the want strings (such that the errors are pretty
	// printed).
	print(*a) //want "\u001B"
}
