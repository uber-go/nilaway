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

// <nilaway no inference>
package suite

import (
	"go.uber.org/trustedfunc/github.com/stretchr/testify/assert"
	"go.uber.org/trustedfunc/github.com/stretchr/testify/require"
)

type any interface{}

// these stubs simulate the real `github.com/stretchr/testify/suite` package because we can't import it in tests

type Suite struct {
	*assert.Assertions
	require *require.Assertions
}

func (suite *Suite) Require() *require.Assertions {
	return suite.require
}

func (suite *Suite) Assert() *assert.Assertions {
	return suite.Assertions
}
