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

package annotation

// TriggerKind indicates the kind of the producer / consume trigger, e.g., a trigger will always
// fire, or a trigger will only fire if the underlying site is nilable etc.
type TriggerKind uint8

const (
	// Always indicates a trigger will always fire.
	Always TriggerKind = iota + 1
	// Conditional indicates a trigger will only fire depending on the nilability of the
	// underlying site.
	Conditional
	// DeepConditional indicates a trigger will only fire depending on the deep nilability of the
	// underlying site.
	DeepConditional
	// Never indicates a trigger will never fire.
	Never
)
