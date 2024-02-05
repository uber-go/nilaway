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
