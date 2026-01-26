package templ

import (
	"context"
	"io"
)

type Component interface {
	Render(ctx context.Context, w io.Writer) error
}

type ComponentFunc func(ctx context.Context, w io.Writer) error

// Render the template.
func (cf ComponentFunc) Render(ctx context.Context, w io.Writer) error {
	return cf(ctx, w)
}

type Error struct {
	Err      error
	FileName string
	Line     int
	Col      int
}

func (e Error) Error() string { return "templ-error" }

func InitializeContext(ctx context.Context) context.Context {
	return ctx
}

func GetChildren(ctx context.Context) Component {
	return NopComponent
}

func ClearChildren(ctx context.Context) context.Context {
	return ctx
}

var NopComponent Component

func JoinStringErrs(s string) (string, error) {
	return s, nil
}

func EscapeString(s string) string {
	return s
}
