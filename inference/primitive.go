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
	"go/token"

	"go.uber.org/nilaway/annotation"
	"golang.org/x/tools/go/analysis"
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

func fullTriggerAsPrimitive(pass *analysis.Pass, trigger annotation.FullTrigger) primitiveFullTrigger {
	producer, consumer := trigger.Prestrings(pass)
	return primitiveFullTrigger{
		ProducerRepr: producer,
		ConsumerRepr: consumer,
		Pos:          trigger.Consumer.Expr.Pos(),
	}
}

// A primitiveSite represents an atomic choice that may be made about annotations. It is
// more specific than a Key only in factoring out information such as depth (deep annotation
// or not that would make the choice anything other than a boolean).
//
// Triggers created by the assertions analyzer can either be reduced to a primitiveSite,
// or they are a PrimitiveAlways or a PrimitiveNever
//
// Equality on these structs is vital to correctness as they form the keys in the implication graphs
// shared by inference (InferredAnnotationMaps). In particular, if the encoding through
// newPrimitiveSite below is not injective, then learned facts about different annotation sites
// will overwrite each other. Injectivity is currently guaranteed through the combination of `Pos` -
// an integer offset within the file set analyzed, and `PkgStringRepr` - a string representation of
// the package this site was found in.
//
// Further, the mapping from AnnotationKeys to PrimitiveAnnotationSites must be deterministic - or it
// is possible that information about a site will be missed because it is stored under a different
// encoding.
//
// Finally, it is essential that the information contained in these objects is minimal - as they are
// encoded into `Fact`s so frequently that artifact sizes would explode if these got too large.
// This means no extensive string representations, and no deep structs.
type primitiveSite struct {
	StringRepr    string
	IsDeep        bool
	Pos           token.Pos
	PkgStringRepr string
	Exported      bool
}

// newPrimitiveSite encodes a passed Key as a primitiveSite, needing a boolean `isDeep` to complete
// the translation.
// As discussed above for `primitiveSite`, it is vital that this function is injective,
// deterministic, and produces minimized output. Multi Package inference relies on all of these
// properties.
func newPrimitiveSite(key annotation.Key, isDeep bool) primitiveSite {
	pkgStringRepr := ""
	if pkg := key.Object().Pkg(); pkg != nil {
		pkgStringRepr = pkg.Path()
	}
	return primitiveSite{
		StringRepr:    key.String(),
		IsDeep:        isDeep,
		Pos:           key.Object().Pos(),
		PkgStringRepr: pkgStringRepr,
		Exported:      key.Object().Exported(),
	}
}

func (s primitiveSite) String() string {
	deepStr := ""
	if s.IsDeep {
		deepStr = "Deep "
	}
	return deepStr + s.StringRepr
}
