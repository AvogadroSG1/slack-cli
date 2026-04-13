package dispatch

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/slack-go/slack"
)

// testMethod returns a MethodDef suitable for pagination tests.
func testMethod() registry.MethodDef {
	return registry.MethodDef{
		APIMethod:   "conversations.list",
		Paginated:   true,
		CursorParam: "cursor",
		CursorField: "next_cursor",
		ResponseKey: "channels",
	}
}

func TestPaginateSinglePage(t *testing.T) {
	t.Cleanup(func() { ClearDispatch() })

	callCount := 0
	RegisterDispatch("conversations.list", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		callCount++
		return map[string]any{
			"channels": []any{"general", "random"},
		}, nil
	})

	// fetchAll=false should delegate directly to Execute and return the raw
	// response without wrapping.
	flags := map[string]any{"limit": 100}
	got, err := Paginate(context.Background(), &slack.Client{}, testMethod(), flags, false, 0, 1000)
	if err != nil {
		t.Fatalf("Paginate() returned unexpected error: %v", err)
	}

	want := map[string]any{
		"channels": []any{"general", "random"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Paginate(fetchAll=false) mismatch (-want +got):\n%s", diff)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call to Execute, got %d", callCount)
	}
}

func TestPaginateMultiplePages(t *testing.T) {
	t.Cleanup(func() { ClearDispatch() })

	callCount := 0
	RegisterDispatch("conversations.list", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		callCount++
		switch callCount {
		case 1:
			return map[string]any{
				"channels":    []any{"general", "random"},
				"next_cursor": "page2",
			}, nil
		case 2:
			return map[string]any{
				"channels":    []any{"dev", "ops"},
				"next_cursor": "",
			}, nil
		default:
			t.Fatalf("unexpected call %d", callCount)
			return nil, nil
		}
	})

	flags := map[string]any{}
	got, err := Paginate(context.Background(), &slack.Client{}, testMethod(), flags, true, 0, 1000)
	if err != nil {
		t.Fatalf("Paginate() returned unexpected error: %v", err)
	}

	want := map[string]any{
		"results": []any{"general", "random", "dev", "ops"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Paginate(fetchAll=true, 2 pages) mismatch (-want +got):\n%s", diff)
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls to Execute, got %d", callCount)
	}
}

func TestPaginateRespectsLimit(t *testing.T) {
	t.Cleanup(func() { ClearDispatch() })

	callCount := 0
	RegisterDispatch("conversations.list", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		callCount++
		switch callCount {
		case 1:
			return map[string]any{
				"channels":    []any{"general", "random", "dev"},
				"next_cursor": "page2",
			}, nil
		case 2:
			return map[string]any{
				"channels":    []any{"ops", "infra"},
				"next_cursor": "page3",
			}, nil
		default:
			t.Fatalf("unexpected call %d — should have stopped at limit", callCount)
			return nil, nil
		}
	})

	flags := map[string]any{}
	got, err := Paginate(context.Background(), &slack.Client{}, testMethod(), flags, true, 4, 1000)
	if err != nil {
		t.Fatalf("Paginate() returned unexpected error: %v", err)
	}

	// limit=4 so the first page (3 items) is not enough; after the second
	// page we have 5, which gets trimmed to 4.
	want := map[string]any{
		"results": []any{"general", "random", "dev", "ops"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Paginate(limit=4) mismatch (-want +got):\n%s", diff)
	}
}

func TestPaginateRespectsContextCancellation(t *testing.T) {
	t.Cleanup(func() { ClearDispatch() })

	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	RegisterDispatch("conversations.list", func(_ context.Context, _ *slack.Client, _ map[string]any) (any, error) {
		callCount++
		// Cancel the context after the first page so the next iteration
		// of the loop sees it.
		if callCount == 1 {
			cancel()
		}
		return map[string]any{
			"channels":    []any{"general", "random"},
			"next_cursor": "page2",
		}, nil
	})

	flags := map[string]any{}
	got, err := Paginate(ctx, &slack.Client{}, testMethod(), flags, true, 0, 1000)
	if err != nil {
		t.Fatalf("Paginate() returned unexpected error: %v", err)
	}

	result, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", got)
	}

	if partial, _ := result["partial"].(bool); !partial {
		t.Error("expected partial=true in cancelled response")
	}

	if reason, _ := result["reason"].(string); reason != "interrupted" {
		t.Errorf("expected reason=interrupted, got %q", reason)
	}

	if cursor, _ := result["next_cursor"].(string); cursor != "page2" {
		t.Errorf("expected next_cursor=page2, got %q", cursor)
	}

	items, _ := result["results"].([]any)
	if diff := cmp.Diff([]any{"general", "random"}, items); diff != "" {
		t.Errorf("partial results mismatch (-want +got):\n%s", diff)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call before cancellation, got %d", callCount)
	}
}
