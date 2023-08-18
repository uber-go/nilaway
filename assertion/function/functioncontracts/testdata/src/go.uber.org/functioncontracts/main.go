/*
Test functionality of collecting function contracts from upstream packages.
*/

package functioncontracts

import (
	"go.uber.org/functioncontracts/infer"
	"go.uber.org/functioncontracts/parse"
)

func use() {
	_ = parse.ExportedFromParse(nil)
	_ = infer.ExportedFromInfer(nil)
}
