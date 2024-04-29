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

// Package gclplugin implements the golangci-lint's module plugin interface for NilAway to be used
// as a private linter in golangci-lint. See more details at
// https://golangci-lint.run/plugins/module-plugins/.
package gclplugin

import (
	"fmt"

	"github.com/golangci/plugin-module-register/register"
	"go.uber.org/nilaway"
	"go.uber.org/nilaway/config"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("nilaway", New)
}

// New returns the golangci-lint plugin that wraps the NilAway analyzer.
func New(settings any) (register.LinterPlugin, error) {
	// Parse the settings to the correct type (map[string]string) similar to command line flags.
	s, ok := settings.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expect NilAway's configurations to a map from string to "+
			"string (similar to command line flags), got %T", settings)
	}
	conf := make(map[string]string, len(s))
	for k, v := range s {
		vStr, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("expect NilAway's configuration values for %q to be strings, got %T", k, v)
		}
		conf[k] = vStr
	}

	return &NilAwayPlugin{conf: conf}, nil
}

// NilAwayPlugin is the NilAway plugin wrapper for golangci-lint.
type NilAwayPlugin struct {
	conf map[string]string
}

// BuildAnalyzers builds the NilAway analyzer with the configurations applied to the config analyzer.
func (p *NilAwayPlugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	// Apply the configurations to the config analyzer.
	for k, v := range p.conf {
		if err := config.Analyzer.Flags.Set(k, v); err != nil {
			return nil, fmt.Errorf("set config flag %s with %s: %w", k, v, err)
		}
	}

	return []*analysis.Analyzer{nilaway.Analyzer}, nil
}

// GetLoadMode returns the load mode of the NilAway plugin (requiring types info).
func (p *NilAwayPlugin) GetLoadMode() string { return register.LoadModeTypesInfo }
