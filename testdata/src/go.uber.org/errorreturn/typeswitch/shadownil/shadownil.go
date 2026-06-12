// Package shadownil tests that the `case nil` arm detection in type switches is not fooled by a
// user-declared type named `nil` -- the predeclared identifier is shadowable. Here `case nil`
// matches the concrete struct type, so the operand is non-nil in that arm and, conversely, can
// still be nil in the default arm.
package shadownil

type stringer interface{ String() string }

type nil struct{}

func (nil) String() string { return "type named nil" }

func deref(v stringer) {
	switch v.(type) {
	case nil:
		// v's dynamic type is the struct `nil`, i.e., v is non-nil here. This must NOT be
		// confused with a check for the nil value: the other arms remain unguarded.
	default:
		println(v.String()) //want "called `String"
	}
}

func useDeref() {
	var s stringer
	deref(s)
}
