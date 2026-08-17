package common

func IsNilOrEmpty(p *int) bool { return p == nil || *p == 0 }

func CheckOther(p, q *int) bool { return q == nil }
