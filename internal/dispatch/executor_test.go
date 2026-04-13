package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
)

func TestRegisterAndExecute(t *testing.T) {
	t.Cleanup(func() { ClearDispatch() })

	// Track what the handler receives.
	type call struct {
		APIMethod string
		Flags     map[string]any
	}
	var got call

	RegisterDispatch("chat.postMessage", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		got = call{APIMethod: "chat.postMessage", Flags: flags}
		return "ok", nil
	})

	flags := map[string]any{"channel": "C123", "text": "hello"}
	result, err := Execute(context.Background(), &slack.Client{}, "chat.postMessage", flags)
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}

	if diff := cmp.Diff("ok", result); diff != "" {
		t.Errorf("Execute() result mismatch (-want +got):\n%s", diff)
	}

	want := call{APIMethod: "chat.postMessage", Flags: flags}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("handler call mismatch (-want +got):\n%s", diff)
	}
}

func TestExecuteUnknownMethod(t *testing.T) {
	t.Cleanup(func() { ClearDispatch() })

	_, err := Execute(context.Background(), &slack.Client{}, "bogus.method", nil)
	if err == nil {
		t.Fatal("Execute() expected error for unknown method, got nil")
	}

	if !errors.Is(err, ErrUnknownMethod) {
		t.Errorf("Execute() error = %v, want errors.Is ErrUnknownMethod", err)
	}
}

func TestClearDispatch(t *testing.T) {
	RegisterDispatch("users.list", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		return nil, nil
	})

	ClearDispatch()

	_, err := Execute(context.Background(), &slack.Client{}, "users.list", nil)
	if !errors.Is(err, ErrUnknownMethod) {
		t.Errorf("after ClearDispatch, Execute() error = %v, want ErrUnknownMethod", err)
	}
}
