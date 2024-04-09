// Package disabled is meant to check if our group-error-messages flag has effect.
package disabled

// When the group-error-messages flag is set to false, the two dereference error messages should be reported separately.
func test() {
	var x *int
	_ = *x //want "dereferenced"
	_ = *x //want "dereferenced"
}
