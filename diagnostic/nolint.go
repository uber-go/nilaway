package diagnostic

import (
	"go/ast"
	"reflect"
	"slices"
	"strings"

	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/tokenhelper"
	"golang.org/x/tools/go/analysis"
)

// NoLintAnalyzer is an analyzer that reads all NilAway's nolint comments. This is needed since NilAway
// is able to report upstream violations when analyzing downstream packages. Currently, most
// drivers are not able to respect this (they only respect nolint comments on current packages).
// Therefore, inside NilAway we parse the nolint comments, export them as facts, and then do the
// filtering ourselves in [diagnostic.Engine].
var NoLintAnalyzer = &analysis.Analyzer{
	Name:       "nilaway_nolint_analyzer",
	Doc:        "Read NilAway's nolint comments and export them as facts for NilAway's diagnostic engine.",
	Run:        analysishelper.WrapRun(run),
	FactTypes:  []analysis.Fact{new(NoLint)},
	Requires:   []*analysis.Analyzer{},
	ResultType: reflect.TypeOf((*analysishelper.Result[[]Range])(nil)),
}

// NoLint is a fact that stores the ranges of "//nolint:nilaway" comments for cross-package nolint
// suppression support.
type NoLint struct {
	// Ranges lists the ranges of the nolint scopes in the package.
	Ranges []Range
}

// AFact makes NoLint satisfy the analysis.Fact interface such that it can be exported as a fact.
func (*NoLint) AFact() {}

// Range is a minimal struct that stores the filename and the start and end lines of a nolint scopes.
type Range struct {
	Filename string
	From, To int
}

func run(pass *analysis.Pass) ([]Range, error) {
	var ranges []Range
	for _, f := range pass.Files {
		// CommentMap will correctly associate comments to the largest node group
		// applicable. This handles inline comments that might trail a large
		// assignment and will apply the comment to the entire assignment.
		commentMap := ast.NewCommentMap(pass.Fset, f, f.Comments)
		for node, groups := range commentMap {
			for _, group := range groups {
				for _, comm := range group.List {
					if !nolintContainsNilAway(comm.Text) {
						continue
					}
					fromPos, toPos := pass.Fset.Position(node.Pos()), pass.Fset.Position(node.End())
					rng := &Range{Filename: tokenhelper.RelToCwd(fromPos.Filename), From: fromPos.Line, To: toPos.Line}
					ranges = append(ranges, *rng)
				}
			}
		}
	}

	// Import all nolint ranges from upstream.
	var upstreamRanges []Range
	for _, f := range pass.AllPackageFacts() {
		upstreamNoLintRanges, ok := f.Fact.(*NoLint)
		if !ok {
			continue
		}
		upstreamRanges = append(upstreamRanges, upstreamNoLintRanges.Ranges...)
	}

	pass.ExportPackageFact(&NoLint{Ranges: ranges})
	return slices.Concat(ranges, upstreamRanges), nil
}

// https://github.com/bazel-contrib/rules_go/blob/eb13b736d9568044427f23359329155e67071948/go/tools/builders/nolint.go#L21

// nolintContainsNilAway checks if the
func nolintContainsNilAway(text string) bool {
	text = strings.TrimLeft(text, "/ ")
	if !strings.HasPrefix(text, "nolint") {
		return false
	}

	// strip explanation comments
	split := strings.Split(text, "//")
	text = strings.TrimSpace(split[0])

	parts := strings.Split(text, ":")
	if len(parts) == 1 {
		return true
	}
	for _, linter := range strings.Split(strings.TrimSpace(parts[1]), ",") {
		if strings.EqualFold(linter, "all") || strings.EqualFold(linter, "nilaway") {
			return true
		}
	}
	return false
}
