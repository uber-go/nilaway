/*
This is a test for checking affiliations anlysis for struct fields (embedded or explicit) declared as interfaces,
and instantiated with structs that implement the interface.
*/
package embedding

type I interface {
	foo(x *int) *int
}

// below test checks struct embedding at depth 1 (T embeds S)
type A1 struct{}

func (A1) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

type B1 struct {
	A1
}

func testEmbeddingDepth1() *int {
	var i I
	i = B1{}
	return i.foo(nil) // (error reported at the deref inside A1.foo())
}

// below test checks struct embedding at arbitrary depth (e.g., depth = 5, A2 embeds B2 embeds C2 embeds D2 embeds E2)
type A2 struct {
	B2
}

type B2 struct {
	C2
}

type C2 struct {
	D2
}

type D2 struct {
	E2
}

type E2 struct {
	f *int
}

func (e *E2) foo(x *int) *int {
	if e.f != nil {
		return e.f
	}
	_ = *x //want "passed as parameter"
	return x
}

func testEmbeddingDepth5() {
	var i I = &A2{}
	_ = i.foo(nil) // (error reported at the deref inside E2.foo())
}

// below test checks overriding of struct methods. A3 implements I.foo() violating the contravariance property of parameters.
// Now B3 embeds A3 and overrides the implementation of foo() by making it contravariance safe. Now instantiating I with B3
// should not report an error, while instantiating I with A3 should report an error as demonstrated below.
type A3 struct {
	f *int
}

func (A3) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

type B3 struct {
	A3
}

func (B3) foo(x *int) *int {
	if x == nil {
		z := 0
		return &z
	}
	return x
}

func testOverridding() {
	var i1 I = &B3{}
	i1.foo(nil) // safe, since B3.foo() guards its parameter against nil

	var i2 I = &A3{}
	i2.foo(nil) // (error reported at the deref inside A3.foo())
}

// below test checks anonymous fields in structs
type A4 struct{}

func (A4) foo(x *int) *int {
	_ = *x //want "passed as arg"
	return x
}

type B4 struct {
	f int
	A4
}

func (b *B4) foo(x *int) *int {
	if x == nil {
		return &b.f
	}
	return x
}

func testAnonymousFields(cond bool) *int {
	b := B4{}
	if cond {
		return b.A4.foo(nil) // (error reported at the deref inside A4.foo())
	}
	return b.foo(nil) // safe, since B4.foo() guards its parameter against nil
}

// below test checks embedding of multiple structs
type A5 struct {
	B5
	C5
}

type B5 struct{}

func (B5) foo(x *int) *int {
	_ = *x //want "passed as arg"
	return x
}

type C5 struct{}

func (C5) foo(x *int) *int {
	_ = *x //want "passed as arg"
	return x
}

func testEmbeddingMultipleStructs() {
	a := &A5{}
	_ = a.B5.foo(nil) // (error reported at the deref inside B5.foo())
	_ = a.C5.foo(nil) // (error reported at the deref inside C5.foo())
}

// below test checks for recursive embedding of structs
type A6 struct{}

func (A6) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

type B6 struct {
	A6
	*B6
}

func testRecursion() *int {
	var i I
	i = B6{}
	return i.foo(nil) // (error reported at the deref inside A6.foo())
}

// below test checks embedding of multiple interfaces within a struct, and embedding of interfaces within an interface
type J interface {
	bar() *int
}

type A9 struct {
	I
	J
}

type B9 struct{}

func (B9) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

type C9 struct{}

func (*C9) bar() *int {
	return nil
}

func testMultipleEmbeddedInterfaces() {
	a9 := &A9{I: &B9{}, J: &C9{}}
	_ = a9.foo(nil) // (error reported at the deref inside B9.foo())
	// Both nil flows into J.bar()'s result (from C9.bar() and A7.bar() below) are reported at
	// this first dereference of the interface method result.
	_ = *a9.bar() //want "returned" "returned"
}

type IandJ interface {
	I
	J
}

type A7 struct{}

func (*A7) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

func (*A7) bar() *int {
	return nil
}

func testEmbeddingInterfaceInInterface() {
	var i IandJ = &A7{}
	_ = i.foo(nil) // (error reported at the deref inside A7.foo())
	_ = *i.bar()   // (the A7.bar() nil flow through J.bar() is reported at the deref in testMultipleEmbeddedInterfaces() above)
}

// below test checks embedding of interface within a struct
type A8 struct{}

func (*A8) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

// B8 embeds I, but does not itself implement I.foo()
type B8 struct {
	I
}

// C8 embeds I, and implements I.foo()
type C8 struct {
	I
}

func (*C8) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

// D8 declares I as field g, but does not itself implement I.foo()
type D8 struct {
	g I
}

type E8 struct{}

func (*E8) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

func testEmbeddingInterfaceInStruct(x int) *int {
	switch x {
	case 1:
		// TODO: currently such "empty" implementations (meaning B8 does not actually implement the methods of I, but is
		//  still considered to implement I since it embeds I) are not analyzed.
		var i1 I = &B8{}
		return i1.foo(nil)
	case 2:
		var i2 I = &B8{&A8{}}
		return i2.foo(nil) // (error reported at the deref inside A8.foo())
	case 3:
		var i3 I = &C8{}
		return i3.foo(nil) // (error reported at the deref inside C8.foo())
	case 4:
		d8 := &D8{g: &E8{}}
		return d8.g.foo(nil) // (error reported at the deref inside E8.foo())
	}
	return &x
}

// below test checks nested structs
type A10 struct {
	I
}

type B10 struct {
	I
}

type C10 struct {
	D10
}

type D10 struct{}

func (*D10) foo(x *int) *int {
	_ = *x //want "passed as parameter"
	return x
}

func testNestedStructs() {
	a := &A10{&B10{&C10{D10: D10{}}}}
	_ = a.foo(nil) // (error reported at the deref inside D10.foo())
}

// below test checks a non-trivial case simulated from https://github.com/golang/go/pull/60823
type Conn interface {
	RemoteAddr() Addr
}

type Addr interface {
	String() string
}

type httpConn struct {
	rwc Conn
}

func (c *httpConn) serve() {
	_ = c.rwc.RemoteAddr().String() //want "returned"
}

type netConn struct{}

func (c *netConn) RemoteAddr() Addr {
	if true {
		return nil
	}
	return &addrImpl{}
}

type addrImpl struct{}

func (a *addrImpl) String() string {
	return ""
}

func main() {
	c := &httpConn{
		rwc: &netConn{},
	}
	c.serve()
}
