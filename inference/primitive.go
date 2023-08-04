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
	"path"
	"path/filepath"
	"strings"

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
	// ProducerRepr stores the struct that can produce a string representation of the producer.
	ProducerRepr annotation.Prestring
	// ProducerRepr stores the struct that can produce a string representation of the consumer.
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
	// "Result 0 of function foo"). Any random prefixes added by the build system (e.g., bazel
	// sandbox prefix) must be trimmed for cross-package reference.
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

type fileInfo struct {
	file    *token.File
	isLocal bool
}

// primitivizer is able to convert full triggers and annotation sites to their primitive forms. It
// is useful for getting the correct primitive sites and positions for upstream objects due to the
// lack of complete position information in downstream analysis. For example:
//
// upstream/main.go:
// const GlobalVar *int
//
// downstream/main.go:
// func main() { print(*upstream.GlobalVar) }
//
// Here, when analyzing the upstream package we will export a primitive site for `GlobalVar` that
// encodes the package and site representations, and more importantly, the position information to
// uniquely identify the site. However, when analyzing the downstream package, the
// `upstream/main.go` file in the `analysis.Pass.Fset` will not have complete line and column
// information. Instead, the [importer] injects 65535 fake "lines" into the file, and the object
// we get for `upstream.GlobalVar` will contain completely different position information due to
// this hack (in practice, we have observed that the lines for the objects are correct, but not
// others). This leads to mismatches in our inference engine: we cannot find nilabilities of
// upstream objects in the imported InferredMap since their Position fields in the primitive sites
// are different. The primitivizer here contains logic to fix such discrepancies so that the
// returned primitive sites for the same objects always contain correct (and same) Position information.
//
// [importer]: https://cs.opensource.google/go/x/tools/+/refs/tags/v0.7.0:go/internal/gcimporter/bimport.go;l=375-385;drc=c1dd25e80b559a5b0e8e2dd7d5bd1e946aa996a0;bpv=0;bpt=0
type primitivizer struct {
	pass *analysis.Pass
	// upstreamObjPositions maps "pkg repr + object path" to the correct position.
	upstreamObjPositions map[string]token.Position
	files                map[string]fileInfo
	// execRoot is the cached bazel sandbox prefix for trimming the filenames.
	execRoot string
}

// newPrimitivizer returns a new primitivizer.
func newPrimitivizer(pass *analysis.Pass) *primitivizer {
	// To tackle the position discrepancies for upstream sites, we have added an ObjectPath field,
	// which can be used to uniquely identify an exported object relative to the package. Then,
	// we can simply cache the correct position information when importing InferredMaps, since the
	// positions collected in the upstream analysis are always correct. Later when querying upstream
	// objects in downstream analysis, we can look up the cache and fill in the correct position in
	// the returned primitive site instead.

	// Create a cache for upstream object positions.
	upstreamObjPositions := make(map[string]token.Position)
	for _, packageFact := range pass.AllPackageFacts() {
		importedMap, ok := packageFact.Fact.(*InferredMap)
		if !ok {
			continue
		}
		importedMap.Range(func(site primitiveSite, _ InferredVal) bool {
			if site.ObjectPath == "" {
				return true
			}

			objRepr := site.PkgRepr + "." + string(site.ObjectPath)
			if existing, ok := upstreamObjPositions[objRepr]; ok && existing != site.Position {
				/*config.WriteToLog(fmt.Sprintf(
				"conflicting position information on upstream object %q: existing: %v, got: %v",
				objRepr, existing, site.Position))*/
				panic(fmt.Sprintf(
					"conflicting position information on upstream object %q: existing: %v, got: %v",
					objRepr, existing, site.Position))
			}
			upstreamObjPositions[objRepr] = site.Position
			return true
		})
	}

	// Find the bazel execroot (i.e., random sandbox prefix) for trimming the file names.
	execRoot, err := os.Getwd()
	if err != nil {
		panic("cannot get current working directory")
	}
	// config.WriteToLog(fmt.Sprintf("exec root: %q", execRoot))

	// Iterate all files within the Fset (which includes upstream and current package files), and
	// store the mapping between its file name (modulo the bazel prefix) and the token.File object.
	files := make(map[string]fileInfo)
	pass.Fset.Iterate(func(file *token.File) bool {
		name, err := filepath.Rel(execRoot, file.Name())
		if err != nil {
			// For files in standard libraries, there is no bazel sandbox prefix, so we can just
			// keep the original name.
			name = file.Name()
		}
		files[name] = fileInfo{
			file:    file,
			isLocal: strings.HasSuffix(path.Dir(name), pass.Pkg.Path()),
		}
		return true
	})

	return &primitivizer{
		pass:                 pass,
		upstreamObjPositions: upstreamObjPositions,
		files:                files,
		execRoot:             execRoot,
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
	pkgRepr := ""
	if pkg := key.Object().Pkg(); pkg != nil {
		pkgRepr = pkg.Path()
	}

	objPath, err := objectpath.For(key.Object())
	if err != nil {
		// An error will occur when trying to get object path for unexported objects, in which case
		// we simply assign an empty object path.
		objPath = ""
	}

	var position token.Position
	// For upstream objects, we need to look up the local position cache for correct positions.
	if key.Object().Pkg() != nil && p.pass.Pkg != key.Object().Pkg() {
		// Correct upstream information may not always be in the cache: we may not even have it
		// since we skipped analysis for standard and 3rd party libraries.
		if p, ok := p.upstreamObjPositions[pkgRepr+"."+string(objPath)]; ok {
			position = p
		}
	}

	// Default case (local objects or objects from skipped upstream packages), we can simply use
	// their Object.Pos() and retrieve the position information. However, we must trim the bazel
	// sandbox prefix from the filenames for cross-package references.
	if !position.IsValid() {
		position = p.pass.Fset.Position(key.Object().Pos())
		if name, err := filepath.Rel(p.execRoot, position.Filename); err == nil {
			position.Filename = name
		}
	}

	site := primitiveSite{
		PkgRepr:    pkgRepr,
		Repr:       key.String(),
		IsDeep:     isDeep,
		Exported:   key.Object().Exported(),
		ObjectPath: objPath,
		Position:   position,
	}

	/*if objPath != "" {
		// config.WriteToLog(fmt.Sprintf("objpath: %s.%s for site %v", pkgRepr, objPath, site))
	}*/

	return site
}

// sitePos takes the primitive site (with accurate position information) and converts it to a
// token.Pos that is relative to local Fset for reporting purposes.
func (p *primitivizer) sitePos(site primitiveSite) token.Pos {
	// Retrieve the file from cache.
	info, ok := p.files[site.Position.Filename]
	if !ok {
		panic(fmt.Sprintf("file does not exist in downstream analysis: %q", site.Position.Filename))
	}

	// For local files, we can accurate restore the token.Pos.
	if info.isLocal {
		return info.file.Pos(site.Position.Offset)
	}

	// However, files in upstream packages are conceptually 65535 * '\n' (see docs on primitivizer
	// for more details), therefore, we can only restore a local token.Pos that accurately tracks
	// the line, but not the column.
	return info.file.LineStart(site.Position.Line)
}
