package typeshelper

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsIterType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		typeStr string
		want    bool
	}{
		{"ValidIterator0", "func(func() bool)", true},
		{"ValidIterator1", "func(func(int) bool)", true},
		{"ValidIterator2", "func(func(int, string) bool)", true},
		{"InvalidNonFunc", "int", false},
		{"InvalidFuncWrongReturn", "func(func(int) int)", false},
		{"InvalidFuncNoBool", "func(func(int, string))", false},
	}

	fset := token.NewFileSet()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := types.NewPackage("testpkg", "testpkg")
			typeInfo, err := types.Eval(fset, pkg, 0, tt.typeStr)
			if err != nil {
				t.Fatalf("failed to evaluate type: %v", err)
			}

			got := IsIterType(typeInfo.Type)
			require.Equal(t, tt.want, got, "IsIterType(%s) = %v, want %v", tt.typeStr, got, tt.want)
		})
	}
}
