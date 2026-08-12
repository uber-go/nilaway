package infertruerecvfield

type Event struct {
	Action *int
	Other  *int
}

func (e *Event) PointerValid() bool { return e.Action != nil } // want PointerValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\}\\]"

func (e Event) ValueValid() bool { return e.Action != nil } // want ValueValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\}\\]"

func (e Event) CompoundValid() bool { return e.Action != nil && e.Other != nil } // want CompoundValid:"&\\[{\\[\\] \\[\\] true => recv.field\\(0\\)\\.nonnil\\} {\\[\\] \\[\\] true => recv.field\\(1\\)\\.nonnil\\}\\]"

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
