package infertruerecvfield

type Event struct {
	Action *int
	Other  *int
}

func IsNilOrEmpty(p *int) bool { return p == nil || *p == 0 } // want IsNilOrEmpty:"&\\[{\\[\\] \\[\\] false => arg\\(0\\)\\.nonnil\\}\\]"

func DerefOnlyHelper(p *int) bool {
	if p == nil {
		return false
	}
	_ = *p
	return true
}

func sinkImpure(*int) {}

func Impure(p *int) bool { sinkImpure(p); return p == nil }

func (e *Event) PointerValid() bool { return e.Action != nil } // want PointerValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\}\\]"

func (e *Event) HelperValid() bool { return !IsNilOrEmpty(e.Action) } // want HelperValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\}\\]"

func (e *Event) WrongHelperValid() bool { return !IsNilOrEmpty(e.Other) } // want WrongHelperValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(1\\)\\.nonnil\\}\\]"

func (e Event) ValueValid() bool { return e.Action != nil } // want ValueValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\}\\]"

func (e Event) CompoundValid() bool { return e.Action != nil && e.Other != nil } // want CompoundValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\} {\\[\\] \\[\\] true => recv.field\\(1\\)\\.nonnil\\}\\]"

func (e *Event) MultiClauseValid() bool { // want MultiClauseValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\} {\\[\\] \\[\\] true => recv.field\\(1\\)\\.nonnil\\}\\]"
	if IsNilOrEmpty(e.Action) || IsNilOrEmpty(e.Other) {
		return false
	}
	return true
}

func (e *Event) PartialClauseValid() bool { return !IsNilOrEmpty(e.Action) } // want PartialClauseValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\}\\]"

func sinkImpureEvent(*Event) {}

func (e *Event) ImpureMultiClause() bool {
	sinkImpureEvent(e)
	return !IsNilOrEmpty(e.Action) && !IsNilOrEmpty(e.Other)
}

func (e Event) Vacuous() bool { return true }

func (e Event) Or(other bool) bool { return e.Action != nil || other }

func (e *Event) NotChecked() bool { return e.Action == nil }

func (e *Event) Mutated() bool {
	e.Action = nil
	return e.Action != nil
}

func (e *Event) MutateOther() bool { e.Other = nil; return e.Action != nil }

func sink(**int) bool { return true }

func (e Event) PassFieldAddr() bool { return sink(&e.Action) }
