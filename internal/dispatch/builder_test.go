package dispatch

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/spf13/cobra"
)

// testRegistry returns a small method registry spanning two categories.
func testRegistry() []registry.MethodDef {
	return []registry.MethodDef{
		{
			APIMethod:   "chat.postMessage",
			Category:    "chat",
			Command:     "post-message",
			Description: "Send a message to a channel",
			Aliases:     []string{"pm"},
			Params: []registry.ParamDef{
				{Name: "channel", Type: "string", Required: true, Description: "Channel ID"},
				{Name: "text", Type: "string", Required: true, Description: "Message text"},
				{Name: "as-user", Type: "bool", Description: "Post as authed user"},
			},
		},
		{
			APIMethod:   "chat.update",
			Category:    "chat",
			Command:     "update",
			Description: "Update an existing message",
			Params: []registry.ParamDef{
				{Name: "channel", Type: "string", Required: true, Description: "Channel ID"},
				{Name: "ts", Type: "string", Required: true, Description: "Message timestamp"},
				{Name: "text", Type: "string", Description: "New text"},
			},
		},
		{
			APIMethod:   "users.list",
			Category:    "users",
			Command:     "list",
			Description: "List all users",
			Params: []registry.ParamDef{
				{Name: "limit", Type: "int", Description: "Max results"},
			},
		},
		{
			APIMethod:   "users.info",
			Category:    "users",
			Command:     "info",
			Description: "Get user info",
			Params: []registry.ParamDef{
				{Name: "user", Type: "string", Required: true, Description: "User ID"},
				{Name: "tags", Type: "string-slice", Description: "Extra tags"},
			},
		},
	}
}

func TestBuildCommandsCreatesCorrectCategoryGroups(t *testing.T) {
	root := &cobra.Command{Use: "slack"}
	BuildCommands(root, testRegistry(), nil)

	var got []string
	for _, c := range root.Commands() {
		got = append(got, c.Name())
	}

	want := []string{"chat", "users"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("category groups mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildCommandsCategoriesAreSorted(t *testing.T) {
	reg := []registry.MethodDef{
		{APIMethod: "z.a", Category: "zebra", Command: "a"},
		{APIMethod: "a.b", Category: "alpha", Command: "b"},
		{APIMethod: "m.c", Category: "middle", Command: "c"},
	}
	root := &cobra.Command{Use: "slack"}
	BuildCommands(root, reg, nil)

	var got []string
	for _, c := range root.Commands() {
		got = append(got, c.Name())
	}

	want := []string{"alpha", "middle", "zebra"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("sorted categories mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildCommandsChildCommandsHaveCorrectFlags(t *testing.T) {
	root := &cobra.Command{Use: "slack"}
	BuildCommands(root, testRegistry(), nil)

	tests := []struct {
		name     string
		path     []string
		wantUse  string
		flags    []string
		required []string
	}{
		{
			name:     "chat post-message flags",
			path:     []string{"chat", "post-message"},
			wantUse:  "post-message",
			flags:    []string{"channel", "text", "as-user"},
			required: []string{"channel", "text"},
		},
		{
			name:     "chat update flags",
			path:     []string{"chat", "update"},
			wantUse:  "update",
			flags:    []string{"channel", "ts", "text"},
			required: []string{"channel", "ts"},
		},
		{
			name:    "users list flags",
			path:    []string{"users", "list"},
			wantUse: "list",
			flags:   []string{"limit"},
		},
		{
			name:     "users info flags",
			path:     []string{"users", "info"},
			wantUse:  "info",
			flags:    []string{"user", "tags"},
			required: []string{"user"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _, err := root.Find(tt.path)
			if err != nil {
				t.Fatalf("Find(%v): %v", tt.path, err)
			}
			if cmd.Use != tt.wantUse {
				t.Errorf("Use = %q, want %q", cmd.Use, tt.wantUse)
			}

			for _, flag := range tt.flags {
				f := cmd.Flags().Lookup(flag)
				if f == nil {
					t.Errorf("flag %q not found", flag)
				}
			}

			requiredSet := make(map[string]bool)
			for _, r := range tt.required {
				requiredSet[r] = true
			}

			for _, flag := range tt.flags {
				f := cmd.Flags().Lookup(flag)
				if f == nil {
					continue
				}
				annotations := f.Annotations
				_, isRequired := annotations[cobra.BashCompOneRequiredFlag]
				if requiredSet[flag] && !isRequired {
					t.Errorf("flag %q should be required", flag)
				}
				if !requiredSet[flag] && isRequired {
					t.Errorf("flag %q should not be required", flag)
				}
			}
		})
	}
}

func TestBuildCommandsOverrideReplacesGenerated(t *testing.T) {
	overrideCmd := &cobra.Command{
		Use:   "post-message",
		Short: "Custom override",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	overrides := map[string]*cobra.Command{
		"chat.postMessage": overrideCmd,
	}

	root := &cobra.Command{Use: "slack"}
	BuildCommands(root, testRegistry(), overrides)

	cmd, _, err := root.Find([]string{"chat", "post-message"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	if cmd.Short != "Custom override" {
		t.Errorf("Short = %q, want %q", cmd.Short, "Custom override")
	}

	// The override command should not have the generic flags.
	if cmd.Flags().Lookup("channel") != nil {
		t.Error("override command should not have generic 'channel' flag")
	}
}

func TestBuildCommandsAliasesAreSet(t *testing.T) {
	root := &cobra.Command{Use: "slack"}
	BuildCommands(root, testRegistry(), nil)

	cmd, _, err := root.Find([]string{"chat", "pm"})
	if err != nil {
		t.Fatalf("Find via alias: %v", err)
	}

	if cmd.Use != "post-message" {
		t.Errorf("alias should resolve to post-message, got %q", cmd.Use)
	}
}

func TestBuildCommandsGenericRunEReturnsError(t *testing.T) {
	root := &cobra.Command{Use: "slack"}
	BuildCommands(root, testRegistry(), nil)

	cmd, _, err := root.Find([]string{"users", "list"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	runErr := cmd.RunE(cmd, nil)
	if runErr == nil {
		t.Fatal("RunE should return an error for unimplemented dispatch")
	}
	if !strings.Contains(runErr.Error(), "dispatch not yet implemented") {
		t.Errorf("unexpected error message: %v", runErr)
	}
}

func TestBuildCommandsFlagTypes(t *testing.T) {
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

	root := &cobra.Command{Use: "slack"}
	BuildCommands(root, reg, nil)

	cmd, _, err := root.Find([]string{"test", "method"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}

	tests := []struct {
		flag     string
		wantType string
	}{
		{"str-flag", "string"},
		{"int-flag", "int"},
		{"bool-flag", "bool"},
		{"slice-flag", "stringSlice"},
		{"json-flag", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flag)
			if f == nil {
				t.Fatalf("flag %q not found", tt.flag)
			}
			if f.Value.Type() != tt.wantType {
				t.Errorf("flag %q type = %q, want %q", tt.flag, f.Value.Type(), tt.wantType)
			}
		})
	}
}

func TestBuildCommandsEmptyRegistry(t *testing.T) {
	root := &cobra.Command{Use: "slack"}
	BuildCommands(root, nil, nil)

	if len(root.Commands()) != 0 {
		t.Errorf("expected no sub-commands for empty registry, got %d", len(root.Commands()))
	}
}
