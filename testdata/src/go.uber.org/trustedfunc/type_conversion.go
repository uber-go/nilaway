//  Copyright (c) 2024 Uber Technologies, Inc.
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

package trustedfunc

type Struct struct {
	Field int
}

type StructPtr *Struct

func typeConversionBasic() {
	var explicit *Struct
	explicit = (*Struct)(nil)
	_ = explicit.Field // want "deref"

	explicitShort := (*Struct)(nil)
	_ = explicitShort.Field // want "deref"

	explicitWithoutParens := StructPtr(nil)
	_ = explicitWithoutParens.Field // want "deref"

	var implicit *Struct
	implicit = nil
	_ = implicit.Field // want "deref"
}

func typeConversionWithNonNil() {
	var s *Struct = &Struct{Field: 42}
	explicit := (*Struct)(s)
	_ = explicit.Field

	var implicit *Struct
	implicit = s
	_ = implicit.Field
}

func typeConversionToNonNilable() {
	var i int = 42
	converted := int(i)
	_ = converted // Should not report nil dereference

	f := float64(i)
	_ = f // Should not report nil dereference
}
