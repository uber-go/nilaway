package inference

import cockroachdbErrors "stubs/github.com/cockroachdb/errors"

// cockroachdbErrorsNewIsNonNil checks that `cockroachdb/errors.New` is
// modeled as returning a non-nil error.
func cockroachdbErrorsNewIsNonNil() {
	err := cockroachdbErrors.New("some error")
	print(err.Error())
}

// cockroachdbErrorsJoinIsNonNil checks that `cockroachdb/errors.Join` is
// modeled as returning a non-nil error.
// `Join` can return nil if all arguments are nil, but we model it as non-nil
// for simplicity. This is a conscious trade-off.
func cockroachdbErrorsJoinIsNonNil() {
	err := cockroachdbErrors.Join()
	print(err.Error())
}
