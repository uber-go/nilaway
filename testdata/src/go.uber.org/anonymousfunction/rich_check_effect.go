package anonymousfunction

var dummy bool

type myErr2 struct{}

func (myErr2) Error() string { return "myErr2 message" }

var globalFunc = func() (*int, error) {
	if dummy {
		return nil, &myErr2{}
	}
	return new(int), nil
}

func testAnonErrReturningFunc(i int) {
	f1 := func() (*int, error) {
		if dummy {
			return nil, &myErr2{}
		}
		return new(int), nil
	}

	f2 := func() (*int, error) {
		if dummy {
			return new(int), &myErr2{}
		}
		return new(int), nil
	}

	f3 := func() (*int, error) {
		if dummy {
			return nil, &myErr2{}
		}
		return nil, nil
	}

	switch i {
	case 1:
		x, err := f1()
		if err != nil {
			return
		}
		_ = *x

	case 2:
		if x, err := f1(); err != nil {
			_ = *x // want "dereferenced"
		}

	case 3:
		x, err := f2()
		if err != nil {
			// safe since f2() always returns a non-nil value
			_ = *x
		}

	case 4:
		x, err := f3()
		if err != nil {
			return
		}
		// unsafe since f3() always returns a nil value
		_ = *x // want "dereferenced"

	case 5:
		x, err := func() (*int, error) {
			if dummy {
				return nil, &myErr2{}
			}
			return new(int), nil
		}()
		if err != nil {
			return
		}
		_ = *x

	case 6:
		if x2, err2 := func() (*int, error) {
			if dummy {
				return nil, &myErr2{}
			}
			return new(int), nil
		}(); err2 != nil {
			_ = *x2 // want "dereferenced"
		}

	case 7:
		x, err := func() (*int, error) {
			if dummy {
				return new(int), &myErr2{}
			}
			return new(int), nil
		}()
		if err != nil {
			// safe since f2() always returns a non-nil value
			_ = *x
		}

	case 8:
		x, err := func() (*int, error) {
			if dummy {
				return nil, &myErr2{}
			}
			return nil, nil
		}()
		if err != nil {
			return
		}
		// unsafe since the anonymous function always returns a nil value
		_ = *x // want "dereferenced"

	case 9:
		x, err := globalFunc()
		if err != nil {
			return
		}
		_ = *x
	}
}
