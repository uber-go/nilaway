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
	ProducerRepr annotation.Prestring
	ConsumerRepr annotation.Prestring
	Pos          token.Pos
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
	// PkgRepr is the string representation of the package this site resides in.
	PkgRepr string
	// Repr is the string representation of the site itself.
	Repr string
	// IsDeep is used to differentiate shallow and deep nilabilities of the same sites.
	IsDeep bool
	// Position stores the complete position information (filename, offset, line, column) of the
	// site. It is essential in maintaining the injectivities of the sites since Repr only encodes
	// minimal information for error printing purposes. For example, the first return value of two
	// same-name methods for different structs could end up having the same Repr (e.g.,
	// "Result 0 of function foo"). Note that any random filename prefixes added by the build
	// system (e.g., bazel sandbox prefix) must be trimmed for cross-package reference.
	Position token.Position
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

// fileInfo bundles the token.File object and auxiliary information about it, e.g., whether it is
// a fake file (i.e., imported from archive), for uses in primitivizer.
type fileInfo struct {
	file   *token.File
	isFake bool
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
// Note that this fix (mapping) is two-ways:
// (1) Local incorrect upstream position to correct one for matching upstream sites in the
// inferred map (such that all position information in primitiveSite is always correct). This is
// done via primitivizer.site.
// (2) Correct upstream position to local incorrect one for error reporting purposes (the analysis
// framework only allows reporting via local token.Pos). This is done via primitivizer.sitePos.
//
// [archive importer]: https://github.com/golang/tools/blob/fa12f34b4218307705bf0365ab7df7c119b3653a/internal/gcimporter/bimport.go#L59-L69
type primitivizer struct {
	pass *analysis.Pass
	// upstreamObjPositions maps "<pkg repr>.<object path>" to the correct position.
	upstreamObjPositions map[string]token.Position
	// files maps the file name (modulo the possible build-system prefix) to the token.File object
	// for faster lookup when converting correct upstream position back to local token.Pos for
	// reporting purposes.
	files map[string]fileInfo
	// curDir is the current working directory, which is used to trim the prefix (e.g., bazel
	// random sandbox prefix) from the file names for cross-package references.
	curDir string
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

		importedMap.Range(func(site primitiveSite, _ InferredVal) bool {
			if site.ObjectPath == "" {
				return true
			}

			objRepr := site.PkgRepr + "." + string(site.ObjectPath)
			if existing, ok := upstreamObjPositions[objRepr]; ok && existing != site.Position {
				panic(fmt.Sprintf(
					"conflicting position information on upstream object %q: existing: %v, got: %v",
					objRepr, existing, site.Position))
			}
			upstreamObjPositions[objRepr] = site.Position
			return true
		})
	}

	// Find the current working directory (e.g., random sandbox prefix) for trimming the file names.
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("cannot get current working directory: %v", err))
	}

	// Iterate all files within the Fset (which includes upstream and current-package files), and
	// store the mapping between its file name (modulo the possible build-system prefix) and the
	// token.File object. This is needed for converting correct upstream position back to local
	// incorrect token.Pos for error reporting purposes.
	files := make(map[string]fileInfo)
	pass.Fset.Iterate(func(file *token.File) bool {
		name, err := filepath.Rel(cwd, file.Name())
		if err != nil {
			// For files that are not in the execroot (e.g., stdlib files start with "$GOROOT", and
			// upstream files that do not have the build-system prefix), we can simply use the
			// original file name.
			name = file.Name()
		}

		// The file will be fake (conceptually "\n" * 65535) if it is imported from archive. So we
		// check if there are any gaps between the line starts to determine if the file is fake.
		// TODO: Go 1.21 introduced a new "token.File.Lines()" API to directly get the underlying
		//  lines slice, which should allow faster checks since calling "token.File.LineStart"
		//  repeatedly is slow due to locks/unlocks.
		isFake := true
		var prev token.Pos
		for i := 1; i <= file.LineCount(); i++ {
			p := file.LineStart(i)
			if prev.IsValid() && p-prev > 1 {
				isFake = false
				break
			}
			prev = p
		}
		files[name] = fileInfo{
			file:   file,
			isFake: isFake,
		}
		return true
	})

	return &primitivizer{
		pass:                 pass,
		upstreamObjPositions: upstreamObjPositions,
		files:                files,
		curDir:               cwd,
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
		ProducerRepr: producer,
		ConsumerRepr: consumer,
	}
}

// site returns the primitive version of the annotation site.
func (p *primitivizer) site(key annotation.Key, isDeep bool) primitiveSite {
	objPath, err := objectpath.For(key.Object())
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
		// Generated files contain "//line" directives that point back to the original source file
		// for better error reporting, and PositionFor supports reading that information and adjust
		// the position accordingly. However, those source files are never truly analyzed, meaning
		// the downstream analysis will try to look for the generated files instead of the source
		// files. Therefore, here we use the unadjusted position instead.
		position = p.pass.Fset.PositionFor(key.Object().Pos(), false /* adjusted */)
		if name, err := filepath.Rel(p.curDir, position.Filename); err == nil {
			position.Filename = name
		}
	}

	return primitiveSite{
		PkgRepr:    pkgRepr,
		Repr:       key.String(),
		IsDeep:     isDeep,
		Exported:   key.Object().Exported(),
		ObjectPath: objPath,
		Position:   position,
	}
}

// _fakeFileMaxLines is the maximum number of lines that the archive importer will add to a (fake)
// file when it imports a package. See [the importer code] for more details. We use this to create
// more fake files when necessary (see [primitivizer.sitePos]).
// [the importer code]: https://cs.opensource.google/go/x/tools/+/master:internal/gcimporter/bimport.go;l=34;bpv=0;bpt=1
const _fakeFileMaxLines = 64 * 1024

// sitePos takes the primitive site (with accurate position information) and converts it to a
// token.Pos that is relative to local Fset for reporting purposes _only_.
func (p *primitivizer) sitePos(site primitiveSite) token.Pos {
	info, ok := p.files[site.Position.Filename]
	if !ok {
		// For incremental build systems like bazel, the pass.Fset contains only the files in
		// current and _directly_ imported packages (see [gcexportdata] for more details). However,
		// analyzer facts are imported transitively from all imported packages, and NilAway is able
		// to operate across all those packages. As a result, if NilAway ever needs to report an
		// error on a file from a transitively imported package, we need to create a fake file in
		// the file set.
		// [gcexportdata]: https://pkg.go.dev/golang.org/x/tools/go/gcexportdata
		info = fileInfo{
			// Fake lines will be padded later.
			file:   p.pass.Fset.AddFile(site.Position.Filename, p.pass.Fset.Base(), _fakeFileMaxLines),
			isFake: true,
		}
		p.files[site.Position.Filename] = info
	}

	if info.isFake {
		// If the file is fake (imported from archive), it may not contain fake lines for unexported
		// objects (as an "optimization", see [importer code]). However, NilAway may report errors
		// on unexported objects due to multi-package inference. In such cases, we pad the file with
		// more fake lines.
		// [importer code]: https://cs.opensource.google/go/x/tools/+/refs/tags/v0.12.0:internal/gcimporter/bimport.go;l=36-69;drc=ad74ff6345e3663a8f1a4ba5c6e85d54a6fd5615
		if site.Position.Line > info.file.LineCount() {
			// We are adding offsets to fake lines here, and offset == fake line number - 1. So we
			// can start from the current max line number to the desired line number - 1.
			for i := info.file.LineCount(); i < site.Position.Line; i++ {
				info.file.AddLine(i)
			}
		}

		// For fake files, we can only report accurate line number but not column number.
		return info.file.LineStart(site.Position.Line)
	}

	// For non-fake files, the position is accurate.
	return info.file.Pos(site.Position.Offset)
}
