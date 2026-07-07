package typeswitch

import "errors"

var dummy int

func aa() (*int, error) {
	if dummy == 1 {
		return nil, errors.New("some error")
	}
	return new(int), nil
}

func bb() {
	ptr, err := aa()
	switch err.(type) {
	case nil:
		_ = *ptr // safe: err is nil in this case, so ptr must be non-nil
	}
}

// Same as bb, but the dereference happens in the default arm, where err can be non-nil and hence
// ptr can be nil: the diagnostic must be preserved.
func bbDefault() {
	ptr, err := aa()
	switch err.(type) {
	case nil:
	default:
		_ = *ptr //want "dereferenced"
	}
}

// A `case nil` arm must still be recognized as a guard when an ok-form type assertion on an
// *unrelated* variable precedes the switch (the gotest.tools/assert.NilError shape).
func check(t interface{}, v interface{}) {
	if _, ok := t.(interface{ Helper() }); ok {
		println("helper")
	}
	switch x := v.(type) {
	case nil:
		return
	case error:
		// Unreachable with v == nil: a nil interface value has no dynamic type, so it can only
		// match `case nil` above.
		println(x.Error())
	}
}

func returnsNilError() error {
	return nil
}

func useCheck() {
	check(0, returnsNilError())
}

// A literal nil argument flowing into a type switch with a `case nil` arm must not be reported.
func checkLitNil(v interface{}) {
	switch x := v.(type) {
	case nil:
		return
	case error:
		println(x.Error())
	}
}

func useCheckLitNil() {
	checkLitNil(nil)
}

// Matching any *interface* case arm (not just `case nil`) guarantees the operand is non-nil: a
// nil interface value has no dynamic type, so it can only match `case nil` or `default`. Hence no
// diagnostic here even without a `case nil` arm.
func checkNoNilArm(v interface{}) {
	switch x := v.(type) {
	case error:
		println(x.Error())
	}
}

func useCheckNoNilArm() {
	checkNoNilArm(nil)
}

// In the default arm, an ok-form type assertion on the bound variable keeps its use safe: no
// diagnostic.
func checkDefaultArm(v interface{}) {
	switch x := v.(type) {
	case error:
		println(x.Error())
	default:
		if y, ok := x.(interface{ String() string }); ok {
			println(y.String())
		}
	}
}

type stringer interface{ String() string }

// In the default arm, the operand can still be nil, so calling a method on the bound variable
// must be reported.
func checkDefaultArmDeref(v stringer) {
	switch x := v.(type) {
	case error:
		println(x.Error())
	default:
		println(x.String()) //want "called `String"
	}
}

func useCheckDefaultArmDeref() {
	checkDefaultArmDeref(nil)
}

// A multi-type clause containing nil must not mark the bound variable (which keeps the operand's
// static type) non-nil inside the body, while a subsequent interface arm is still recognized as
// non-nil. This also pins the chain walk over clauses listing multiple case types.
func checkMultiTypeClause(v stringer) {
	switch x := v.(type) {
	case nil, error:
		println(x.(error).Error())
	case interface{ Len() int }:
		println(x.Len()) // safe: nil can only match the `case nil, error` clause above
	}
}

// A concrete (non-interface) case arm must not mark the *bound variable* non-nil: the dynamic
// value can be a typed nil pointer even though the interface itself is non-nil.
func checkConcreteArm(v interface{}) *int {
	switch x := v.(type) {
	case *int:
		return x
	}
	return new(int)
}

// A parenthesized `case (nil):` must be treated identically to `case nil:`.
func bbParenNil() {
	ptr, err := aa()
	switch err.(type) {
	case (nil):
		_ = *ptr // safe: err is nil in this case, so ptr must be non-nil
	}
}

// A type-parameter case arm must not be treated as an interface arm: a value matching `case T`
// can still be a typed nil pointer (e.g., T instantiated with *int), just like a concrete-typed
// arm. This currently produces no diagnostic either way (NilAway's generics support does not yet
// track typed-nil flows through instantiations); the test pins that no condition is synthesized
// and nothing crashes.
func checkTypeParamArm[T stringer](v interface{}) {
	switch x := v.(type) {
	case T:
		println(x.String())
	}
}
