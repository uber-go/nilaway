package downstream

import "go.uber.org/nilaway/integration/upstream"

func test() {
	print(*upstream.NilableValue) //want "global variable `NilableValue` dereferenced"

	v := upstream.NilableFunc()
	print(*v) //want "result 0 of `NilableFunc\(\)` dereferenced"

	upstream.NonnilParam(nil)

	var s *upstream.S
	// Error: dereference a nilable receiver in upstream package, which expects the receiver to be non-nil.
	s.NonnilRecv()
	s.NilableRecv() // safe
}
