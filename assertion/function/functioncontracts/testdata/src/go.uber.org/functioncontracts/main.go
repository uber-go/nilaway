/*
Test functionality of parsing function contracts.
*/
package functioncontracts

// contract(nonnil -> nonnil)
func f1(x *int) *int {
	if x == nil {
		return x
	}
	return new(int)
}

// contract(nonnil -> true)
func f2(x *int) bool {
	if x == nil {
		return false
	}
	return true
}

// contract(nonnil -> false)
func f3(x *int) bool {
	if x == nil {
		return true
	}
	return false
}

// contract(_, nonnil -> nonnil, true)
func multipleValues(key string, deft *int) (*int, bool) {
	m := map[string]*int{}
	x, _ := m[key]
	if x != nil {
		return x, true
	}
	if deft != nil {
		return deft, true
	}
	return nil, false
}

// contract(_, nonnil -> nonnil, true)
// contract(nonnil, _ -> nonnil, true)
func multipleContracts(x *int, y *int) (*int, bool) {
	if x == nil && y == nil {
		return nil, false
	}
	return new(int), true
}

// This contract `// contract(nonnil -> nonnil)` does not hold for the function because the
// function has no param or return. Only a contract in its own line should be parsed, not even `//
// contract(nonnil -> nonnil)`.
func contractCommentInOtherLine() {}
