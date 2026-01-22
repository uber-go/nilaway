// Package printfullfilepath is meant to check if our print-full-file-path flag has effect.
package printfullfilepath

func main() {
	var a *int
	// Ensure that more than two filename components are printed when the
	// print-full-file-path flag is enabled.
	print(*a) //want "testdata/src/printfullfilepath/main.go"
}
