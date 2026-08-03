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

package limitations

// Regression guards: two cases around a value-typed map (`map[K]Struct`), each carrying no //want:
//
//  1. valMapReadDirect: indexing a value-typed map yields the zero struct on a miss (never nil), so a
//     deep read through its address does not need a `, ok` guard.
//  2. valMapBuild -> valMapDeepRead: the address of a local stored in a struct field follows the
//     local's real value, so the field is non-nil across the boundary. The map is incidental here —
//     a plain local reproduces the same shape.

type valMapTabConfig struct{ TabID string }
type valMapParams struct{ TabsConfig map[string]valMapTabConfig }

// valMapReadDirect: a value-map miss is the zero struct and `p` is the address of a local, so the
// deep read cannot panic.
func valMapReadDirect(params valMapParams, key string) string {
	tabConfig := params.TabsConfig[key] // value-typed map element: zero value on miss, never nil
	p := &tabConfig
	return p.TabID // no diagnostic
}

// valMapValueUseOK is the negative control: using the element as a value (no address-of, no pointer
// deep read) isolates the address-of flow as the trigger.
func valMapValueUseOK(params valMapParams, key string) string {
	tabConfig := params.TabsConfig[key]
	return tabConfig.TabID // safe — no //want
}

type valMapCardParams struct{ TabConfig *valMapTabConfig }

func valMapDeepRead(p valMapCardParams) string {
	return p.TabConfig.TabID // no diagnostic (p.TabConfig credited non-nil via the caller)
}

// valMapBuild: the element's address is stored in a pointer field of a struct bound to a local, then
// crosses into valMapDeepRead. `&tabConfig` is non-nil, so the field is summarized non-nil.
func valMapBuild(params valMapParams, key string) string {
	tabConfig := params.TabsConfig[key]
	cp := valMapCardParams{TabConfig: &tabConfig}
	return valMapDeepRead(cp)
}
