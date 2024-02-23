package upstream

// NonnilToNonnil returns a nonnil pointer if the input is nonnil, otherwise a nil pointer is
// returned. It is meant to be called in downstream packages.
func NonnilToNonnil(v *int) *int {
	if v != nil {
		a := 1
		return &a
	}
	return nil
}
