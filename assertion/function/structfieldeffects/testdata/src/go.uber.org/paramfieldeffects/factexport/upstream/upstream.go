package upstream // want package:".*"

type Leaf struct {
	Ptr *int
}

type Node struct {
	Child *Leaf
}

type Outer struct {
	Mid *Node
}

// Ordinary field-access analysis requires o to be non-nil. These field-read effects additionally
// require Mid and Mid.Child to be non-nil; Ptr itself may be nil because it is not dereferenced.
func ExportedRead(o *Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	_ = o.Mid.Child.Ptr
}

func ReadOneLevelDeep(o *Outer) { // expect_effects:
	_ = o.Mid
}

func UnusedRead(o *Outer) {}

func ExportedNoRead(o *Outer) {
	_ = o
}

func unexportedRead(o *Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	_ = o.Mid.Child.Ptr
}

func ExportedWrite(o *Outer) { // expect_effects: param_writes:0:Mid
	o.Mid = nil
}

func ExportedDeepWrite(o *Outer) { // expect_effects: param_reads:0:Mid param_writes:0:Mid.Child
	o.Mid.Child = nil
}

func ExportedForwardWrite(o *Outer) { // expect_effects: param_reads:0:Mid param_writes:0:Mid.Child
	ExportedDeepWrite(o)
}

func GenericExportedWrite[T any](o *Outer) { // expect_effects: param_writes:0:Mid
	o.Mid = nil
}
