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

package asthelper

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDocContains(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		description string
		s           string
		node        *ast.CommentGroup
		want        bool
	}{
		{
			description: "contains the text",
			s:           "42",
			node: &ast.CommentGroup{List: []*ast.Comment{
				{Slash: token.Pos(1), Text: "my comment 42"},
			}},
			want: true,
		},
		{
			description: "contains the text in separate comment inside the group",
			s:           "42",
			node: &ast.CommentGroup{List: []*ast.Comment{
				{Slash: token.Pos(1), Text: "my comment"},
				{Slash: token.Pos(10), Text: "some 42 some other text"},
			}},
			want: true,
		},
		{
			description: "does not contain the text in nil group",
			s:           "42",
			node:        nil,
			want:        false,
		},
		{
			description: "does not contain the text",
			s:           "42",
			node: &ast.CommentGroup{List: []*ast.Comment{
				{Slash: token.Pos(1), Text: "my comment"},
				{Slash: token.Pos(10), Text: "some some other text"},
			}},
			want: false,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, DocContains(tc.node, tc.s))
		})
	}
}
