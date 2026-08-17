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

// Package functioncontracts implements a sub-analyzer to analyze function contracts in a package,
// i.e., parsing specified function contracts written as special comments before function
// declarations, or automatically inferring function contracts from the function body.
package functioncontracts

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"runtime/debug"
	"sync"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util/analysishelper"
	"go.uber.org/nilaway/util/typeshelper"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/objectpath"
)

const _doc = "Read the contracts of each function in this package, returning the results."

// Analyzer here is the analyzer than reads function contracts. It returns the map generated from
// reading the function contracts in the source code.
var Analyzer = &analysis.Analyzer{
	Name:       "nilaway_function_contracts_analyzer",
	Doc:        _doc,
	Run:        analysishelper.WrapRun(run),
	ResultType: reflect.TypeOf((*analysishelper.Result[Map])(nil)),
	FactTypes:  []analysis.Fact{new(Contracts), new(ForwardedContracts)},
	Requires:   []*analysis.Analyzer{config.Analyzer, buildssa.Analyzer},
}

// Contracts represents the list of contracts for a function.
type Contracts []Contract

// ForwardedContracts carries upstream method contracts across a package boundary. Object facts
// cannot be exported for objects owned by another package, so methods are identified by paths and
// resolved again from the importing package's type graph.
type ForwardedContracts struct {
	Functions []ForwardedFunction
}

// ForwardedFunction associates an upstream method (by object path) with the contracts that this
// package re-exports on its behalf.
type ForwardedFunction struct {
	ObjectPath objectpath.Path
	Contracts  Contracts
}

// AFact enables use of the facts passing mechanism in Go's analysis framework.
func (*ForwardedContracts) AFact() {}

// AFact enables use of the facts passing mechanism in Go's analysis framework.
func (*Contracts) AFact() {}

// Map stores the mappings from *types.Func to associated function contracts.
type Map map[*types.Func]Contracts

func run(p *analysis.Pass) (Map, error) {
	pass := analysishelper.NewEnhancedPass(p)
	conf := pass.ResultOf[config.Analyzer].(*config.Config)
	if !conf.IsPkgInScope(pass.Pkg) {
		return make(Map), nil
	}

	// Collect contracts from the current package.
	contracts, err := collectFunctionContracts(pass)
	if err != nil {
		return nil, err
	}
	if err := importUpstreamContracts(pass, contracts); err != nil {
		return nil, err
	}
	if conf.ExperimentalStructInitV2Enable {
		snapshot := make(Map, len(contracts))
		for fn, ctrts := range contracts {
			snapshot[fn] = ctrts
		}
		phase2 := make(Map)
		ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
		ssaOfFunc := make(map[*types.Func]*ssa.Function, len(ssaInput.SrcFuncs))
		for _, fnssa := range ssaInput.SrcFuncs {
			if fnssa != nil {
				if fn, ok := fnssa.Object().(*types.Func); ok {
					ssaOfFunc[fn] = fnssa
				}
			}
		}
		resolveContract := func(callee *ssa.Function) Contracts {
			if o, ok := callee.Object().(*types.Func); ok {
				return snapshot[o]
			}
			return nil
		}
		for _, file := range pass.Files {
			if !conf.IsFileInScope(file) {
				continue
			}
			for _, decl := range file.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				funcObj := pass.TypesInfo.ObjectOf(funcDecl.Name).(*types.Func)
				if _, ok := contracts[funcObj]; ok {
					continue
				}
				sig := funcObj.Type().(*types.Signature)
				eligible := sig.Recv() != nil && sig.Results().Len() == 1 && sig.Results().At(0).Type().Underlying().String() == "bool" && !sig.Variadic() && hasNilableReceiverField(sig) && receiverInPackage(sig, pass.Pkg)
				if !eligible {
					continue
				}
				fnssa := ssaOfFunc[funcObj]
				if fnssa == nil || len(fnssa.Blocks) == 0 {
					continue
				}
				if inferred := inferTrueRecvFieldNonNilContracts(fnssa, resolveContract); len(inferred) != 0 {
					phase2[funcObj] = inferred
				}
			}
		}
		for fn, ctrts := range phase2 {
			contracts[fn] = ctrts
		}
	}

	// The fact mechanism only allows exporting pointer types. However, internally we are using
	// `Contract` as a value type because it is an underlying slice type (such that making it a
	// pointer type will make the rest of the logic more complicated). Therefore, we strictly
	// only convert it from/to a pointer type _here_ during the fact import/exports. Everywhere
	// else in NilAway (this sub-analyzer, as well as the other analyzers) we treat `Contract`
	// simply as a value type.

	// Now, export the contracts for the _exported_ functions in the current package only, as
	// well as contracts for upstream methods whose receiver types are part of this package's API.
	reachableTypes := reachableAPITypeNames(pass)
	packageFact := &ForwardedContracts{}
	for fn, ctrts := range contracts {
		// Check if the function is (1) exported by name (i.e., starts with a capital letter), (2)
		// it is directly inside the package scope (such that it is really visible in downstream
		// packages).
		if fn.Exported() &&
			// fn.Scope() -> the scope of the function body.
			((fn.Scope() != nil &&
				// fn.Scope().Parent() -> the scope of the file.
				fn.Scope().Parent() != nil &&
				// fn.Scope().Parent().Parent() -> the scope of the package.
				fn.Scope().Parent().Parent() == pass.Pkg.Scope()) || isExportedMethodOfPackage(fn, pass.Pkg) ||
				isReachableUpstreamMethod(fn, pass.Pkg, reachableTypes)) {
			if fn.Pkg() == pass.Pkg {
				pass.ExportObjectFact(fn, &ctrts)
			} else if isReachableUpstreamMethod(fn, pass.Pkg, reachableTypes) {
				path, pathErr := (&objectpath.Encoder{}).For(fn)
				if pathErr != nil {
					return nil, fmt.Errorf("create object path for forwarded function %s: %w", fn, pathErr)
				}
				packageFact.Functions = append(packageFact.Functions, ForwardedFunction{ObjectPath: path, Contracts: ctrts})
			}
		}
	}
	if len(packageFact.Functions) != 0 {
		pass.ExportPackageFact(packageFact)
	}
	return contracts, nil
}

func importUpstreamContracts(pass *analysishelper.EnhancedPass, contracts Map) error {
	for _, fact := range pass.AllObjectFacts() {
		fn, ok := fact.Object.(*types.Func)
		if !ok {
			continue
		}
		ctrts, ok := fact.Fact.(*Contracts)
		if !ok || ctrts == nil {
			continue
		}
		if _, ok := contracts[fn]; ok {
			return fmt.Errorf("function %s has multiple contracts", fn.Name())
		}
		contracts[fn] = *ctrts
	}
	for _, fact := range pass.AllPackageFacts() {
		forwarded, ok := fact.Fact.(*ForwardedContracts)
		if !ok {
			continue
		}
		methods := reachableAPITypeMethods(reachableAPITypeNames(pass))
		for _, entry := range forwarded.Functions {
			for fn := range methods {
				path, pathErr := (&objectpath.Encoder{}).For(fn)
				if pathErr == nil && path == entry.ObjectPath {
					contracts[fn] = entry.Contracts
				}
			}
		}
	}
	return nil
}

// reachableAPITypeNames returns the named types occurring anywhere in the exported API of pkg.
// Both input and output positions are intentionally traversed: a caller can obtain a method value
// from a value passed to an exported function, not only from a returned value.
func reachableAPITypeNames(pass *analysishelper.EnhancedPass) map[*types.Named]bool {
	reachable := make(map[*types.Named]bool)
	seen := make(map[types.Type]bool)
	var visit func(types.Type)
	visit = func(t types.Type) {
		t = types.Unalias(t)
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch t := t.(type) {
		case *types.Named:
			reachable[t.Origin()] = true
			for i := 0; i < t.TypeArgs().Len(); i++ {
				visit(t.TypeArgs().At(i))
			}
			visit(t.Underlying())
		case *types.Pointer:
			visit(t.Elem())
		case *types.Array:
			visit(t.Elem())
		case *types.Slice:
			visit(t.Elem())
		case *types.Map:
			visit(t.Key())
			visit(t.Elem())
		case *types.Chan:
			visit(t.Elem())
		case *types.Struct:
			for i := 0; i < t.NumFields(); i++ {
				field := t.Field(i)
				if field.Exported() || field.Embedded() {
					visit(field.Type())
				}
			}
		case *types.Signature:
			visitTuple(t.Params(), visit)
			visitTuple(t.Results(), visit)
		case *types.Interface:
			for i := 0; i < t.NumMethods(); i++ {
				visit(t.Method(i).Type())
			}
			for i := 0; i < t.NumEmbeddeds(); i++ {
				visit(t.EmbeddedType(i))
			}
		case *types.Tuple:
			visitTuple(t, visit)
		case *types.TypeParam:
			visit(t.Constraint())
		}
	}
	for _, name := range pass.Pkg.Scope().Names() {
		obj := pass.Pkg.Scope().Lookup(name)
		if obj.Exported() {
			visit(obj.Type())
		}
	}
	return reachable
}

func visitTuple(tuple *types.Tuple, visit func(types.Type)) {
	for i := 0; i < tuple.Len(); i++ {
		visit(tuple.At(i).Type())
	}
}

func isReachableUpstreamMethod(fn *types.Func, pkg *types.Package, reachable map[*types.Named]bool) bool {
	if fn.Pkg() == nil || fn.Pkg() == pkg || !fn.Exported() {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	t := types.Unalias(sig.Recv().Type())
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() != pkg && reachable[named.Origin()]
}

func reachableAPITypeMethods(reachable map[*types.Named]bool) map[*types.Func]bool {
	methods := make(map[*types.Func]bool)
	for named := range reachable {
		for i := 0; i < named.NumMethods(); i++ {
			methods[named.Method(i)] = true
		}
	}
	return methods
}

// functionResult is the struct that is received from the channel for each function.
type functionResult struct {
	funcObj   *types.Func
	contracts Contracts
	err       error
}

// collectFunctionContracts collects all the function contracts and returns a map that associates
// every function with its contracts if it has any. We prefer to parse handwritten contracts from
// the comments at the top of each function. Only when there are no handwritten contracts there,
// do we try to automatically infer contracts.
func collectFunctionContracts(pass *analysishelper.EnhancedPass) (Map, error) {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	// Collect ssa for every function.
	ssaInput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	ssaOfFunc := make(map[*types.Func]*ssa.Function, len(ssaInput.SrcFuncs))
	for _, fnssa := range ssaInput.SrcFuncs {
		if fnssa == nil {
			// should be guaranteed to be non-nil; otherwise it would have paniced in the library
			// https://cs.opensource.google/go/x/tools/+/refs/tags/v0.12.0:go/analysis/passes/buildssa/buildssa.go;l=99
			continue
		}
		if funcObj, ok := fnssa.Object().(*types.Func); ok {
			ssaOfFunc[funcObj] = fnssa
		}
	}

	// Set up variables for synchronization and communication.
	var wg sync.WaitGroup
	funcChan := make(chan functionResult)

	m := Map{}
	for _, file := range pass.Files {
		if !conf.IsFileInScope(file) {
			continue
		}
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok {
				// Ignore any non-function declaration
				// TODO: If we want to support contracts for anonymous functions (function
				//  literals) in the future, then we need to handle more types here.
				continue
			}
			funcObj := pass.TypesInfo.ObjectOf(funcDecl.Name).(*types.Func)

			// First, we try to parse the contracts from the comments at the top of the function.
			// If there are any, we do not need to infer contracts for this function.
			if parsedContracts := parseContracts(funcDecl.Doc); len(parsedContracts) != 0 {
				m[funcObj] = parsedContracts
				continue
			}

			// If we reach here, it means that there are no handwritten contracts for this
			// function. We need to infer contracts for this function.
			sig := funcObj.Type().(*types.Signature)
			oldInferenceEligible := funcDecl.Type.Params.NumFields() == 1 &&
				funcDecl.Type.Results.NumFields() == 1 &&
				!typeshelper.TypeBarsNilness(sig.Params().At(0).Type()) &&
				!typeshelper.TypeBarsNilness(sig.Results().At(0).Type()) &&
				!sig.Variadic()
			argFieldInferenceEligible := typeshelper.FuncIsErrReturning(sig) && !sig.Variadic() && typeshelper.FuncHasPointerToStructParam(sig)
			trueArgInferenceEligible := typeshelper.FuncReturnsExactlyBool(sig) && !sig.Variadic() && sig.Params().Len() > 0 && typeshelper.FuncHasNilableParam(sig)
			falseArgInferenceEligible := trueArgInferenceEligible
			pointerReceiver := false
			if sig.Recv() != nil {
				_, pointerReceiver = sig.Recv().Type().(*types.Pointer)
			}
			getterFieldInferenceEligible := sig.Recv() != nil && pointerReceiver && sig.Params().Len() == 0 && sig.Results().Len() == 1 && !sig.Variadic() && hasNilableReceiverField(sig)
			if !oldInferenceEligible && !argFieldInferenceEligible && !trueArgInferenceEligible && !falseArgInferenceEligible && !getterFieldInferenceEligible {
				// We definitely want to ignore any function without any parameters or return
				// values since they cannot have any contracts.

				// TODO: However, we want to analyze for multiple param/return in the future; for
				//  now we consider contract(nonnil->nonnil) only.

				// TODO: If the function has only one parameter and the parameter is variadic, then
				//  it may happen that no argument is passed when calling the function. Such cases
				//  are not handled well when duplicating full triggers from contracted functions,
				//  so we don't infer contract(nonnil->nonnil) for such a function although we can
				//  already.
				continue
			}
			fnssa, ok := ssaOfFunc[funcObj]
			if !ok {
				// For some reason, we cannot find the ssa for this function. We ignore this
				// function.
				continue
			}
			if len(fnssa.Blocks) == 0 {
				// For external functions whose function bodies are defined outside Go (e.g.,
				// assembly), we do not actually have Go source code for them, and there will be no
				// blocks (see the documentation of the ssa package). Therefore, we ignore such functions.
				continue
			}

			// Infer contracts for a function that does not have any contracts specified.
			wg.Go(func() {
				// As a last resort, convert the panics into errors and return.
				defer func() {
					if r := recover(); r != nil {
						e := fmt.Errorf("%s: %s\n%s", config.InternalPanicPrefix, r, string(debug.Stack()))
						funcChan <- functionResult{err: e, funcObj: funcObj}
					}
				}()

				var contracts Contracts
				if oldInferenceEligible {
					contracts = append(contracts, inferContracts(fnssa)...)
				}
				if conf.ExperimentalStructInitV2Enable && argFieldInferenceEligible {
					contracts = append(contracts, inferArgFieldContracts(fnssa)...)
				}
				if conf.ExperimentalStructInitV2Enable && trueArgInferenceEligible {
					contracts = append(contracts, inferTrueArgNonNilContracts(fnssa, nil)...)
				}
				if conf.ExperimentalStructInitV2Enable && falseArgInferenceEligible {
					contracts = append(contracts, inferFalseArgNonNilContracts(fnssa)...)
				}
				if conf.ExperimentalStructInitV2Enable && getterFieldInferenceEligible {
					contracts = append(contracts, inferGetterFieldContracts(fnssa)...)
				}
				if len(contracts) != 0 {
					funcChan <- functionResult{
						funcObj:   funcObj,
						contracts: contracts,
					}
				}
			})
		}
	}

	// Spawn another goroutine that will close the channel when all analyses are done. This makes
	// sure the channel receive logic in the main thread (below) can properly terminate.
	go func() {
		wg.Wait()
		close(funcChan)
	}()

	// Collect inferred contracts from the channel.
	var err error
	for r := range funcChan {
		m[r.funcObj] = r.contracts
		err = errors.Join(err, r.err)
	}

	return m, err
}

func hasNilableReceiverField(sig *types.Signature) bool {
	if sig.Recv() == nil {
		return false
	}
	t := sig.Recv().Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	st := typeshelper.AsDeeplyStruct(t)
	if st == nil {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Embedded() && !typeshelper.TypeBarsNilness(f.Type()) {
			return true
		}
	}
	return false
}

func receiverInPackage(sig *types.Signature, pkg *types.Package) bool {
	t := sig.Recv().Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() == pkg
}

func isExportedMethodOfPackage(fn *types.Func, pkg *types.Package) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	t := sig.Recv().Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	return ok && named.Obj().Pkg() == pkg
}
