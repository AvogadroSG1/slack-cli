package override

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/spf13/cobra"
)

// testFixture returns a small, deterministic set of MethodDefs for testing.
func testFixture() []registry.MethodDef {
	return []registry.MethodDef{
		{
			APIMethod:   "conversations.list",
			Category:    "conversations",
			Command:     "list",
			Description: "List all channels",
		},
		{
			APIMethod:   "chat.postMessage",
			Category:    "chat",
			Command:     "post-message",
			Description: "Send a message to a channel",
		},
		{
			APIMethod:   "users.info",
			Category:    "users",
			Command:     "info",
			Description: "Get information about a user",
		},
	}
}

// withFixture sets registry.Registry for the duration of the test and
// restores the original value on cleanup.
func withFixture(t *testing.T) {
	t.Helper()
	orig := registry.Registry
	registry.Registry = testFixture()
	t.Cleanup(func() { registry.Registry = orig })
}

// executeList runs "api list" with the given extra args and returns stdout.
func executeList(t *testing.T, args ...string) string {
	t.Helper()

	root := &cobra.Command{Use: "slack-cli"}
	RegisterBuiltins(root, nil)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"api", "list"}, args...))

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return buf.String()
}

func TestListDefaultOutput(t *testing.T) {
	withFixture(t)
	out := executeList(t)

	// Default output: one method per line, sorted alphabetically.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{
		"chat.postMessage",
		"conversations.list",
		"users.info",
	}
	if diff := cmp.Diff(want, lines); diff != "" {
		t.Errorf("default output mismatch (-want +got):\n%s", diff)
	}
}

func TestListPrettyOutput(t *testing.T) {
	withFixture(t)
	out := executeList(t, "--pretty")

	for _, header := range []string{"COMMAND", "API METHOD", "DESCRIPTION"} {
		if !strings.Contains(out, header) {
			t.Errorf("pretty output missing header %q", header)
		}
	}

	// Verify method data appears in the table.
	if !strings.Contains(out, "chat.postMessage") {
		t.Error("pretty output missing chat.postMessage")
	}
}

func TestListJSONOutput(t *testing.T) {
	withFixture(t)
	out := executeList(t, "--json")

	var items []methodJSON
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("JSON output is not valid JSON: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Verify sorted order.
	want := []string{"chat.postMessage", "conversations.list", "users.info"}
	var got []string
	for _, it := range items {
		got = append(got, it.APIMethod)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("JSON method order mismatch (-want +got):\n%s", diff)
	}
}

func TestListCategoryFilter(t *testing.T) {
	withFixture(t)
	out := executeList(t, "--category", "chat")

	lines := strings.Split(strings.TrimSpace(out), "\n")
	want := []string{"chat.postMessage"}
	if diff := cmp.Diff(want, lines); diff != "" {
		t.Errorf("category filter mismatch (-want +got):\n%s", diff)
	}
}
