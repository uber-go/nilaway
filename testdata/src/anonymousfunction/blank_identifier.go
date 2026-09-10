// <nilaway anonymous function enable>
package anonymousfunction

import (
	"context"
	"encoding/json"
)

type blankCallClient struct{}

func (c *blankCallClient) Call(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	return nil, nil
}

// blankInGoClosure tests that a blank identifier in a multi-return assignment inside a goroutine
// closure is not collected as a closure variable. Before the fix, the blank identifier `_` was
// incorrectly collected because the Go type checker creates a *types.Var named `_` for the blank
// position in `:=` assignments, but never registers it in any scope, so the scope check in
// collectClosure treated it as a captured variable. This caused ParseExprAsProducer to panic when
// the blank identifier was later passed as a synthetic call argument via funcArgsFromCallExpr.
func blankInGoClosure() {
	done := make(chan error, 1)
	c := &blankCallClient{}
	go func() { // expect_closure: c done
		_, err := c.Call(context.Background(), "tool", json.RawMessage(`{}`))
		done <- err
	}()
	<-done
}

// blankInNestedClosure tests that a blank identifier inside a nested closure is also skipped
// and does not propagate to the outer closure variable set.
func blankInNestedClosure() {
	done := make(chan error, 1)
	c := &blankCallClient{}
	go func() { // expect_closure: c done
		func() { // expect_closure: c done
			_, err := c.Call(context.Background(), "tool", json.RawMessage(`{}`))
			done <- err
		}()
	}()
	<-done
}
