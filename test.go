package nilaway

type Test struct {
	nilField *int
}

func Main() {
	t := &Test{}

	if false {
		A(t)

		return
	}

	
	B(t)
}

func A(t *Test) {
	t.nilField = nil
}

func B(t *Test) {
	println(*t.nilField)
}
