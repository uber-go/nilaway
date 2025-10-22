package typeswitch

var dummy int

func aa() (*int, error) {
	return new(int), nil
}

func bb() {
	ptr, err := aa()
	switch err.(type) {
	case nil:
		_ = *ptr // safe: err is nil in this case, so ptr must be non-nil
	}
}

