/*
This is a test for checking affiliations anlysis for struct fields (embedded or explicit) declared as interfaces,
and instantiated with structs that implement the interface.

<nilaway no inference>
*/
package embedding

type I interface {
	// nilable(x)
	foo(x *int) *int
}

// below test checks struct embedding at depth 1 (T embeds S)
type A1 struct{}

func (A1) foo(x *int) *int { //want "nilable value could be passed as param"
	return x
}

type B1 struct {
	A1
}

func testEmbeddingDepth1() *int {
	var i I
	i = B1{}
	return i.foo(nil) // (error reported at A1.foo() definition)
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

func (e *E2) foo(x *int) *int { //want "nilable value could be passed as param"
	if e.f != nil {
		return e.f
	}
	return x
}

func testEmbeddingDepth5() {
	var i I = &A2{}
	_ = i.foo(nil) // (error reported at E2.foo() definition)
}

// below test checks overriding of struct methods. A3 implements I.foo() violating the contravariance property of parameters.
// Now B3 embeds A3 and overrides the implementation of foo() by making it contravariance safe. Now instantiating I with B3
// should not report an error, while instantiating I with A3 should report an error as demonstrated below.
type A3 struct {
	f *int
}

func (A3) foo(x *int) *int { //want "nilable value could be passed as param"
	return x
}

type B3 struct {
	A3
}

// nilable(x)
func (B3) foo(x *int) *int {
	if x == nil {
		z := 0
		return &z
	}
	return x
}

func testOverridding() {
	var i1 I = &B3{}
	i1.foo(nil) // safe, since B3.foo() accepts a nilable parameter

	var i2 I = &A3{}
	i2.foo(nil) // (error reported at A3.foo() definition)
}

// below test checks anonymous fields in structs
type A4 struct{}

func (A4) foo(x *int) *int {
	return x
}

type B4 struct {
	f int
	A4
}

// nilable(x)
func (b *B4) foo(x *int) *int {
	if x == nil {
		return &b.f
	}
	return x
}

func testAnonymousFields(cond bool) *int {
	b := B4{}
	if cond {
		return b.A4.foo(nil) //want "nilable value passed as arg"
	}
	return b.foo(nil) // safe, since B4.foo() accepts a nilable parameter
}

// below test checks embedding of multiple structs
type A5 struct {
	B5
	C5
}

type B5 struct{}

func (B5) foo(x *int) *int {
	return x
}

type C5 struct{}

func (C5) foo(x *int) *int {
	return x
}

func testEmbeddingMultipleStructs() {
	a := &A5{}
	_ = a.B5.foo(nil) //want "nilable value passed as arg"
	_ = a.C5.foo(nil) //want "nilable value passed as arg"
}

// below test checks for recursive embedding of structs
type A6 struct{}

func (A6) foo(x *int) *int { //want "nilable value could be passed as param"
	return x
}

type B6 struct {
	A6
	*B6
}

func testRecursion() *int {
	var i I
	i = B6{}
	return i.foo(nil) // (error reported at A6.foo() definition)
}

// below test checks embedding of multiple interfaces within a struct, and embedding of interfaces within an interface
type J interface {
	bar() *int //want "nilable value could be returned" "nilable value could be returned"
}

type A9 struct {
	I
	J
}

type B9 struct{}

func (B9) foo(x *int) *int { //want "nilable value could be passed as param"
	return x
}

type C9 struct{}

// nilable(result 0)
func (*C9) bar() *int {
	return nil
}

func testMultipleEmbeddedInterfaces() {
	a9 := &A9{I: &B9{}, J: &C9{}}
	_ = a9.foo(nil) // (error reported at B9.foo() definition)
	_ = a9.bar()    // (error reported at J.bar() declaration)
}

type IandJ interface {
	I
	J
}

type A7 struct{}

func (*A7) foo(x *int) *int { //want "nilable value could be passed as param"
	return x
}

// nilable(result 0)
func (*A7) bar() *int {
	return nil
}

func testEmbeddingInterfaceInInterface() {
	var i IandJ = &A7{}
	_ = i.foo(nil) // (error reported at A7.foo() definition)
	_ = i.bar()    // (error reported at J.bar() declaration)
}

// below test checks embedding of interface within a struct
type A8 struct{}

func (*A8) foo(x *int) *int { //want "nilable value could be passed as param"
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

func (*C8) foo(x *int) *int { //want "nilable value could be passed as param"
	return x
}

// D8 declares I as field g, but does not itself implement I.foo()
type D8 struct {
	g I
}

type E8 struct{}

func (*E8) foo(x *int) *int { //want "nilable value could be passed as param"
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
		return i2.foo(nil) // (error reported at A8.foo() definition)
	case 3:
		var i3 I = &C8{}
		return i3.foo(nil) // (error reported at C8.foo() definition)
	case 4:
		d8 := &D8{g: &E8{}}
		return d8.g.foo(nil) // (error reported at E8.foo() definition)
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

func (*D10) foo(x *int) *int { //want "nilable value could be passed as param"
	return x
}

func testNestedStructs() {
	a := &A10{&B10{&C10{D10: D10{}}}}
	_ = a.foo(nil) // (error reported at D10.foo() definition)
}

// below test checks a non-trivial case simulated from https://github.com/golang/go/pull/60823
type Conn interface {
	RemoteAddr() Addr //want "nilable value could be returned"
}

type Addr interface {
	String() string
}

type httpConn struct {
	rwc Conn
}

func (c *httpConn) serve() {
	_ = c.rwc.RemoteAddr().String()
}

type netConn struct{}

// nilable(result 0)
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
