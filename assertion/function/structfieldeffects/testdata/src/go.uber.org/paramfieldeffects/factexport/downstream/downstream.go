package downstream // want package:".*"

import "go.uber.org/paramfieldeffects/factexport/upstream"

func Forward(o *upstream.Outer) { // expect_reads: param_reads:0:Mid param_reads:0:Mid.Child
	upstream.ExportedRead(o)
}

func ForwardUnused(o *upstream.Outer) {
	upstream.UnusedRead(o)
}

func DirectCall() {
	upstream.ExportedRead(&upstream.Outer{})
}
