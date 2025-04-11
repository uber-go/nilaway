package downstream

import "go.uber.org/nilaway/integration/simple/upstream"

func test() {
	print(*upstream.NilableValue) //want "global variable `NilableValue` dereferenced"

	v := upstream.NilableFunc()
	print(*v) //want "result 0 of `NilableFunc\(\)` dereferenced"

	upstream.NonnilParam(nil)

	var s *upstream.S
	s.NonnilRecv()
	s.NilableRecv() // safe
}


func GiveUpstreamDeref() {
	// Nil source is in the downstream package. However, the nil sink (dereference) is happening
	// in the upstream package. NilAway should report the violation in the upstream package _when_
	// analyzing the downstream package.
	upstream.Deref(nil)
}

func GiveUpstreamDerefNoLint() {
	// Similar to `GiveUpstreamDeref`. However, here the upstream package has `//nolint` mark for
	// NilAway and therefore NilAway should not report the violation. This is to test our
	// cross-package nolint suppression support.
	upstream.DerefNoLintLine(nil)
	upstream.DerefNoLintFunc(nil)
	upstream.DerefNoLintFile(nil)
}

func localNoLintLint() {
	var p *int
	print(*p) //nolint:nilaway
	print(*p) //nolint:all
	print(*p) // nolint     :   nilaway // Explanation
	print(*p) //nolint
	print(*p) ////nolint:nilaway
}

//nolint:nilaway
func localNoLintFunc() {
	var p *int
	print(*p)
	print(*p)
	print(*p)
}
