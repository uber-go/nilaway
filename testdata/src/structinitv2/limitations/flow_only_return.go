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

// Flow-only parameter sources remain unsupported. The collector does not follow values through
// local snapshots, so both callers remain silent until flow-aware collection is added.

type flowLeaf struct {
	Ptr *int
}

type flowDiff struct {
	Value *int
}

func newFlowDiff(l *flowLeaf) *flowDiff {
	value := l.Ptr
	l.Ptr = nil
	return &flowDiff{Value: value}
}

func flowOnlySafeCaller() {
	ptr := 1
	leaf := &flowLeaf{Ptr: &ptr}
	diff := newFlowDiff(leaf)
	print(*diff.Value)
}

func flowOnlyBadCaller() {
	leaf := &flowLeaf{}
	diff := newFlowDiff(leaf)
	print(*diff.Value)
}
