package functioncontracts

type inferredTrueArgMessage struct {
	Field *int
}

func inferredTrueArgPredicate(msg *inferredTrueArgMessage) bool {
	return msg != nil && msg.Field != nil
}

func inferredTrueArgIsNil(msg *inferredTrueArgMessage) bool {
	return msg == nil
}

func inferredTrueArgVacuous(*inferredTrueArgMessage) bool { return true }

type inferredTrueArgInterface interface {
	Valid(*inferredTrueArgMessage) bool
}

type inferredTrueArgValidator struct{}

func (inferredTrueArgValidator) Valid(msg *inferredTrueArgMessage) bool {
	return msg != nil
}

func inferredTrueArgPositive(msg *inferredTrueArgMessage) {
	if inferredTrueArgPredicate(msg) {
		_ = *msg
	}
	if !inferredTrueArgIsNil(msg) {
		_ = *msg
	}
}

func inferredTrueArgOutside(msg *inferredTrueArgMessage) {
	msg = nil
	if inferredTrueArgPredicate(msg) {
		return
	}
	_ = *msg // want "dereferenced"
}

func inferredTrueArgVacuousUse(msg *inferredTrueArgMessage) {
	msg = nil
	if inferredTrueArgVacuous(msg) {
		_ = *msg // want "dereferenced"
	}
}

func inferredTrueArgInterfaceUse(msg *inferredTrueArgMessage) {
	msg = nil
	var validator inferredTrueArgInterface = inferredTrueArgValidator{}
	if validator.Valid(msg) {
		_ = *msg // want "dereferenced"
	}
}

var inferredTrueArgToggle bool

func inferredTrueArgToggling() *inferredTrueArgMessage {
	inferredTrueArgToggle = !inferredTrueArgToggle
	if inferredTrueArgToggle {
		return &inferredTrueArgMessage{}
	}
	return nil
}

func inferredTrueArgRepeatedCall() {
	if inferredTrueArgPredicate(inferredTrueArgToggling()) {
		_ = *inferredTrueArgToggling() // want "dereferenced"
	}
}
