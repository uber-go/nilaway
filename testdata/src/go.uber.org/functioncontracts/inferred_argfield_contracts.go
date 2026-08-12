package functioncontracts

type inferredArgFieldError struct{}

func (*inferredArgFieldError) Error() string { return "error" }

type inferredArgFieldMsg struct {
	CityID *int
	Name   *string
}

func inferredArgFieldValidate(msg *inferredArgFieldMsg) error {
	if msg.CityID == nil {
		return &inferredArgFieldError{}
	}
	return nil
}

type inferredArgFieldValidator struct{}

func (inferredArgFieldValidator) Validate(msg *inferredArgFieldMsg) error {
	if msg.CityID == nil {
		return &inferredArgFieldError{}
	}
	return nil
}

func inferredArgFieldNilInt() *int       { return nil }
func inferredArgFieldNilString() *string { return nil }

func inferredArgFieldSafe(msg *inferredArgFieldMsg) {
	msg.CityID = inferredArgFieldNilInt()
	err := inferredArgFieldValidate(msg)
	if err != nil {
		return
	}
	_ = *msg.CityID
}

func inferredArgFieldMethodSafe(msg *inferredArgFieldMsg) {
	msg.CityID = inferredArgFieldNilInt()
	err := inferredArgFieldValidator{}.Validate(msg)
	if err != nil {
		return
	}
	_ = *msg.CityID
}

func inferredArgFieldUnchecked(msg *inferredArgFieldMsg) {
	msg.Name = inferredArgFieldNilString()
	err := inferredArgFieldValidate(msg)
	if err != nil {
		return
	}
	_ = *msg.Name // want "dereferenced"
}

func inferredArgFieldWrongBranch(msg *inferredArgFieldMsg) {
	msg.CityID = inferredArgFieldNilInt()
	err := inferredArgFieldValidate(msg)
	if err != nil {
		_ = *msg.CityID // want "dereferenced"
	}
}

func inferredArgFieldReassigned(msg *inferredArgFieldMsg) {
	msg.CityID = inferredArgFieldNilInt()
	err := inferredArgFieldValidate(msg)
	err = nil
	if err == nil {
		_ = *msg.CityID // want "dereferenced"
	}
}

func inferredArgFieldCall(msg *inferredArgFieldMsg) {
	msg.CityID = inferredArgFieldNilInt()
	err := inferredArgFieldValidate(msg)
	if err != nil {
		return
	}
	inferredArgFieldConsume(msg)
	msg.CityID = nil
	_ = *msg.CityID // want "dereferenced"
}

func inferredArgFieldConsume(*inferredArgFieldMsg) {}
