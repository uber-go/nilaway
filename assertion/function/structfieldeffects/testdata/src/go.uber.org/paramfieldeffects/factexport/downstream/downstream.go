package downstream // want package:".*"

import "go.uber.org/paramfieldeffects/factexport/upstream"

func Forward(o *upstream.Outer) { // expect_effects: param_reads:0:Mid param_reads:0:Mid.Child
	upstream.ExportedRead(o)
}

func ForwardUnused(o *upstream.Outer) {
	upstream.UnusedRead(o)
}

func DirectCall() {
	upstream.ExportedRead(&upstream.Outer{})
}

func ForwardWrite(o *upstream.Outer) { // expect_effects: param_writes:0:Mid
	upstream.ExportedWrite(o)
}

func ForwardDeepWrite(o *upstream.Outer) { // expect_effects: param_reads:0:Mid param_writes:0:Mid.Child
	upstream.ExportedDeepWrite(o)
}

func ForwardExportedForwardWrite(o *upstream.Outer) { // expect_effects: param_reads:0:Mid param_writes:0:Mid.Child
	upstream.ExportedForwardWrite(o)
}

func ForwardWriteTwice(o *upstream.Outer) { // expect_effects: param_reads:0:Mid param_writes:0:Mid.Child
	ForwardDeepWrite(o)
}

func ForwardGenericWrite(o *upstream.Outer) { // expect_effects: param_writes:0:Mid
	upstream.GenericExportedWrite[int](o)
}
