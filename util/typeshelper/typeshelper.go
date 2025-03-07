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

// Package typeshelper implements utility functions for the go/types package.
package typeshelper

import (
	"go/types"
)

// IsIterType returns true if the underlying type is an iterator func:
//
// func(func() bool)
// func(func(K) bool)
// func(func(K, V) bool)
//
// See more at https://tip.golang.org/doc/go1.23.
func IsIterType(t types.Type) bool {
	// Ensure it is a function signature.
	sig, ok := t.Underlying().(*types.Signature)
	if !ok {
		return false
	}

	// Ensure it has exactly one parameter (the yield func).
	params := sig.Params()
	if params.Len() != 1 {
		return false
	}

	// Ensure the single parameter is a function type (the yield func).
	paramType, ok := params.At(0).Type().Underlying().(*types.Signature)
	if !ok {
		return false
	}

	// Ensure the yield func takes fewer than 2 arguments and returns exactly one boolean value.
	res := paramType.Results()
	if paramType.Params().Len() > 2 || res.Len() != 1 {
		return false
	}

	// Final check: ensure the return type of the yield func is a boolean.
	basic, ok := res.At(0).Type().Underlying().(*types.Basic)
	return ok && basic.Kind() == types.Bool
}
