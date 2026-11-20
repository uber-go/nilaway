//  Copyright (c) 2026 Uber Technologies, Inc.
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

package inference

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway/annotation"
	"go.uber.org/nilaway/util/orderedmap"
)

type recordingConflictHandler struct {
	triggers []annotation.FullTrigger
}

func (h *recordingConflictHandler) AddSingleAssertionConflict(trigger annotation.FullTrigger) {
	h.triggers = append(h.triggers, trigger)
}

func (*recordingConflictHandler) AddOverconstraintConflict(_, _ ExplainedBool) {
	panic("unexpected overconstraint conflict")
}

func newAlwaysFailingTrigger() annotation.FullTrigger {
	return annotation.FullTrigger{
		Producer: &annotation.ProduceTrigger{Annotation: &annotation.ProduceTriggerTautology{}},
		Consumer: &annotation.ConsumeTrigger{Annotation: &annotation.ConsumeTriggerTautology{}},
	}
}

func TestControlledTriggerActivationOrder(t *testing.T) {
	t.Parallel()

	// Unordered map iteration may match insertion order on an individual run.
	for range 32 {
		want := []annotation.FullTrigger{newAlwaysFailingTrigger(), newAlwaysFailingTrigger(), newAlwaysFailingTrigger()}
		controlled := orderedmap.New[annotation.FullTrigger, bool]()
		for _, trigger := range want {
			controlled.Store(trigger, true)
		}
		site := primitiveSite{Repr: "controller"}
		handler := &recordingConflictHandler{}
		engine := &Engine{
			diagnosticEngine:         handler,
			controlledTriggersBySite: map[primitiveSite]*orderedmap.OrderedMap[annotation.FullTrigger, bool]{site: controlled},
		}

		engine.activateControlledTriggers(site, TrueBecauseAnnotation{})
		require.Len(t, handler.triggers, len(want))
		for i, trigger := range want {
			// Compare identity: these triggers have identical contents.
			require.True(t, trigger == handler.triggers[i], "unexpected trigger at index %d", i)
		}
	}
}

func TestControlledTriggerActivationConditions(t *testing.T) {
	t.Parallel()

	trigger := newAlwaysFailingTrigger()
	controlled := orderedmap.New[annotation.FullTrigger, bool]()
	controlled.Store(trigger, true)
	site := primitiveSite{Repr: "controller"}
	handler := &recordingConflictHandler{}
	engine := &Engine{
		diagnosticEngine:         handler,
		controlledTriggersBySite: map[primitiveSite]*orderedmap.OrderedMap[annotation.FullTrigger, bool]{site: controlled},
	}

	engine.activateControlledTriggers(primitiveSite{Repr: "other"}, TrueBecauseAnnotation{})
	require.Empty(t, handler.triggers)
	engine.activateControlledTriggers(site, FalseBecauseAnnotation{})
	require.Empty(t, handler.triggers)
	engine.activateControlledTriggers(site, TrueBecauseAnnotation{})
	require.Equal(t, []annotation.FullTrigger{trigger}, handler.triggers)
}
