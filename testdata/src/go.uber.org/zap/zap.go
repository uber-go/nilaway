// Reproduces a panic where ctrlflow returns a nil *cfg.CFG for a function
// with a non-nil body. ctrlflow's buildDecl short-circuits CFG construction
// for functions in its hard-coded knownIntrinsic list (see
// golang.org/x/tools/go/analysis/passes/ctrlflow), so CFGs.FuncDecl returns
// nil even when decl.Body != nil. The function analyzer must skip these
// rather than dereference the nil graph in preprocess.copyGraph.
//
// (*Logger).Fatal and (*Logger).Panic are entries in that intrinsic list;
// either alone is enough to trigger the panic. The package import path
// must literally be "go.uber.org/zap" for ctrlflow's IsMethodNamed check
// to match.

package zap

type Logger struct{}

func (l *Logger) Fatal(msg string) {}
