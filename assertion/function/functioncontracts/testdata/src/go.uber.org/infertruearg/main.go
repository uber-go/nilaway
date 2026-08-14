package infertruearg

type Message struct {
	Field *int
}

func Compare(p *Message) bool { return p != nil } // want Compare:"&\\[{\\[\\] \\[\\] true => arg\\(0\\)\\.nonnil\\}\\]"

func Compound(p *Message) bool { return p != nil && p.Field != nil } // want Compound:"&\\[{\\[\\] \\[\\] true => arg\\(0\\)\\.nonnil\\}\\]"

func Negated(p *Message) bool { return !(p == nil) } // want Negated:"&\\[{\\[\\] \\[\\] true => arg\\(0\\)\\.nonnil\\}\\]"

type Validator struct{}

func (Validator) Method(p *Message) bool { return p != nil } // want Method:"&\\[{\\[\\] \\[\\] true => arg\\(0\\)\\.nonnil\\}\\]"

func Vacuous(p *Message) bool { return true }

func Or(p *Message, other bool) bool { return p != nil || other }

func Variadic(p *Message, rest ...bool) bool { return p != nil }

func Reassigned(p *Message) bool {
	p = nil
	return p != nil
}

func PhiFromCompare(p *Message) bool {
	result := false
	if p == nil {
		result = true
	}
	return result
}
