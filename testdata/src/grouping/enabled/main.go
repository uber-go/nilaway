// Package grouping is meant to check if our group-error-messages flag has effect.
package enabled

// When the group-error-messages flag is set to true, the two dereference error messages should be grouped together.
func test() {
	var x *int
	_ = *x //want "Same nil source could also cause potential nil panic"
	_ = *x
}
