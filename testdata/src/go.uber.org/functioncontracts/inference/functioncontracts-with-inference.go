// <nilaway contract enable>
package inference

import "math/rand"

// Test the contracted function contains a full trigger nilable -> return 0.
// contract(nonnil -> nonnil)
func fooReturn(x *int) *int { // want "^ Annotation on Result 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	if x != nil {
		// Return nonnil
		return new(int)
	}
	// Return nonnil or nil randomly
	if rand.Float64() > 0.5 {
		return new(int)
	} else {
		return nil
	}
}

func barReturn1() {
	n := 1
	a1 := &n
	b1 := fooReturn(a1)
	print(*b1) // No "nilable value dereferenced" wanted
}

func barReturn2() {
	var a2 *int
	b2 := fooReturn(a2)
	print(*b2) // "nilable value dereferenced" wanted
}

// Test the contracted function contains a full trigger param 0 -> nonnil.
// contract(nonnil -> nonnil)
func fooParam(x *int) *int { // want "^ Annotation on Param 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	sink(*x) // "nilable value dereferenced" wanted
	return new(int)
}

func barParam1() {
	n := 1
	a1 := &n
	b1 := fooParam(a1)
	print(*b1)
}

func barParam2() {
	var a2 *int
	b2 := fooParam(a2)
	print(*b2)
}

func sink(v int) {}

// Test the contracted function contains a full trigger param 0 -> return 0.
// contract(nonnil -> nonnil)
func fooParamAndReturn(x *int) *int { // want "^ Annotation on Result 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	return x
}

func barParamAndReturn1() {
	n := 1
	a1 := &n
	b1 := fooParamAndReturn(a1)
	print(*b1) // No "nilable value dereferenced" wanted
}

func barParamAndReturn2() {
	var a2 *int
	b2 := fooParamAndReturn(a2)
	print(*b2) // "nilable value dereferenced" wanted
}

// Test the contracted function contains another contracted function.
// contract(nonnil -> nonnil)
func fooNested(x *int) *int {
	return fooBase(x)
}

// contract(nonnil -> nonnil)
func fooBase(x *int) *int { // want "^ Annotation on Result 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	return x
}

func barNested1() {
	n := 1
	a1 := &n
	b1 := fooNested(a1)
	print(*b1) // No "nilable value dereferenced" wanted
}

func barNested2() {
	var a2 *int
	b2 := fooNested(a2)
	print(*b2) // "nilable value dereferenced" wanted
}

// Test the contracted function is called by another function.
// contract(nonnil -> nonnil)
func fooParamCalledInAnotherFunction(s *int) *int { // want "^ Annotation on Param 0.*\n.*Must be NILABLE.*\n.*AND.*\n.*Must be NONNIL.*NONNIL$"
	sink(*s)
	return new(int)
}

func barParamCalledInAnotherFunction() {
	var s *int
	call(fooParamCalledInAnotherFunction(s))
}

func call(x *int) {}
