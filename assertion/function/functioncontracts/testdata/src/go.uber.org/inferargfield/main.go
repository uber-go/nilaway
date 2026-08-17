package inferargfield

import (
	"errors"
	"fmt"
)

type Error struct{}

func (*Error) Error() string { return "error" }

type Embedded struct {
	EmbeddedField *int
}

type Msg struct {
	CityID *int
	Name   *string
	UserID *int
	Embedded
}

func Plain(msg *Msg) error { //want Plain:"field"
	if msg.CityID == nil {
		return &Error{}
	}
	return nil
}

func MultipleFields(msg *Msg) error { //want MultipleFields:"field"
	if msg.CityID == nil {
		return &Error{}
	}
	if msg.Name == nil {
		return &Error{}
	}
	return nil
}

func MultipleParams(first *Msg, second *Msg) error { //want MultipleParams:"field"
	if second.CityID == nil {
		return &Error{}
	}
	return nil
}

func Reversed(msg *Msg) error { //want Reversed:"field"
	if nil != msg.CityID {
		return nil
	}
	return &Error{}
}

func (m *Msg) Method(msg *Msg) error { //want Method:"field"
	if msg.Name == nil {
		return &Error{}
	}
	return nil
}

func Variadic(msg *Msg, rest ...int) error {
	if msg.CityID == nil {
		return &Error{}
	}
	return nil
}

func ValueStruct(msg Msg) error {
	if msg.CityID == nil {
		return &Error{}
	}
	return nil
}

func EmbeddedField(msg *Msg) error {
	if msg.EmbeddedField == nil {
		return &Error{}
	}
	return nil
}

func Store(msg *Msg) error {
	if msg.CityID == nil {
		msg.CityID = new(int)
	}
	return nil
}

func take(*Msg)      {}
func takeField(*int) {}

func AddressEscapes(msg *Msg) error {
	if msg.CityID == nil {
		return &Error{}
	}
	take(msg)
	return nil
}

func MissingProof(msg *Msg) error {
	if msg.CityID == nil {
		return nil
	}
	return &Error{}
}

func CallOnNilErrorPath(msg *Msg) error {
	if msg.CityID == nil {
		return nil
	}
	take(msg)
	return nil
}

func LoopFixpoint(msg *Msg, n int) error {
	for i := 0; i < n; i++ {
		if msg.CityID == nil {
			return &Error{}
		}
	}
	return nil
}

type Ctx struct{}

var ctx Ctx
var errNil = &Error{}

type svc struct{}

func (*svc) logAndHandleError(_ Ctx, err error, _ string) error { return err }

func (s *svc) ValidateQPos(msg *Msg) error {
	if msg.UserID == nil {
		return s.logAndHandleError(ctx, errNil, "missing")
	}
	return nil
}

func (s *svc) UnsoundValidator(msg *Msg) error {
	if msg.UserID == nil {
		return s.logAndHandleError(ctx, errNil, "missing")
	}
	return nil
}

func ErrorsNew(msg *Msg) error { //want ErrorsNew:"field"
	if msg.UserID == nil {
		return errors.New("missing")
	}
	return nil
}

func FmtErrorf(msg *Msg) error { //want FmtErrorf:"field"
	if msg.UserID == nil {
		return fmt.Errorf("missing")
	}
	return nil
}

func customError() error { return nil }

func CustomErrorConstructor(msg *Msg) error {
	if msg.UserID == nil {
		return customError()
	}
	return nil
}

func PassFieldToCall(msg *Msg) error {
	if msg.CityID == nil {
		return &Error{}
	}
	takeField(msg.CityID)
	return nil
}

var unrelated *int

func UnrelatedStore(msg *Msg, condition bool) error {
	if msg.CityID == nil {
		return &Error{}
	}
	if condition {
		unrelated = new(int)
	}
	return nil
}

func LoopStore(msg *Msg, n int) error {
	if msg.CityID == nil {
		return &Error{}
	}
	for i := 0; i < n; i++ {
		unrelated = new(int)
	}
	return nil
}
