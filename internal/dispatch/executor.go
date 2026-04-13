// Package dispatch maps Slack API method names to typed handler functions
// and executes them by name.
package dispatch

import (
	"context"
	"errors"
	"fmt"

	"github.com/slack-go/slack"
)

// DispatchFunc is the signature every Slack API handler must satisfy.
type DispatchFunc func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error)

// ErrUnknownMethod is returned when Execute is called with an API method
// that has no registered handler.
var ErrUnknownMethod = errors.New("unknown API method")

// dispatchMap holds the global set of registered dispatch functions keyed
// by their Slack API method name (e.g. "chat.postMessage").
var dispatchMap = map[string]DispatchFunc{}

// RegisterDispatch adds fn as the handler for apiMethod. It overwrites any
// previously registered handler for the same method.
func RegisterDispatch(apiMethod string, fn DispatchFunc) {
	dispatchMap[apiMethod] = fn
}

// ClearDispatch removes all registered handlers. It exists for use in tests.
func ClearDispatch() {
	dispatchMap = map[string]DispatchFunc{}
}

// Execute looks up the handler registered for apiMethod and calls it. If no
// handler is registered, it returns an error that wraps ErrUnknownMethod.
func Execute(ctx context.Context, client *slack.Client, apiMethod string, flags map[string]any) (any, error) {
	fn, ok := dispatchMap[apiMethod]
	if !ok {
		return nil, fmt.Errorf("%s: %w", apiMethod, ErrUnknownMethod)
	}
	return fn(ctx, client, flags)
}
