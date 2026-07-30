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

import (
	"fmt"
	"go/types"

	"github.com/stretchr/testify/mock"
)

// mockKey is a mock implementation of the Key interface
type mockKey struct {
	mock.Mock
}

func (m *mockKey) Lookup(m2 Map) (Val, bool) {
	args := m.Called(m2)
	return args.Get(0).(Val), args.Bool(1)
}

func (m *mockKey) Object() types.Object {
	args := m.Called()
	return args.Get(0).(types.Object)
}

func (m *mockKey) equals(other Key) bool {
	args := m.Called(other)
	return args.Bool(0)
}

func (m *mockKey) copy() Key {
	args := m.Called()
	return args.Get(0).(Key)
}

func newMockKey() *mockKey {
	mockedKey := new(mockKey)
	mockedKey.ExpectedCalls = nil
	mockedKey.On("equals", mock.Anything).Return(true)

	copiedMockKey := new(mockKey)
	mockedKey.ExpectedCalls = nil
	mockedKey.On("equals", mock.Anything).Return(true)

	mockedKey.On("copy").Return(copiedMockKey)
	return mockedKey
}

// mockProducingAnnotationTrigger is a mock implementation of the ProducingAnnotationTrigger interface
type mockProducingAnnotationTrigger struct {
	mock.Mock
}

func (m *mockProducingAnnotationTrigger) CheckProduce(m2 Map) bool {
	args := m.Called(m2)
	return args.Bool(0)
}

func (m *mockProducingAnnotationTrigger) NeedsGuardMatch() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockProducingAnnotationTrigger) SetNeedsGuard(b bool) {
	m.Called(b)
}

func (m *mockProducingAnnotationTrigger) Repr() fmt.Stringer {
	args := m.Called()
	return args.Get(0).(fmt.Stringer)
}

func (m *mockProducingAnnotationTrigger) Kind() TriggerKind {
	args := m.Called()
	return args.Get(0).(TriggerKind)
}

func (m *mockProducingAnnotationTrigger) UnderlyingSite() Key {
	args := m.Called()
	return args.Get(0).(Key)
}

func (m *mockProducingAnnotationTrigger) equals(other ProducingAnnotationTrigger) bool {
	args := m.Called(other)
	return args.Bool(0)
}
