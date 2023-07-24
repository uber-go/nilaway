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

// Package producer contains definitions for parsed producers, which are the result of ParseExprAsProducer.
package producer

import "go.uber.org/nilaway/annotation"

// ParsedProducer is one of the output objects of ParseExprAsProducer - it represents a production
// of a value, interfaced to abstract away the potential to also include deep production of that
// value, i.e. production of indices of that value
type ParsedProducer interface {
	GetShallow() *annotation.ProduceTrigger
	GetDeep() *annotation.ProduceTrigger
	GetFieldProducers() []*annotation.ProduceTrigger
	IsDeep() bool

	// GetDeepSlice returns a 0 or 1 length slice; sometimes this is a more convenient representation
	GetDeepSlice() []*annotation.ProduceTrigger
}
