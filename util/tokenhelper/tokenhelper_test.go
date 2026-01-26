//  Copyright (c) 2025 Uber Technologies, Inc.
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

package tokenhelper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRelToCwd(t *testing.T) {
	t.Parallel()

	cwd, err := os.Getwd()
	require.NoError(t, err)

	testcases := []struct {
		give string
		want string
	}{
		{give: filepath.Join(cwd, "testdata", "foo.go"), want: filepath.Join("testdata", "foo.go")},
		{give: filepath.Join("testdata", "foo.go"), want: filepath.Join("testdata", "foo.go")},
	}
	for _, tc := range testcases {
		t.Run(tc.give, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, RelToCwd(tc.give))
		})
	}
}
