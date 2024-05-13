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
	// TODO: FP: this should be safe due to the contract. We should remove this once contract
	//  support is fixed in NilAway.
	print(*r) //want "result 0 of `NonnilToNonnil\(\)` dereferenced"
}