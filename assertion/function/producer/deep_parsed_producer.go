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

// DeepParsedProducer is a ParsedProducer that contains information about deeply produced values.
// This information will only be read if the produced value turns out to be one of the following cases.
// 1) DeepProducer for map or slice
// 2) FieldProducers (struct or pointer to a struct): It holds the producers for each field of the struct. FieldProducers is either nil or
// it has fixed size equal to the number of fields in the struct. Since, we only add producers for the field that can have
// nil value (pointers, interfaces, slices, etc.), many of field producers will have nil value.
// NOTE: If the array for FieldProducers results in increased memory usage of Nilaway, we can replace it with more compact
// data structure in the future.
type DeepParsedProducer struct {
	ShallowProducer *annotation.ProduceTrigger
	DeepProducer    *annotation.ProduceTrigger
	FieldProducers  []*annotation.ProduceTrigger
}

// GetShallow for a DeepParsedProducer returns the ProduceTrigger producing the value itself
func (dp DeepParsedProducer) GetShallow() *annotation.ProduceTrigger {
	return dp.ShallowProducer
}

// GetDeep for a DeepParsedProducer returns the ProduceTrigger producing indices of the value
func (dp DeepParsedProducer) GetDeep() *annotation.ProduceTrigger {
	return dp.DeepProducer
}

// GetFieldProducers returns field producers
func (dp DeepParsedProducer) GetFieldProducers() []*annotation.ProduceTrigger {
	return dp.FieldProducers
}

// IsDeep for a DeepParsedProducer returns true
func (dp DeepParsedProducer) IsDeep() bool { return true }

// GetDeepSlice for a DeepParsedProducer returns a singular slice containing the deep ProduceTrigger
func (dp DeepParsedProducer) GetDeepSlice() []*annotation.ProduceTrigger {
	return []*annotation.ProduceTrigger{dp.DeepProducer}
}
