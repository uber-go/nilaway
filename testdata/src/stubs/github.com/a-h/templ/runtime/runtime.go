package runtime

import (
	"context"
	"io"

	"stubs/github.com/a-h/templ"
)

type GeneratedComponentInput struct {
	Writer  io.Writer
	Context context.Context
}

func GeneratedTemplate(fn func(GeneratedComponentInput) error) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return fn(GeneratedComponentInput{Context: ctx, Writer: w})
	})
}

type Buffer struct {
	io.Writer
}

func (b Buffer) WriteString(s string) (int, error) {
	return 0, nil
}

func GetBuffer(w io.Writer) (Buffer, bool) {
	return Buffer{w}, true
}

func ReleaseBuffer(buf Buffer) error {
	return nil
}

func WriteString(w io.Writer, lineNumber int, s string) error {
	return nil
}
