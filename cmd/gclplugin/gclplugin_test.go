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

package gclplugin

import (
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"github.com/stretchr/testify/require"
	"go.uber.org/nilaway"
	"go.uber.org/nilaway/config"
)

func TestPlugin(t *testing.T) {
	t.Parallel()

	plugin, err := New(map[string]any{"pretty-print": "false"})
	require.NoError(t, err)
	require.NotNil(t, plugin)

	require.Equal(t, register.LoadModeTypesInfo, plugin.GetLoadMode())
	analyzers, err := plugin.BuildAnalyzers()
	require.NoError(t, err)
	require.Len(t, analyzers, 1)
	require.Equal(t, nilaway.Analyzer, analyzers[0])

	// The config flag should be set to the value passed in the settings.
	require.Equal(t, "false", config.Analyzer.Flags.Lookup(config.PrettyPrintFlag).Value.String())
}

func TestPlugin_IncorrectSettingsType(t *testing.T) {
	t.Parallel()

	plugin, err := New(map[string]any{"pretty-print": "false", "invalid": []string{"123", "234"}})
	require.Error(t, err)
	require.Nil(t, plugin)
}

func TestPlugin_IncorrectSettings(t *testing.T) {
	t.Parallel()

	plugin, err := New(map[string]any{"invalid": "123"})
	// The settings are applied when we build the analyzers, so the error should be thrown there.
	require.NoError(t, err)
	require.NotNil(t, plugin)

	analyzers, err := plugin.BuildAnalyzers()
	require.ErrorContains(t, err, "invalid")
	require.Empty(t, analyzers)
}
