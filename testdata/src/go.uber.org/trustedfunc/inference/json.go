package inference

import "encoding/json"

// testJSONUnmarshal exercises the `ErrorReturnNonnilArg` hook: `json.Unmarshal(data, &v)` populates
// `v`, so the pointee is treated as non-nil once the error return is checked to be nil.
func testJSONUnmarshal(data []byte) {
	// `err != nil` early return: pointee is non-nil on the fallthrough (error-is-nil) path.
	var v1 *int
	if err := json.Unmarshal(data, &v1); err != nil {
		return
	}
	print(*v1) // safe

	// `err == nil` positive check: pointee is non-nil inside the block.
	var v2 *int
	err := json.Unmarshal(data, &v2)
	if err == nil {
		print(*v2) // safe
	}

	// Error return not checked at all: no guarantee.
	var v3 *int
	json.Unmarshal(data, &v3)
	print(*v3) //want "unassigned variable `v3` dereferenced"

	// Error return discarded into the blank identifier: no guarantee.
	var v4 *int
	_ = json.Unmarshal(data, &v4)
	print(*v4) //want "unassigned variable `v4` dereferenced"

	// Dereference on the error path (`err != nil`): pointee is not guarded here.
	var v5 *int
	if err := json.Unmarshal(data, &v5); err != nil {
		print(*v5) //want "unassigned variable `v5` dereferenced"
	}
}
