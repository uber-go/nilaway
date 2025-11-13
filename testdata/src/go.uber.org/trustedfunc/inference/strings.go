package inference

import "strings"

func stringsTest(c string) {
	switch c {
	case "strings.Split":
		// The result is guaranteed to be non-nil with at least one element.
		res := strings.Split("", " ")
		print(res[0])
	case "strings.Split (false negative)":
		// The result is still non-nil, but an empty slice.
		res := strings.Split("", "")
		// Technically this is unsafe, but it falls out of scope for NilAway.
		print(res[0])
	}
}
