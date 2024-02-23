package upstream

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
