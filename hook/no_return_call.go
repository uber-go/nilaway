//  Copyright (c) 2025 Uber Technologies, Inc.
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

package hook

import (
	"go/ast"
	"regexp"
	"slices"

	"golang.org/x/tools/go/analysis"
)

// IsNoReturnCall returns whether the specific call expression terminates the program unconditionally.
// It is used to model certain 3rd party or stdlib functions where the control flow construction
// is not able to infer the function is no-return. For example:
//
// `zap.Fatal`-related: they have complex logic that eventually calls a hook that is almost always
// configured to just panic (but we cannot infer that purely from code).
//
// `testing.TB.Fatal`-related: they are interface methods without implementations.
func IsNoReturnCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	return slices.ContainsFunc(_terminatingCalls, func(sig trustedFuncSig) bool { return sig.match(pass, call) })
}

var _terminatingCalls = []trustedFuncSig{
	// `zap.Logger.Fatal`
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?go\.uber\.org/zap.Logger$`),
		funcNameRegex:  regexp.MustCompile(`^Fatal$`),
	},
	// `zap.SugaredLogger.Fatal` / `zap.SugaredLogger.Fatalf` / `zap.SugaredLogger.Fatalln` / `zap.SugaredLogger.Fatalw`
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^(stubs/)?go\.uber\.org/zap.SugaredLogger$`),
		funcNameRegex:  regexp.MustCompile(`^Fatal(f|ln|w)?$`),
	},
	// `testing.TB.Fatal` / `testing.TB.Fatalf` / `testing.TB.SkipNow` / `testing.TB.Skip` / `testing.TB.Skipf`
	// since it is an interface rather than a concrete implementation, the control flow analyzer
	// will not be able to infer that this is a no-return function. So, here we model it.
	{
		kind:           _method,
		enclosingRegex: regexp.MustCompile(`^testing.TB$`),
		funcNameRegex:  regexp.MustCompile(`^(Fatal|Fatalf|SkipNow|Skip|Skipf)$`),
	},
}
