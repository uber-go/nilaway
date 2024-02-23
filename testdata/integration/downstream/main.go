package downstream

import "go.uber.org/nilaway/integration/upstream"

func test() {
	print(*upstream.NilableValue) //want "global variable `NilableValue` dereferenced"

	v := upstream.NilableFunc()
	print(*v) //want "result 0 of `NilableFunc\(\)` dereferenced"

	upstream.NonnilParam(nil)

	var s *upstream.S
	s.NonnilRecv()
	s.NilableRecv() // safe
}
