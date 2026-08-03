package excludetestfiles

func testHelper() {
	var b *int
	// This nil dereference should NOT be reported when exclude-test-files is enabled,
	// since the report position is in a test file.
	print(*b)

	// Calling Deref with nil: the dereference happens in the non-test file, but
	// the nil originates from this test file, so the diagnostic should be excluded
	// because the nil flow involves a test file.
	Deref(nil)
}
