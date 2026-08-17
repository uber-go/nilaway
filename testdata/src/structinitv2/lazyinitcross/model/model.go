package model

import "structinitv2/lazyinitcross/common"

type Inner struct{ Val *int }

type Outer struct{ F *Inner }

func (o *Outer) GetF() *Inner {
	if o == nil {
		return nil
	}
	return o.F
}

type Bad struct {
	F, Other *Inner
}

func (b *Bad) GetF() *Inner { return b.Other }

type SideEffect struct{ F *Inner }

func (s *SideEffect) GetF() *Inner {
	s.F = &Inner{}
	return s.F
}

type T struct{ M map[string]*Inner }

func (t *T) GetM() map[string]*Inner {
	if t == nil {
		return nil
	}
	return t.M
}

type TWrong struct {
	M      map[string]*Inner
	MOther map[string]*Inner
}

type Event struct{ Code *int }

func (e *Event) IsValid() bool { return !common.IsNilOrEmpty(e.Code) }

type BadEvent struct {
	Code  *int
	Other *int
}

func (e *BadEvent) IsValid() bool { return !common.IsNilOrEmpty(e.Other) }

type SideEvent struct{ Code *int }

func touch(*SideEvent) {}

func (e *SideEvent) IsValid() bool {
	touch(e)
	return !common.IsNilOrEmpty(e.Code)
}

type ActionsEvent struct{ A, B, C *int }

func (e *ActionsEvent) IsValid() bool {
	if common.IsNilOrEmpty(e.A) || common.IsNilOrEmpty(e.B) || common.IsNilOrEmpty(e.C) {
		return false
	}
	return true
}

type UncheckedEvent struct{ A, B *int }

func (e *UncheckedEvent) IsValid() bool { return !common.IsNilOrEmpty(e.A) }

type OtherEvent struct{ A, Q *int }

func (e *OtherEvent) IsValid() bool {
	if common.CheckOther(e.A, e.Q) {
		return false
	}
	return true
}

func (t *TWrong) GetM() map[string]*Inner {
	if t != nil {
		return t.MOther // WRONG: returns a different field
	}
	return nil
}
