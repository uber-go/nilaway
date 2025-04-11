package upstream

var _caseNo int

var NilableValue *int = nil

func NilableFunc() *int { return nil }

func NonnilParam(v *int) {
	print(*v) //want "function parameter `v` dereferenced"
}

type S struct {
	f int
}

func (s *S) NilableRecv() {
	if s == nil {
		print("s is nil")
	} else {
		print(s.f)
	}
}

func (s *S) NonnilRecv() {
	print(s.f) //want "read by method receiver `s` accessed field `f`"
}

func dereference() {
	var v *int
	print(*v) //want "unassigned variable `v` dereferenced"
}

func DerefNoLintLine(v *int) {
	print(*v) //nolint:nilaway
	print(*v) // nolint:all
	print(*v) //     nolint:     nilaway
	print(*v) ////nolint:nilaway
	print(*v) //nolint
}

//nolint:nilaway
func DerefNoLintFunc(v *int) {
	print(*v)
}

func Deref(v *int, ) {
	switch _caseNo {
	case 1:
		print(*v) //want "function parameter `v` dereferenced"
	case 2:
		print(*v) //nolint:other_linter //want "function parameter `v` dereferenced"
	}
}
