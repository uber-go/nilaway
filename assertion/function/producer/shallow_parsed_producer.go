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

package producer

import "go.uber.org/nilaway/annotation"

// ShallowParsedProducer is a ParsedProducer that does not contain information about deeply
// produced values
type ShallowParsedProducer struct {
	Producer *annotation.ProduceTrigger
}

// GetShallow for a ShallowParsedProducer contains the singular ProduceTrigger of this object
func (sp ShallowParsedProducer) GetShallow() *annotation.ProduceTrigger {
	return sp.Producer
}

// GetDeep for a ShallowParsedProducer returns nil
// nilable(result 0)
func (sp ShallowParsedProducer) GetDeep() *annotation.ProduceTrigger { return nil }

// GetFieldProducers returns nil as field producers
func (sp ShallowParsedProducer) GetFieldProducers() []*annotation.ProduceTrigger {
	return nil
}

// IsDeep for a ShallowParsedProducer returns false
func (sp ShallowParsedProducer) IsDeep() bool { return false }

// GetDeepSlice for a ShallowParsedProducer returns an empty slice
// nilable(result 0)
func (sp ShallowParsedProducer) GetDeepSlice() []*annotation.ProduceTrigger { return nil }
