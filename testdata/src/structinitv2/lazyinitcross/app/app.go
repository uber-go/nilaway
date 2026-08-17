package app

import "structinitv2/lazyinitcross/model"

// TRUE 0068 shape: pinned literal source, cross-package getter guard, alloc F only.
func guardedTrueShape() {
	o := &model.Outer{}
	if o.GetF() == nil {
		o.F = &model.Inner{}
	}
	_ = o.F.Val // F deref suppressed by cross-package getter contract
}

// negative control: pinned literal, NO guard -> MUST fire
func brokenNoGuard() {
	o := &model.Outer{}
	_ = o.F.Val //want "uninitialized field"
}

func mapGuarded(o *model.T) {
	if o.GetM() == nil {
		o.M = map[string]*model.Inner{}
	}
	o.M["k"] = &model.Inner{}
}

func mapNoGuard() {
	o := &model.T{}
	o.M["k"] = &model.Inner{} //want "uninitialized field"
}

// pinned-literal guarded map case: the guard+alloc must suppress the FP
func mapGuardedPinned() {
	o := &model.T{}
	if o.GetM() == nil {
		o.M = map[string]*model.Inner{}
	}
	o.M["k"] = &model.Inner{} // NO want: suppressed by Gate B getter-guard + MapWrittenTo
}

// pinned-literal, WRONG getter (returns a different field) -> MUST still fire
func mapGuardedWrongGetter() {
	o := &model.TWrong{}
	if o.GetM() == nil { // GetM returns MOther, not M
		o.M = map[string]*model.Inner{}
	}
	o.M["k"] = &model.Inner{} //want "uninitialized field"
}

func isValidGuarded() {
	e := &model.Event{}
	if !e.IsValid() {
		return
	}
	_ = *e.Code
}

func isValidWrongField() {
	e := &model.BadEvent{}
	if !e.IsValid() {
		return
	}
	_ = *e.Code //want "uninitialized field"
}

func isValidImpureHelper() {
	e := &model.SideEvent{}
	if !e.IsValid() {
		return
	}
	_ = *e.Code //want "uninitialized field"
}

func isValidNoGuard() {
	e := &model.Event{}
	_ = *e.Code //want "uninitialized field"
}

func valueReceiverMultiClause() {
	e := &model.ActionsEvent{}
	if !e.IsValid() {
		return
	}
	_ = *e.A
	_ = *e.B
	_ = *e.C
}

func multiClauseUncheckedField() {
	e := &model.UncheckedEvent{}
	if !e.IsValid() {
		return
	}
	_ = *e.B //want "uninitialized field"
}

func multiClauseNoProofHelper() {
	e := &model.OtherEvent{}
	if !e.IsValid() {
		return
	}
	_ = *e.A //want "uninitialized field"
}

func multiClauseNoGuard() {
	e := &model.ActionsEvent{}
	_ = *e.A //want "uninitialized field"
}
