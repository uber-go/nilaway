//  Copyright (c) 2026 Uber Technologies, Inc.
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
	"go/types"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const (
	// _modulePattern loads every package in the NilAway module: facts and the types stored inside
	// them may be defined anywhere in the module.
	_modulePattern = "go.uber.org/nilaway/..."
)

// gobCandidate describes a concrete type that may be stored in an interface-typed slot of a
// gob-encoded fact, and hence must be registered with gob.
type gobCandidate struct {
	obj *types.TypeName
	// ptr indicates that the concrete value converted to the interface is a pointer.
	ptr bool
}

// TestGobRegistrationCompleteness statically verifies that every concrete type that may be gob
// encoded as part of an exported fact is registered with gob. Forgetting to register a type does
// _not_ fail in-process drivers (e.g., our own analysistest-based tests or the standalone
// checker, which pass facts in memory), but crashes modular drivers that serialize facts via gob
// (go vet -vettool, bazel/nogo, golangci-lint) at fact-export time with
// "gob: type not registered for interface: ...". This test lets CI catch such gaps early.
//
// Note that the fact types themselves (e.g., *InferredMap) never require manual registration:
// analysis drivers register everything declared in Analyzer.FactTypes automatically. Manual
// registration is only required for the dynamic types stored in interface-typed slots _inside_ a
// fact, which is precisely what this test verifies.
//
// The test works as follows:
//  1. It discovers every fact type in the module (i.e., types with an AFact method) and walks the
//     type graph that gob would encode for each of them (exported fields only; for fact types
//     with a custom GobEncode method, the types actually passed to (*gob.Encoder).Encode inside
//     that method), and collects every interface-typed slot it encounters.
//  2. It builds SSA for the module and, for each such interface, collects the concrete type from
//     every conversion to that interface. These conversions are the complete set of ways concrete
//     values enter the interface; this avoids guessing from type names or including types that
//     implement an interface but are never stored in it.
//  3. It reads the actual registration list (gobRegisteredTypes, the data that GobRegister
//     iterates) and diffs the candidate set against it, failing with actionable messages for any
//     gap.
func TestGobRegistrationCompleteness(t *testing.T) {
	t.Parallel()

	cfg := &packages.Config{
		Mode: packages.LoadSyntax,
	}
	pkgs, err := packages.Load(cfg, _modulePattern)
	require.NoError(t, err)
	require.NotEmpty(t, pkgs)
	for _, pkg := range pkgs {
		require.Empty(t, pkg.Errors, "package %s failed to load", pkg.PkgPath)
	}

	prog, _ := ssautil.Packages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	// Discover all concrete types that may occupy interface slots in gob-encoded facts.
	candidates := collectGobCandidates(t, pkgs, prog)

	// Read the actual registration list and verify it against the discovered candidates.
	registered := collectRegisteredTypes(t)
	require.NotEmpty(t, registered, "no gob registrations found in gobRegisteredTypes")

	want := make([]string, 0, len(candidates))
	for _, cand := range candidates {
		want = append(want, cand.key())
	}
	sort.Strings(want)
	got := make([]string, 0, len(registered))
	for typ := range registered {
		got = append(got, typ)
	}
	sort.Strings(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("gobRegisteredTypes does not match the types that may be encoded inside an "+
			"exported fact (-want +got):\n%s", diff)
	}
}

func (c gobCandidate) key() string {
	name := c.obj.Pkg().Path() + "." + c.obj.Name()
	if c.ptr {
		return "*" + name
	}
	return name
}

// collectFactRoots discovers every fact type in the loaded packages (i.e., named types whose
// pointer method set contains AFact) and returns the type graph roots that gob would encode for
// them.
func collectFactRoots(t *testing.T, pkgs []*packages.Package, prog *ssa.Program) []types.Type {
	t.Helper()

	var roots []types.Type
	for _, pkg := range pkgs {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj, ok := scope.Lookup(name).(*types.TypeName)
			if !ok || obj.IsAlias() {
				continue
			}
			named, ok := types.Unalias(obj.Type()).(*types.Named)
			if !ok {
				continue
			}

			var (
				isFact    bool
				gobEncode *types.Func
			)
			ms := types.NewMethodSet(types.NewPointer(named))
			for i := 0; i < ms.Len(); i++ {
				method, ok := ms.At(i).Obj().(*types.Func)
				if !ok {
					continue
				}
				switch method.Name() {
				case "AFact":
					isFact = true
				case "GobEncode":
					gobEncode = method
				}
			}
			if !isFact {
				continue
			}
			factName := pkg.PkgPath + "." + obj.Name()

			if gobEncode == nil {
				// Standard gob encoding: traverse the fact type itself (the traversal follows
				// exported fields only, matching gob semantics).
				roots = append(roots, named)
				continue
			}

			// The fact defines a custom GobEncode, so gob only sees whatever that method chooses
			// to encode: discover the types of all values it passes to (*gob.Encoder).Encode.
			encodeRoots := gobEncodeRoots(t, prog.FuncValue(gobEncode))
			require.NotEmpty(t, encodeRoots,
				"fact %s has a custom GobEncode method, but no direct (*gob.Encoder).Encode call "+
					"could be found in its body; either encode the data directly in GobEncode or "+
					"extend this test to understand the indirection", factName)
			roots = append(roots, encodeRoots...)
		}
	}
	return roots
}

// gobEncodeRoots discovers the type graph roots of a fact with a custom GobEncode method: the
// static types of all values passed to (*gob.Encoder).Encode within that method's body. This is
// exactly what gob sees at encoding time, without needing hardcoded knowledge of which fields the
// encoder chooses to serialize.
func gobEncodeRoots(t *testing.T, fn *ssa.Function) []types.Type {
	t.Helper()
	require.NotNil(t, fn, "could not resolve GobEncode method to its SSA function")

	var roots []types.Type
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			common := call.Common()
			callee := common.StaticCallee()
			if callee == nil {
				continue
			}
			obj := callee.Object()
			if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != "encoding/gob" ||
				obj.Name() != "Encode" {
				continue
			}
			require.Len(t, common.Args, 2, "unexpected call signature for encoding/gob.Encoder.Encode")
			arg := common.Args[1]
			if makeIface, ok := arg.(*ssa.MakeInterface); ok {
				arg = makeIface.X
			}
			roots = append(roots, arg.Type())
		}
	}
	return roots
}

// collectGobCandidates walks the gob-encoded type graph starting from the given fact roots and
// returns the concrete types that may be stored in interface-typed slots.
func collectGobCandidates(
	t *testing.T, pkgs []*packages.Package, prog *ssa.Program,
) []gobCandidate {
	t.Helper()

	var (
		candidates []gobCandidate
		candSeen   = map[string]bool{}
		visited    = map[string]bool{}
		worklist   = collectFactRoots(t, pkgs, prog)
	)
	conversions := collectInterfaceConversions(prog)
	addCandidate := func(c gobCandidate) {
		if !candSeen[c.key()] {
			candSeen[c.key()] = true
			candidates = append(candidates, c)
			// Concrete implementations are gob-encoded themselves, so their (exported) fields
			// must be traversed as well (e.g., LocatedRepr.Contained is a nested Repr).
			worklist = append(worklist, c.obj.Type())
		}
	}

	for len(worklist) > 0 {
		typ := types.Unalias(worklist[len(worklist)-1])
		worklist = worklist[:len(worklist)-1]
		if visited[typ.String()] {
			continue
		}
		visited[typ.String()] = true

		switch tt := typ.(type) {
		case *types.Pointer:
			worklist = append(worklist, tt.Elem())
		case *types.Slice:
			worklist = append(worklist, tt.Elem())
		case *types.Array:
			worklist = append(worklist, tt.Elem())
		case *types.Map:
			worklist = append(worklist, tt.Key(), tt.Elem())
		case *types.Named:
			if _, ok := tt.Underlying().(*types.Interface); ok {
				for _, c := range interfaceCandidates(t, conversions, tt) {
					addCandidate(c)
				}
				continue
			}
			worklist = append(worklist, tt.Underlying())
		case *types.Struct:
			for i := 0; i < tt.NumFields(); i++ {
				// gob only encodes exported fields.
				if f := tt.Field(i); f.Exported() {
					worklist = append(worklist, f.Type())
				}
			}
		case *types.Basic:
			// Nothing to do for basic types.
		case *types.Interface:
			// Unnamed interfaces (e.g., `any`) cannot be handled by this test.
			require.Fail(t, "unnamed interface type discovered in the gob-encoded type graph",
				"type %s cannot be enumerated; please restructure the fact types or extend this test", typ)
		default:
			require.Fail(t, "unsupported type discovered in the gob-encoded type graph",
				"type %s (%T) is not supported by this test; please extend the traversal logic", typ, typ)
		}
	}
	return candidates
}

// interfaceConversion records an SSA conversion from a concrete value to an interface.
type interfaceConversion struct {
	concrete types.Type
	iface    types.Type
	function string
}

// collectInterfaceConversions returns every concrete-to-interface conversion in the loaded
// module. A concrete dynamic value can enter an interface only through one of these conversions.
func collectInterfaceConversions(prog *ssa.Program) []interfaceConversion {
	var conversions []interfaceConversion
	for fn := range ssautil.AllFunctions(prog) {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				makeIface, ok := instr.(*ssa.MakeInterface)
				if !ok {
					continue
				}
				conversions = append(conversions, interfaceConversion{
					concrete: makeIface.X.Type(),
					iface:    makeIface.Type(),
					function: fn.String(),
				})
			}
		}
	}
	return conversions
}

// interfaceCandidates returns every concrete type converted to the given named interface in the
// loaded module.
func interfaceCandidates(
	t *testing.T, conversions []interfaceConversion, named *types.Named,
) []gobCandidate {
	t.Helper()

	var out []gobCandidate
	seen := map[string]bool{}
	add := func(c gobCandidate) {
		if !seen[c.key()] {
			seen[c.key()] = true
			out = append(out, c)
		}
	}
	ifaceName := named.Obj().Pkg().Path() + "." + named.Obj().Name()
	for _, conversion := range conversions {
		if !types.Identical(types.Unalias(conversion.iface), named) {
			continue
		}

		concrete := types.Unalias(conversion.concrete)
		ptr := false
		if p, ok := concrete.(*types.Pointer); ok {
			ptr = true
			concrete = types.Unalias(p.Elem())
		}
		concreteNamed, ok := concrete.(*types.Named)
		require.Truef(t, ok,
			"concrete value %s converted to %s in %s is not a named type; "+
				"extend gobCandidate to represent this type",
			conversion.concrete, ifaceName, conversion.function)
		if !ok {
			continue
		}
		add(gobCandidate{
			obj: concreteNamed.Obj(),
			ptr: ptr,
		})
	}
	return out
}

// collectRegisteredTypes reads gobRegisteredTypes - the data that GobRegister actually iterates -
// and returns the set of registered types, keyed the same way as gobCandidate.key(). Since
// GobRegister registers exactly this slice, diffing against it is diffing against what actually
// gets registered at runtime.
func collectRegisteredTypes(t *testing.T) map[string]bool {
	t.Helper()

	registered := map[string]bool{}
	baseSeen := map[string]bool{}
	for i, v := range gobRegisteredTypes {
		rt := reflect.TypeOf(v)
		require.NotNil(t, rt, "gobRegisteredTypes[%d] is an untyped nil", i)
		ptr := rt.Kind() == reflect.Pointer
		if ptr {
			rt = rt.Elem()
		}
		require.NotEmpty(t, rt.Name(), "gobRegisteredTypes[%d] (%v) is not a named type", i, rt)
		key := rt.PkgPath() + "." + rt.Name()
		if ptr {
			key = "*" + key
		}
		// gob keys its registry on the base (pointer-free) type, so a slice containing the same
		// type twice - even as T once and *T once - would make GobRegister panic when
		// registering the type under a second sequential name. Catch that here with an
		// actionable message instead.
		base := strings.TrimPrefix(key, "*")
		require.False(t, baseSeen[base],
			"gobRegisteredTypes[%d] (%s) duplicates an earlier entry; gob.RegisterName panics "+
				"when registering the same type under a second name, so GobRegister would panic "+
				"at analyzer init time. Remove the duplicate entry.", i, key)
		baseSeen[base] = true
		registered[key] = true
	}
	return registered
}
