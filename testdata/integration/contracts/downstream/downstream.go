package downstream

import "go.uber.org/nilaway/integration/contracts/upstream"

func NonnilToNonnil(v *int) *int {
	if v != nil {
		a := 1
		return &a
	}
	return nil
}

func GiveNil() {
	r := NonnilToNonnil(nil)
	print(*r) //want "result 0 of `NonnilToNonnil\(\)` dereferenced"
}

func GiveNonnil() {
	a := 1
	r := NonnilToNonnil(&a)
	print(*r) // Safe!
}

func GiveUpstreamNil() {
	r := upstream.NonnilToNonnil(nil)
	print(*r) //want "result 0 of `NonnilToNonnil\(\)` dereferenced"
}

func GiveUpstreamNonnil() {
	a := 1
	r := upstream.NonnilToNonnil(&a)
	print(*r) // Safe due to the contract!
}
