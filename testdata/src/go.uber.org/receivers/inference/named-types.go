package inference

// Named Slice type

type myIntSlice []int

func (m myIntSlice) len() {
	print(len(m))
}

func (m myIntSlice) count() int {
	count := 0
	for _, e := range m {
		count = count + e
	}
	return count
}

func (m myIntSlice) fetch(i int) int {
	return m[i]
}

type T struct {
	f *int
}

type myTSlice []T

// Note to navigate our test setup, we need several duplicate fetch methods to ensure that the "want" message can be triggered
// for the appropriate fetch methods.

func (m myTSlice) fetch1(i int) *int {
	return m[i].f //want "sliced into"
}

func (m myTSlice) fetch2(i int) *int {
	return m[i].f
}

func (m myTSlice) fetch3(i int) *int {
	return m[i].f
}

func (m myTSlice) fetch4(i int) *int {
	return m[i].f
}

func (m myTSlice) fetch5(i int) *int {
	return m[i].f //want "sliced into"
}

func newMyIntSlice(input myIntSlice) myIntSlice {
	var result myIntSlice
	for _, t := range input {
		result = append(result, t)
	}
	return result
}

func testNamedSlice(i int) {
	switch i {
	case 0:
		res := map[string]myIntSlice{}
		res["abc"].len()

	case 1:
		m := myIntSlice{1, 2, 3}
		print(newMyIntSlice(m).count())
		print(newMyIntSlice(nil).count())

	case 2:
		var m myTSlice
		_ = m.fetch1(i) // error reported in "fetch1" method

	case 3:
		m := myTSlice{{f: new(int)}, {f: new(int)}, {f: new(int)}}
		_ = m.fetch2(i)

	case 4:
		var m myTSlice
		m = append(m, T{})
		m[0].f = nil

		// TODO: Error should be reported on the line below. It is currently not reported because of the suppression of
		//  struct field assignment logic that we added until we add object sensitivity for precise handling (issue #339).
		_ = *m.fetch3(i) // "dereferenced"

	case 5:
		var m myTSlice
		m = append(m, T{})
		m[0].f = new(int)

		_ = *m.fetch4(i)

	case 6:
		var m myTSlice
		_ = *m.fetch5(i) // error reported in "fetch5" method
	}
}

type myIntPointers []*int

func (m myIntPointers) fetch(i int) *int {
	for x := range m {
		if x == i {
			return m[x]
		}
	}
	return nil
}

func testNamedSliceOfPointers(i int) {
	switch i {
	case 0:
		var m myIntPointers
		_ = m.fetch(i)

	case 1:
		m := myIntPointers{new(int), new(int), new(int)}
		_ = m.fetch(i)

	case 2:
		var m myIntPointers
		_ = *m.fetch(i) //want "dereferenced"

	case 3:
		m := myIntPointers{new(int), new(int), new(int)}
		*m.fetch(i) = 42 //want "dereferenced"

	case 4:
		m := myIntPointers{nil, nil}
		_ = *m.fetch(i) //want "dereferenced"
	}
}

// Named Map type
type myIntMap map[string]*int

func (m myIntMap) get(key string) *int {
	return m[key]
}

func testNamedMap(i int) {
	switch i {
	case 0:
		var m myIntMap
		_ = *m.get("key") //want "dereferenced"

	case 1:
		m := myIntMap{"a": new(int), "b": new(int)}
		if v := m.get("a"); v != nil {
			_ = *v
		}

	case 2:
		m := myIntMap{"a": new(int), "b": new(int)}
		if v := m["a"]; v != nil {
			_ = *v
		}

	case 3:
		m := myIntMap{"a": new(int), "b": new(int)}
		if _, ok := m["a"]; ok {
			_ = *m["a"]
		}

	case 4:
		var m myIntMap
		_ = *m["a"] //want "dereferenced"
	}
}
