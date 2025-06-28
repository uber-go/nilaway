package templ


import (
	"context"
	"io"
)


func main() {
	ctx := context.Background()
	writer := io.Discard
    Hello().Render(ctx, writer)
}
