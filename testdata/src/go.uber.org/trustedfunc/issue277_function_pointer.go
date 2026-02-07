package trustedfunc

// Test for issue #277: function pointers as rich check effect functions

type S2 struct {
	f func() (*int, error)
}

// This is the false positive from the issue - should NOT report a warning
func testFunctionPointerErrorFP(s *S2) {
	v, err := s.f()
	if err != nil {
		return
	}
	_ = *v // no error - err was checked
}

// This SHOULD report a warning because we don't check err
func testFunctionPointerErrorNoCheck(s *S2) {
	v, _ := s.f()
	_ = *v // want "deref"
}

// Another checked example - should NOT report
func testFunctionPointerErrorChecked(s *S2) {
	v, err := s.f()
	if err != nil {
		return
	}
	_ = *v // no error - err was checked
}

// Test with local function variable - no check
func testLocalFunctionVarNoCheck() {
	var f func() (*int, error)
	v, _ := f()
	_ = *v // want "deref"
}

// Test with local function variable - checked
func testLocalFunctionVarChecked() {
	var f func() (*int, error)
	v, err := f()
	if err != nil {
		return
	}
	_ = *v // no error - err was checked
}

// Test with parameter function variable - no check
func testParamFunctionVarNoCheck(f func() (*int, error)) {
	v, _ := f()
	_ = *v // want "deref"
}

// Test with parameter function variable - checked
func testParamFunctionVarChecked(f func() (*int, error)) {
	v, err := f()
	if err != nil {
		return
	}
	_ = *v // no error - err was checked
}

// Test from the issue description - this is the false positive case
// Should NOT report a warning
func testme(s *S2) {
	v, err := s.f()
	if err != nil {
		return
	}
	_ = *v // no error - err was checked
}
