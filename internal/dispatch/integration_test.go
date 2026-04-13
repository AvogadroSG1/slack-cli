package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// newTestRoot returns a root command with the global persistent flags that
// BuildCommandsWithClient and wiredCommand depend on.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{
		Use:           "slack-cli",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.Bool("pretty", false, "")
	pf.Bool("all", false, "")
	pf.Int("limit", 0, "")
	pf.Int("max-results", 10000, "")
	return root
}

func TestIntegrationExecuteWritesJSON(t *testing.T) {
	t.Cleanup(func() { ClearDispatch() })

	// Register a mock dispatch function that returns known data.
	RegisterDispatch("users.info", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		return map[string]any{
			"ok":   true,
			"user": flags["user"],
		}, nil
	})

	reg := []registry.MethodDef{
		{
			APIMethod:   "users.info",
			Category:    "users",
			Command:     "info",
			Description: "Get user info",
			Params: []registry.ParamDef{
				{Name: "user", Type: "string", Required: true, Description: "User ID"},
			},
		},
	}

	var stdout bytes.Buffer
	root := newTestRoot()
	BuildCommandsWithClient(root, reg, nil, &slack.Client{}, &stdout)

	root.SetArgs([]string{"users", "info", "--user", "U12345"})
	err := root.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext() returned unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal stdout JSON: %v\nraw output: %s", err, stdout.String())
	}

	want := map[string]any{
		"ok":   true,
		"user": "U12345",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("JSON output mismatch (-want +got):\n%s", diff)
	}
}

func TestIntegrationPaginateWritesJSON(t *testing.T) {
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
				"channels":    []any{"dev"},
				"next_cursor": "",
			}, nil
		default:
			t.Fatalf("unexpected call %d", callCount)
			return nil, nil
		}
	})

	reg := []registry.MethodDef{
		{
			APIMethod:   "conversations.list",
			Category:    "conversations",
			Command:     "list",
			Description: "List conversations",
			Paginated:   true,
			CursorParam: "cursor",
			CursorField: "next_cursor",
			ResponseKey: "channels",
		},
	}

	var stdout bytes.Buffer
	root := newTestRoot()
	BuildCommandsWithClient(root, reg, nil, &slack.Client{}, &stdout)

	root.SetArgs([]string{"conversations", "list", "--all"})
	err := root.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext() returned unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to unmarshal stdout JSON: %v\nraw output: %s", err, stdout.String())
	}

	want := map[string]any{
		"results": []any{"general", "random", "dev"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("paginated JSON output mismatch (-want +got):\n%s", diff)
	}

	if callCount != 2 {
		t.Errorf("expected 2 calls to Execute, got %d", callCount)
	}
}

func TestIntegrationNilClientReturnsAuthError(t *testing.T) {
	reg := []registry.MethodDef{
		{
			APIMethod:   "users.info",
			Category:    "users",
			Command:     "info",
			Description: "Get user info",
			Params: []registry.ParamDef{
				{Name: "user", Type: "string", Required: true, Description: "User ID"},
			},
		},
	}

	var stdout bytes.Buffer
	root := newTestRoot()
	// Pass nil client to simulate missing SLACK_TOKEN.
	BuildCommandsWithClient(root, reg, nil, nil, &stdout)

	root.SetArgs([]string{"users", "info", "--user", "U12345"})
	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() expected error for nil client, got nil")
	}

	code := ExitCode(err)
	if code != 2 {
		t.Errorf("ExitCode = %d, want 2 (AuthError)", code)
	}

	// stdout should be empty -- error goes to stderr.
	if stdout.Len() != 0 {
		t.Errorf("expected empty stdout, got %q", stdout.String())
	}
}

func TestIntegrationExtractFlagsAllTypes(t *testing.T) {
	t.Cleanup(func() { ClearDispatch() })

	var captured map[string]any
	RegisterDispatch("test.method", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		captured = flags
		return map[string]any{"ok": true}, nil
	})

	reg := []registry.MethodDef{
		{
			APIMethod: "test.method",
			Category:  "test",
			Command:   "method",
			Params: []registry.ParamDef{
				{Name: "str-flag", Type: "string", Description: "a string"},
				{Name: "int-flag", Type: "int", Description: "an int"},
				{Name: "bool-flag", Type: "bool", Description: "a bool"},
				{Name: "slice-flag", Type: "string-slice", Description: "a slice"},
				{Name: "json-flag", Type: "json", Description: "json value"},
			},
		},
	}

	var stdout bytes.Buffer
	root := newTestRoot()
	BuildCommandsWithClient(root, reg, nil, &slack.Client{}, &stdout)

	root.SetArgs([]string{
		"test", "method",
		"--str-flag", "hello",
		"--int-flag", "42",
		"--bool-flag",
		"--slice-flag", "a,b,c",
		"--json-flag", `{"key":"val"}`,
	})

	err := root.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext() returned unexpected error: %v", err)
	}

	want := map[string]any{
		"str-flag":   "hello",
		"int-flag":   42,
		"bool-flag":  true,
		"slice-flag": []string{"a", "b", "c"},
		"json-flag":  `{"key":"val"}`,
	}
	if diff := cmp.Diff(want, captured); diff != "" {
		t.Errorf("extracted flags mismatch (-want +got):\n%s", diff)
	}
}

func TestIntegrationOverrideBypassesDispatch(t *testing.T) {
	var stdout bytes.Buffer
	overrideCalled := false

	overrideCmd := &cobra.Command{
		Use:   "info",
		Short: "Custom override",
		RunE: func(cmd *cobra.Command, args []string) error {
			overrideCalled = true
			_, err := stdout.WriteString(`{"override":true}`)
			return err
		},
	}

	overrides := map[string]*cobra.Command{
		"users.info": overrideCmd,
	}

	reg := []registry.MethodDef{
		{
			APIMethod:   "users.info",
			Category:    "users",
			Command:     "info",
			Description: "Get user info",
		},
	}

	root := newTestRoot()
	BuildCommandsWithClient(root, reg, overrides, nil, &stdout)

	root.SetArgs([]string{"users", "info"})
	err := root.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext() returned unexpected error: %v", err)
	}

	if !overrideCalled {
		t.Error("expected override command to be called")
	}
}
