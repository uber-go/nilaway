package excludetestfilesbaseline

func testHelper() {
	var b *int
	print(*b) // want "unassigned variable `b` dereferenced"
}
