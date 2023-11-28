//  Copyright (c) 2023 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package inference

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"

	"go.uber.org/nilaway/annotation"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/objectpath"
)

// A primitiveFullTrigger is a reduced version of an annotation.FullTrigger that can embedded into an
// InferredMap for importation/exportation via the Facts mechanism. This reduction step must
// be performed because FullTriggers themselves contain `types.Object`s, which have no exported fields
// and thus cannot be present in Facts-communicated data structures. PrimitiveFullTriggers encode
// only that information that will be relevant for formatting error messages: a prestring
// representation  of their production, a prestring representation of their consumption, and the
// position in the source.
// See annotation.Prestring for more info, but in short prestrings are structs that store some
// minimal information that will vary between string representations meant to be passed with the
// static type information necessary to format that minimal information into a full string
// representation without needing to encode it all when using Gob encodings through the Facts mechanism
type primitiveFullTrigger struct {
	Position     token.Position
	ProducerRepr annotation.Prestring
	ConsumerRepr annotation.Prestring
}

// A primitiveSite represents an atomic choice that may be made about annotations. It is
// more specific than an annotation.Key only in factoring out information such as depth (deep
// annotation or not that would make the choice anything other than a boolean).
//
// Equality on these structs is vital to correctness as they form the keys in the implication graphs
// shared by inference (InferredMap). In particular, if the encoding through primitiveSite below is
// not injective, then learned facts about different annotation sites will overwrite each other.
//
// Further, the mapping from annotation.Key to primitiveSite must be deterministic - or it
// is possible that information about a site will be missed because it is stored under a different
// encoding.
//
// Finally, it is essential that the information contained in these objects is minimal - as they are
// encoded into `Fact`s so frequently that artifact sizes would explode if these got too large.
// This means no extensive string representations, and no deep structs.
type primitiveSite struct {
	// Position stores the complete position information (filename, offset, line, column) of the
	// site. It is essential in maintaining the injectivities of the sites since Repr only encodes
	// minimal information for error printing purposes. For example, the first return value of two
	// same-name methods for different structs could end up having the same Repr (e.g.,
	// "Result 0 of function foo"). Note that any random filename prefixes added by the build
	// system (e.g., bazel sandbox prefix) must be trimmed for cross-package reference.
	Position token.Position
	// PkgPath is the string representation of the package this site resides in.
	PkgPath string
	// Repr is the string representation of the site itself.
	Repr string
	// IsDeep is used to differentiate shallow and deep nilabilities of the same sites.
	IsDeep bool
	// Exported indicates whether this site is exported in the package or not.
	Exported bool
	// ObjectPath is an opaque name that identifies a types.Object relative to its package (see
	// objectpath.Path for more details). This is essential in order to match upstream objects in
	// downstream analysis. The position information of upstream objects is incomplete due to the
	// way the nogo driver loads packages. As a result, the Position in this struct could differ in
	// downstream analysis, even when referring to the same upstream object. ObjectPath here is
	// useful in correctly matching the upstream objects, and subsequently fixing the Position in
	// the primitivizer. Note that ObjectPath only exists for exported objects; otherwise it will
	// be empty ("").
	ObjectPath objectpath.Path
}

// String returns the string representation of the primitive site for debugging purposes _only_.
func (s *primitiveSite) String() string {
	deepStr := ""
	if s.IsDeep {
		deepStr = "Deep "
	}
	return deepStr + s.Repr
}

// primitivizer is able to convert full triggers and annotation sites to their primitive forms. It
// is useful for getting the correct primitive sites and positions for upstream objects due to the
// lack of complete position information in downstream analysis in incremental build systems (e.g.,
// bazel), where upstream symbol information is loaded from archive. For example:
//
// upstream/main.go:
// const GlobalVar *int
//
// downstream/main.go:
// func main() { print(*upstream.GlobalVar) }
//
// Here, when analyzing the upstream package we will export a primitive site for `GlobalVar` that
// encodes the package and site representations, and more importantly, the position information to
// uniquely identify the site. However, in incremental build systems, when analyzing the downstream
// package, the `upstream/main.go` file in the `analysis.Pass.Fset` will not have complete line and
// column information. Instead, the [archive importer] injects 65535 fake "lines" into the file,
// and the object we get for `upstream.GlobalVar` will contain completely different position
// information due to this hack (in practice, we have observed that the lines for the objects are
// correct, but not others). This leads to mismatches in our inference engine: we cannot find
// nilabilities of upstream sites in the imported InferredMap since the positions of the primitive
// sites are different. The primitivizer here contains logic to fix such discrepancies.
// Specifically, local incorrect upstream position will be fixed to match the upstream sites in the
// inferred map (such that all position information in primitiveSite is always correct).
//
// [archive importer]: https://github.com/golang/tools/blob/fa12f34b4218307705bf0365ab7df7c119b3653a/internal/gcimporter/bimport.go#L59-L69
type primitivizer struct {
	pass *analysis.Pass
	// upstreamObjPositions maps "<pkg path>.<object path>" to the correct position.
	upstreamObjPositions map[string]token.Position
	// curDir is the current working directory, which is used to trim the prefix (e.g.,  random
	// sandbox prefix if using bazel) from the file names for cross-package references.
	curDir string
	// objPathEncoder is used to encode object paths, which amortizes the cost of encoding the
	// paths of multiple objects.
	objPathEncoder *objectpath.Encoder
}

// newPrimitivizer returns a new and properly-initialized primitivizer.
func newPrimitivizer(pass *analysis.Pass) *primitivizer {
	// To tackle the position discrepancies for upstream sites, we have added an ObjectPath field
	// to primitiveSite, which can be used to uniquely identify an exported object relative to the
	// package. Then, we simply cache the correct position information when importing
	// InferredMaps, since the positions collected in the upstream analysis are always correct.
	// Later when querying upstream objects in downstream analysis, we can look up the cache and
	// fill in the correct position in the returned primitive site instead.

	// Create a cache for upstream object positions.
	upstreamObjPositions := make(map[string]token.Position)
	for _, pkgFact := range pass.AllPackageFacts() {
		importedMap, ok := pkgFact.Fact.(*InferredMap)
		if !ok {
			continue
		}

		importedMap.OrderedRange(func(site primitiveSite, _ InferredVal) bool {
			if site.ObjectPath == "" {
				return true
			}

			objRepr := site.PkgPath + "." + string(site.ObjectPath)
			if existing, ok := upstreamObjPositions[objRepr]; ok && existing != site.Position {
				panic(fmt.Sprintf(
					"conflicting position information on upstream object %q: existing: %v, got: %v",
					objRepr, existing, site.Position,
				))
			}
			upstreamObjPositions[objRepr] = site.Position
			return true
		})
	}

	// Find the current working directory (e.g., random sandbox prefix if using bazel) for
	// trimming the file names.
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("cannot get current working directory: %v", err))
	}

	return &primitivizer{
		pass:                 pass,
		upstreamObjPositions: upstreamObjPositions,
		curDir:               cwd,
		objPathEncoder:       &objectpath.Encoder{},
	}
}

// fullTrigger returns the primitive version of the full trigger.
func (p *primitivizer) fullTrigger(trigger annotation.FullTrigger) primitiveFullTrigger {
	// Expr is always nonnil, but our struct init analysis is capped at depth 1 so NilAway does not
	// know this fact. Here, we explicitly guard against such cases to provide a hint.
	if trigger.Consumer.Expr == nil {
		panic(fmt.Sprintf("consume trigger %v has a nil Expr", trigger.Consumer))
	}

	producer, consumer := trigger.Prestrings(p.pass)
	return primitiveFullTrigger{
		Position:     p.toPosition(trigger.Consumer.Expr.Pos()),
		ProducerRepr: producer,
		ConsumerRepr: consumer,
	}
}

// site returns the primitive version of the annotation site.
func (p *primitivizer) site(key annotation.Key, isDeep bool) primitiveSite {
	objPath, err := p.objPathEncoder.For(key.Object())
	if err != nil {
		// An error will occur when trying to get object path for unexported objects, in which case
		// we simply assign an empty object path.
		objPath = ""
	}

	pkgRepr := ""
	if pkg := key.Object().Pkg(); pkg != nil {
		pkgRepr = pkg.Path()
	}

	var position token.Position
	// For upstream objects, we need to look up the local position cache for correct positions.
	if key.Object().Pkg() != p.pass.Pkg {
		// Correct upstream information may not always be in the cache: we may not even have it
		// since we skipped analysis for standard and 3rd party libraries.
		if p, ok := p.upstreamObjPositions[pkgRepr+"."+string(objPath)]; ok {
			position = p
		}
	}

	// Default case (local objects or objects from skipped upstream packages), we can simply use
	// their Object.Pos() and retrieve the position information. However, we must trim the possible
	// build-system sandbox prefix from the filenames for cross-package references.
	if !position.IsValid() {
		position = p.toPosition(key.Object().Pos())
	}

	return primitiveSite{
		PkgPath:    pkgRepr,
		Repr:       key.String(),
		IsDeep:     isDeep,
		Exported:   key.Object().Exported(),
		ObjectPath: objPath,
		Position:   position,
	}
}

// toPosition returns the correct position information for the given pos, removing sandbox prefix
// if any.
func (p *primitivizer) toPosition(pos token.Pos) token.Position {
	// Generated files contain "//line" directives that point back to the original source file
	// for better error reporting, and PositionFor supports reading that information and adjust
	// the position accordingly (i.e., returning a position that points back to the original source
	// file). However, since we are using the precise position information for correctly
	// identifying upstream objects in our cross-package inference, such adjustment will break it
	// the inference (downstream analysis knows nothing about the "original source file").
	// Therefore, here we explicitly disable the adjustment.
	position := p.pass.Fset.PositionFor(pos, false /* adjusted */)

	// For build systems that employ sandboxing (e.g., bazel), the file names in the `Fset` may
	// contain a random prefix. For example:
	//   <SANDBOX_PREFIX>/<WORKSPACE_UUID>/src/mypkg/mysrc1.go
	//   <SANDBOX_PREFIX>/<WORKSPACE_UUID>/src/mypkg/mysrc2.go
	//   src/upstream/mysrc1.go
	//   src/upstream/mysrc2.go
	// Notice that the upstream files do not have this prefix, since this information is loaded
	// from archive file (that stores the symbol information etc.), but not from the sandbox.
	// So, we trim the `<SANDBOX_PREFIX>/<WORKSPACE_UUID>/` here, which is CWD set by bazel build.
	// For other drivers (standard or golangci-lint), we won't even have this prefix prepended,
	// since the file paths will always be in the form of the relative paths (e.g.,
	// `src/mypkg/mysrc1.go`). Trimming the prefixes here for them is simply a no-op.
	if name, err := filepath.Rel(p.curDir, position.Filename); err == nil {
		position.Filename = name
	}

	return position
}
