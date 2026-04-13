# Slack CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI (`slack-cli`) that wraps the full Slack Web API (excluding admin methods) using `slack-go/slack`, with generated type-safe dispatch, JSON-first output, and nested subcommands.

**Architecture:** A `go generate` tool (`cmd/introspect/`) parses the `slack-go/slack` SDK using `golang.org/x/tools/go/packages` to extract method signatures, then emits a method registry (`internal/registry/generated.go`) and type-safe dispatch functions (`internal/dispatch/generated_dispatch.go`). At runtime, `internal/dispatch/builder.go` reads the registry and dynamically builds a Cobra command tree. Each command delegates to a generated dispatch function that calls the SDK directly -- no runtime reflection.

**Tech Stack:** Go 1.26+, `github.com/slack-go/slack`, `github.com/spf13/cobra`, `golang.org/x/tools/go/packages`, `github.com/google/go-cmp`

**Spec:** `docs/superpowers/specs/2026-04-09-slack-cli-design.md`

**Go Guidelines:** `~/.claude/guidelines/golang.md` -- stdlib `testing` + `go-cmp`, no testify, table-driven tests, `gofmt`, MixedCaps naming, short receiver names.

---

## File Map

| File | Responsibility |
|---|---|
| `go.mod` | Module definition (`github.com/poconnor/slack-cli`) |
| `Makefile` | Build, generate, test, lint, install targets |
| `cmd/slack-cli/main.go` | Entry point: signal handling, auth, root Cobra command, global flags |
| `internal/registry/method.go` | `MethodDef`, `ParamDef` struct definitions |
| `internal/registry/generated.go` | Auto-generated method table (output of `go generate`) |
| `internal/exitcode/exitcode.go` | Exit code constants and error classifier |
| `internal/validate/validate.go` | Input validation (channel IDs, user IDs, timestamps, file paths) |
| `internal/dispatch/builder.go` | Builds Cobra command tree from registry + overrides |
| `internal/dispatch/executor.go` | Thin lookup: maps API method name to dispatch function |
| `internal/dispatch/generated_dispatch.go` | Auto-generated type-safe SDK call functions |
| `internal/dispatch/output.go` | JSON (default) and pretty output formatting |
| `internal/dispatch/pagination.go` | `--all` auto-pagination with streaming |
| `internal/override/override.go` | Override registry + built-in commands (`api list`, `version`) |
| `cmd/introspect/main.go` | Code generator: parses SDK, emits registry + dispatch |
| `cmd/introspect/templates.go` | Go templates for generated code |
| `CLAUDE.md` | AI agent developer instructions |

---

## Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `CLAUDE.md`
- Create: `cmd/slack-cli/main.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/poconnor/ObsidianNotes/code/Slack-API-CLI
go mod init github.com/poconnor/slack-cli
```

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/slack-go/slack@latest
go get github.com/spf13/cobra@latest
go get github.com/google/go-cmp/cmp@latest
go get golang.org/x/tools/go/packages@latest
```

- [ ] **Step 3: Create minimal main.go that compiles**

Create `cmd/slack-cli/main.go`:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := &cobra.Command{
		Use:   "slack-cli",
		Short: "Slack Web API CLI",
		Long:  "CLI for the full Slack Web API. JSON output by default, --pretty for humans.",
	}

	root.PersistentFlags().Bool("pretty", false, "Human-readable output")
	root.PersistentFlags().Bool("all", false, "Auto-paginate all results")
	root.PersistentFlags().Int("limit", 0, "Max results with --all")
	root.PersistentFlags().String("cursor", "", "Pagination cursor")
	root.PersistentFlags().Duration("timeout", 30*time.Second, "Request timeout")
	root.PersistentFlags().Bool("debug", false, "Debug HTTP traffic to stderr (tokens redacted)")
	root.PersistentFlags().Bool("wait-on-rate-limit", false, "Sleep and retry on rate limit")
	root.PersistentFlags().Int("max-results", 10000, "Hard cap on --all pagination results")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("slack-cli %s (commit %s, built %s)\n", version, commit, date)
		},
	})

	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(3)
	}
}
```

- [ ] **Step 4: Create Makefile**

Create `Makefile`:

```makefile
MODULE   := github.com/poconnor/slack-cli
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: generate build test lint install clean

generate:
	go generate ./...

build: generate
	go build $(LDFLAGS) -o bin/slack-cli ./cmd/slack-cli

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

install: build
	go install $(LDFLAGS) ./cmd/slack-cli

clean:
	rm -rf bin/
```

- [ ] **Step 5: Create CLAUDE.md**

Create `CLAUDE.md`:

```markdown
# CLAUDE.md - slack-cli

## What this project is
Go CLI wrapping the Slack Web API. Agent-first (JSON output), human-friendly (--pretty).

## Build and test
make generate  # Rebuild registry from slack-go/slack SDK
make build     # Build binary to bin/slack-cli
make test      # Run all tests with race detector
make lint      # Run golangci-lint

## Architecture
- `cmd/introspect/` - Type-checked introspection of slack-go/slack, emits registry + dispatch
- `internal/registry/` - MethodDef structs, generated.go is the method table
- `internal/dispatch/` - Cobra command builder, generated type-safe dispatch, output formatting
- `internal/override/` - Hand-crafted commands replacing generated ones
- `internal/exitcode/` - Exit code constants and error classifier
- `internal/validate/` - Input validation
- `cmd/slack-cli/` - Entry point with signal handling

## Key patterns
- Generated dispatch: type-safe SDK calls, no runtime reflection
- Methods using `...MsgOption` use generated option builder maps
- Override system replaces generated commands for methods needing custom UX
- Errors go to stderr (JSON), data goes to stdout
- All SDK calls use *Context methods with cancellable context
- Testing: stdlib testing + go-cmp, httptest.Server for mocking, table-driven tests
```

- [ ] **Step 6: Verify it builds and runs**

```bash
go build -o bin/slack-cli ./cmd/slack-cli
./bin/slack-cli version
./bin/slack-cli --help
```

Expected: `slack-cli dev (commit <hash>, built <date>)` and help text listing global flags.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum cmd/slack-cli/main.go Makefile CLAUDE.md
git commit -m "feat: scaffold slack-cli project with Cobra root command and global flags"
```

---

## Task 2: Exit Code and Error Classification

**Files:**
- Create: `internal/exitcode/exitcode.go`
- Create: `internal/exitcode/exitcode_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/exitcode/exitcode_test.go`:

```go
package exitcode_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"

	"github.com/poconnor/slack-cli/internal/exitcode"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error returns OK",
			err:  nil,
			want: exitcode.OK,
		},
		{
			name: "slack auth error returns AuthError",
			err:  slack.SlackErrorResponse{Err: "invalid_auth"},
			want: exitcode.AuthError,
		},
		{
			name: "slack not_authed returns AuthError",
			err:  slack.SlackErrorResponse{Err: "not_authed"},
			want: exitcode.AuthError,
		},
		{
			name: "slack token_revoked returns AuthError",
			err:  slack.SlackErrorResponse{Err: "token_revoked"},
			want: exitcode.AuthError,
		},
		{
			name: "slack channel_not_found returns APIError",
			err:  slack.SlackErrorResponse{Err: "channel_not_found"},
			want: exitcode.APIError,
		},
		{
			name: "rate limited error returns APIError",
			err:  &slack.RateLimitedError{RetryAfter: 30 * time.Second},
			want: exitcode.APIError,
		},
		{
			name: "status code error returns NetError",
			err:  slack.StatusCodeError{Code: 502, Status: "Bad Gateway"},
			want: exitcode.NetError,
		},
		{
			name: "context deadline returns NetError",
			err:  context.DeadlineExceeded,
			want: exitcode.NetError,
		},
		{
			name: "context canceled returns NetError",
			err:  context.Canceled,
			want: exitcode.NetError,
		},
		{
			name: "wrapped slack error is classified",
			err:  fmt.Errorf("calling api: %w", slack.SlackErrorResponse{Err: "channel_not_found"}),
			want: exitcode.APIError,
		},
		{
			name: "generic network error returns NetError",
			err:  &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			want: exitcode.NetError,
		},
		{
			name: "unknown error returns NetError",
			err:  errors.New("something unexpected"),
			want: exitcode.NetError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := exitcode.Classify(tt.err)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Classify() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/exitcode/... -v
```

Expected: compilation error -- package `exitcode` does not exist yet.

- [ ] **Step 3: Implement exit code classifier**

Create `internal/exitcode/exitcode.go`:

```go
package exitcode

import (
	"context"
	"errors"

	"github.com/slack-go/slack"
)

const (
	OK         = 0
	APIError   = 1
	AuthError  = 2
	InputError = 3
	NetError   = 4
)

var authErrors = map[string]bool{
	"invalid_auth":          true,
	"not_authed":            true,
	"token_revoked":         true,
	"token_expired":         true,
	"account_inactive":      true,
	"missing_scope":         true,
	"not_allowed_token_type": true,
}

func Classify(err error) int {
	if err == nil {
		return OK
	}
	var slackErr slack.SlackErrorResponse
	if errors.As(err, &slackErr) {
		if authErrors[slackErr.Err] {
			return AuthError
		}
		return APIError
	}
	var rateLimitErr *slack.RateLimitedError
	if errors.As(err, &rateLimitErr) {
		return APIError
	}
	var statusErr slack.StatusCodeError
	if errors.As(err, &statusErr) {
		return NetError
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return NetError
	}
	return NetError
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/exitcode/... -v
```

Expected: all 12 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/exitcode/
git commit -m "feat: add exit code constants and error classifier"
```

---

## Task 3: Input Validation

**Files:**
- Create: `internal/validate/validate.go`
- Create: `internal/validate/validate_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/validate/validate_test.go`:

```go
package validate_test

import (
	"testing"

	"github.com/poconnor/slack-cli/internal/validate"
)

func TestChannelID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid C prefix", "C01ABC23DEF", false},
		{"valid D prefix", "D01ABC23DEF", false},
		{"valid G prefix", "G01ABC23DEF", false},
		{"too short", "C01", true},
		{"lowercase", "c01abc23def", true},
		{"plain name", "general", true},
		{"hash prefix", "#general", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.ChannelID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ChannelID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestUserID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid U prefix", "U01ABC23DEF", false},
		{"valid W prefix", "W01ABC23DEF", false},
		{"valid B prefix", "B01ABC23DEF", false},
		{"too short", "U01", true},
		{"lowercase", "u01abc23def", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.UserID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("UserID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid timestamp", "1234567890.123456", false},
		{"missing decimal", "1234567890", true},
		{"wrong decimal places", "1234567890.12", true},
		{"letters", "abcdefghij.123456", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Timestamp(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Timestamp(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestTimeout(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
	}{
		{"valid 30s", "30s", false},
		{"valid 1m", "1m", false},
		{"valid 5m", "5m", false},
		{"zero", "0s", true},
		{"negative", "-1s", true},
		{"over 5m", "6m", true},
		{"invalid", "abc", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Timeout(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Timeout(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/validate/... -v
```

Expected: compilation error -- package `validate` does not exist.

- [ ] **Step 3: Implement validators**

Create `internal/validate/validate.go`:

```go
package validate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

var (
	channelIDRe = regexp.MustCompile(`^[CDG][A-Z0-9]{8,}$`)
	userIDRe    = regexp.MustCompile(`^[UWB][A-Z0-9]{8,}$`)
	timestampRe = regexp.MustCompile(`^\d{10}\.\d{6}$`)
	maxTimeout  = 5 * time.Minute
)

func ChannelID(v string) error {
	if !channelIDRe.MatchString(v) {
		return fmt.Errorf("invalid channel ID %q: expected format C/D/G followed by alphanumeric (e.g., C01ABC23DEF)", v)
	}
	return nil
}

func UserID(v string) error {
	if !userIDRe.MatchString(v) {
		return fmt.Errorf("invalid user ID %q: expected format U/W/B followed by alphanumeric (e.g., U01ABC23DEF)", v)
	}
	return nil
}

func Timestamp(v string) error {
	if !timestampRe.MatchString(v) {
		return fmt.Errorf("invalid timestamp %q: expected format 1234567890.123456", v)
	}
	return nil
}

func Timeout(v string) error {
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %v", v, err)
	}
	if d <= 0 {
		return fmt.Errorf("timeout must be positive, got %s", v)
	}
	if d > maxTimeout {
		return fmt.Errorf("timeout must be <= %s, got %s", maxTimeout, v)
	}
	return nil
}

func JSONValue(v string) error {
	if !json.Valid([]byte(v)) {
		return fmt.Errorf("invalid JSON: %s", v)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/validate/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/validate/
git commit -m "feat: add input validation for channel IDs, user IDs, timestamps, and timeouts"
```

---

## Task 4: Method Registry Types

**Files:**
- Create: `internal/registry/method.go`
- Create: `internal/registry/method_test.go`

- [ ] **Step 1: Write the test**

Create `internal/registry/method_test.go`:

```go
package registry_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/poconnor/slack-cli/internal/registry"
)

func TestMethodDefCLIUse(t *testing.T) {
	tests := []struct {
		name string
		def  registry.MethodDef
		want string
	}{
		{
			name: "simple command",
			def:  registry.MethodDef{Command: "list"},
			want: "list",
		},
		{
			name: "hyphenated command",
			def:  registry.MethodDef{Command: "post-message"},
			want: "post-message",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.def.Command
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Command mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGroupByCategory(t *testing.T) {
	defs := []registry.MethodDef{
		{APIMethod: "chat.postMessage", Category: "chat", Command: "post-message"},
		{APIMethod: "chat.update", Category: "chat", Command: "update"},
		{APIMethod: "conversations.list", Category: "conversations", Command: "list"},
	}
	groups := registry.GroupByCategory(defs)
	if diff := cmp.Diff(2, len(groups)); diff != "" {
		t.Errorf("group count mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(2, len(groups["chat"])); diff != "" {
		t.Errorf("chat group count mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(1, len(groups["conversations"])); diff != "" {
		t.Errorf("conversations group count mismatch (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/registry/... -v
```

Expected: compilation error.

- [ ] **Step 3: Implement registry types**

Create `internal/registry/method.go`:

```go
package registry

// ParamDef describes a single CLI flag for a Slack API method.
type ParamDef struct {
	Name        string // CLI flag name (kebab-case)
	SDKName     string // Go struct field or param name
	Type        string // "string", "int", "bool", "string-slice", "json"
	Required    bool
	Description string
	Default     string
}

// MethodDef describes a single Slack API method exposed as a CLI command.
type MethodDef struct {
	APIMethod   string // e.g., "chat.postMessage"
	Category    string // e.g., "chat"
	Command     string // e.g., "post-message"
	SDKMethod   string // e.g., "PostMessageContext" (documentation only)
	Description string
	DocsURL     string
	Aliases     []string
	Params      []ParamDef
	Paginated   bool
	CursorParam string
	CursorField string // JSON field name for next_cursor in response
	CallStyle   string // "positional", "struct", "msgoption"
	ResponseKey string // JSON key for primary data
}

// Registry is the complete method table. Populated by generated.go.
var Registry []MethodDef

// GroupByCategory groups methods by their Category field.
func GroupByCategory(defs []MethodDef) map[string][]MethodDef {
	groups := make(map[string][]MethodDef)
	for _, d := range defs {
		groups[d.Category] = append(groups[d.Category], d)
	}
	return groups
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/registry/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/registry/
git commit -m "feat: add MethodDef and ParamDef types for method registry"
```

---

## Task 5: Output Formatting

**Files:**
- Create: `internal/dispatch/output.go`
- Create: `internal/dispatch/output_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/dispatch/output_test.go`:

```go
package dispatch_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/poconnor/slack-cli/internal/dispatch"
)

func TestFormatJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"channel": "C123", "ts": "1234567890.123456"}
	err := dispatch.FormatOutput(&buf, data, false)
	if err != nil {
		t.Fatalf("FormatOutput() error = %v", err)
	}
	want := "{\n  \"channel\": \"C123\",\n  \"ts\": \"1234567890.123456\"\n}\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("JSON output mismatch (-want +got):\n%s", diff)
	}
}

func TestFormatPretty(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"name": "general", "id": "C123"}
	err := dispatch.FormatOutput(&buf, data, true)
	if err != nil {
		t.Fatalf("FormatOutput() error = %v", err)
	}
	got := buf.String()
	if len(got) == 0 {
		t.Error("FormatOutput(pretty=true) produced empty output")
	}
}

func TestFormatErrorJSON(t *testing.T) {
	var buf bytes.Buffer
	err := dispatch.FormatError(&buf, "channel_not_found", 1)
	if err != nil {
		t.Fatalf("FormatError() error = %v", err)
	}
	want := "{\n  \"ok\": false,\n  \"error\": \"channel_not_found\",\n  \"exit_code\": 1\n}\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("error output mismatch (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/dispatch/... -v
```

Expected: compilation error.

- [ ] **Step 3: Implement output formatting**

Create `internal/dispatch/output.go`:

```go
package dispatch

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

func FormatOutput(w io.Writer, data any, pretty bool) error {
	if pretty {
		return formatPretty(w, data)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func FormatError(w io.Writer, errMsg string, code int) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{
		"ok":        false,
		"error":     errMsg,
		"exit_code": code,
	})
}

func formatPretty(w io.Writer, data any) error {
	switch v := data.(type) {
	case []any:
		return formatSlicePretty(w, v)
	case map[string]any:
		return formatMapPretty(w, v)
	default:
		return FormatOutput(w, data, false)
	}
}

func formatMapPretty(w io.Writer, m map[string]any) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for k, v := range m {
		fmt.Fprintf(tw, "%s\t%v\n", k, v)
	}
	return tw.Flush()
}

func formatSlicePretty(w io.Writer, items []any) error {
	if len(items) == 0 {
		fmt.Fprintln(w, "(no results)")
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/dispatch/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dispatch/output.go internal/dispatch/output_test.go
git commit -m "feat: add JSON and pretty output formatting"
```

---

## Task 6: Command Builder

**Files:**
- Create: `internal/override/override.go`
- Create: `internal/dispatch/builder.go`
- Create: `internal/dispatch/builder_test.go`

- [ ] **Step 1: Create override registry**

Create `internal/override/override.go`:

```go
package override

import "github.com/spf13/cobra"

var Overrides = map[string]*cobra.Command{}

func Register(apiMethod string, cmd *cobra.Command) {
	Overrides[apiMethod] = cmd
}
```

- [ ] **Step 2: Write the failing tests for builder**

Create `internal/dispatch/builder_test.go`:

```go
package dispatch_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/cobra"

	"github.com/poconnor/slack-cli/internal/dispatch"
	"github.com/poconnor/slack-cli/internal/registry"
)

func TestBuildCommands(t *testing.T) {
	reg := []registry.MethodDef{
		{
			APIMethod:   "chat.postMessage",
			Category:    "chat",
			Command:     "post-message",
			Description: "Post a message",
			Params: []registry.ParamDef{
				{Name: "channel", Type: "string", Required: true, Description: "Channel ID"},
				{Name: "text", Type: "string", Required: true, Description: "Message text"},
			},
		},
		{
			APIMethod:   "chat.update",
			Category:    "chat",
			Command:     "update",
			Description: "Update a message",
		},
		{
			APIMethod:   "conversations.list",
			Category:    "conversations",
			Command:     "list",
			Description: "List conversations",
			Paginated:   true,
		},
	}

	root := &cobra.Command{Use: "slack-cli"}
	dispatch.BuildCommands(root, reg, nil)

	chatCmd, _, err := root.Find([]string{"chat"})
	if err != nil {
		t.Fatalf("could not find chat command: %v", err)
	}
	if diff := cmp.Diff("chat", chatCmd.Use); diff != "" {
		t.Errorf("chat command Use mismatch (-want +got):\n%s", diff)
	}

	postCmd, _, err := root.Find([]string{"chat", "post-message"})
	if err != nil {
		t.Fatalf("could not find chat post-message command: %v", err)
	}
	if diff := cmp.Diff("post-message", postCmd.Use); diff != "" {
		t.Errorf("post-message Use mismatch (-want +got):\n%s", diff)
	}

	channelFlag := postCmd.Flags().Lookup("channel")
	if channelFlag == nil {
		t.Fatal("expected --channel flag on post-message")
	}

	convCmd, _, err := root.Find([]string{"conversations"})
	if err != nil {
		t.Fatalf("could not find conversations command: %v", err)
	}
	if diff := cmp.Diff("conversations", convCmd.Use); diff != "" {
		t.Errorf("conversations Use mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildCommandsWithOverride(t *testing.T) {
	reg := []registry.MethodDef{
		{
			APIMethod:   "chat.postMessage",
			Category:    "chat",
			Command:     "post-message",
			Description: "Post a message",
		},
	}
	overrideCmd := &cobra.Command{
		Use:   "post-message",
		Short: "Custom post message",
	}
	overrides := map[string]*cobra.Command{
		"chat.postMessage": overrideCmd,
	}

	root := &cobra.Command{Use: "slack-cli"}
	dispatch.BuildCommands(root, reg, overrides)

	postCmd, _, err := root.Find([]string{"chat", "post-message"})
	if err != nil {
		t.Fatalf("could not find overridden command: %v", err)
	}
	if diff := cmp.Diff("Custom post message", postCmd.Short); diff != "" {
		t.Errorf("override Short mismatch (-want +got):\n%s", diff)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/dispatch/... -v
```

Expected: compilation error -- `dispatch.BuildCommands` not found.

- [ ] **Step 4: Implement builder**

Create `internal/dispatch/builder.go`:

```go
package dispatch

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/poconnor/slack-cli/internal/registry"
)

func BuildCommands(root *cobra.Command, reg []registry.MethodDef, overrides map[string]*cobra.Command) {
	groups := registry.GroupByCategory(reg)
	categories := make([]string, 0, len(groups))
	for cat := range groups {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, category := range categories {
		methods := groups[category]
		groupCmd := &cobra.Command{
			Use:   category,
			Short: fmt.Sprintf("Slack %s API methods", category),
		}
		for _, m := range methods {
			if overrides != nil {
				if override, ok := overrides[m.APIMethod]; ok {
					groupCmd.AddCommand(override)
					continue
				}
			}
			groupCmd.AddCommand(buildMethodCommand(m))
		}
		root.AddCommand(groupCmd)
	}
}

func buildMethodCommand(m registry.MethodDef) *cobra.Command {
	cmd := &cobra.Command{
		Use:     m.Command,
		Short:   m.Description,
		Aliases: m.Aliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("dispatch not configured for %s", m.APIMethod)
		},
	}
	for _, p := range m.Params {
		switch p.Type {
		case "int":
			cmd.Flags().Int(p.Name, 0, p.Description)
		case "bool":
			cmd.Flags().Bool(p.Name, false, p.Description)
		case "string-slice":
			cmd.Flags().StringSlice(p.Name, nil, p.Description)
		default:
			cmd.Flags().String(p.Name, p.Default, p.Description)
		}
		if p.Required {
			cmd.MarkFlagRequired(p.Name)
		}
	}
	return cmd
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/dispatch/... -v
```

Expected: all tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/dispatch/builder.go internal/dispatch/builder_test.go internal/override/override.go
git commit -m "feat: add command builder that creates Cobra tree from method registry"
```

---

## Task 7: Executor (Dispatch Lookup)

**Files:**
- Create: `internal/dispatch/executor.go`
- Create: `internal/dispatch/executor_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/dispatch/executor_test.go`:

```go
package dispatch_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"

	"github.com/poconnor/slack-cli/internal/dispatch"
)

func TestExecute(t *testing.T) {
	called := false
	dispatch.RegisterDispatch("test.method", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		called = true
		return map[string]string{"ok": "true"}, nil
	})
	defer dispatch.ClearDispatch()

	result, err := dispatch.Execute(context.Background(), nil, "test.method", nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !called {
		t.Error("dispatch function was not called")
	}
	m, ok := result.(map[string]string)
	if !ok {
		t.Fatalf("result type = %T, want map[string]string", result)
	}
	if diff := cmp.Diff("true", m["ok"]); diff != "" {
		t.Errorf("result mismatch (-want +got):\n%s", diff)
	}
}

func TestExecuteUnknownMethod(t *testing.T) {
	dispatch.ClearDispatch()
	_, err := dispatch.Execute(context.Background(), nil, "nonexistent.method", nil)
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
	if !errors.Is(err, dispatch.ErrUnknownMethod) {
		t.Errorf("error = %v, want ErrUnknownMethod", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/dispatch/... -v
```

Expected: compilation error.

- [ ] **Step 3: Implement executor**

Create `internal/dispatch/executor.go`:

```go
package dispatch

import (
	"context"
	"errors"
	"fmt"

	"github.com/slack-go/slack"
)

var ErrUnknownMethod = errors.New("unknown API method")

type DispatchFunc func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error)

var dispatchMap = map[string]DispatchFunc{}

func RegisterDispatch(apiMethod string, fn DispatchFunc) {
	dispatchMap[apiMethod] = fn
}

func ClearDispatch() {
	dispatchMap = map[string]DispatchFunc{}
}

func Execute(ctx context.Context, client *slack.Client, apiMethod string, flags map[string]any) (any, error) {
	fn, ok := dispatchMap[apiMethod]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownMethod, apiMethod)
	}
	return fn(ctx, client, flags)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/dispatch/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dispatch/executor.go internal/dispatch/executor_test.go
git commit -m "feat: add dispatch executor with type-safe function lookup"
```

---

## Task 8: Pagination

**Files:**
- Create: `internal/dispatch/pagination.go`
- Create: `internal/dispatch/pagination_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/dispatch/pagination_test.go`:

```go
package dispatch_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"

	"github.com/poconnor/slack-cli/internal/dispatch"
	"github.com/poconnor/slack-cli/internal/registry"
)

func TestPaginateSinglePage(t *testing.T) {
	dispatch.ClearDispatch()
	dispatch.RegisterDispatch("test.list", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		return map[string]any{
			"items":       []any{"a", "b", "c"},
			"next_cursor": "",
		}, nil
	})

	method := registry.MethodDef{
		APIMethod:   "test.list",
		Paginated:   true,
		CursorParam: "cursor",
		CursorField: "next_cursor",
		ResponseKey: "items",
	}

	result, err := dispatch.Paginate(context.Background(), nil, method, map[string]any{}, false, 0, 10000)
	if err != nil {
		t.Fatalf("Paginate() error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map[string]any", result)
	}
	items := m["items"].([]any)
	if diff := cmp.Diff(3, len(items)); diff != "" {
		t.Errorf("items count mismatch (-want +got):\n%s", diff)
	}
}

func TestPaginateMultiplePages(t *testing.T) {
	dispatch.ClearDispatch()
	callCount := 0
	dispatch.RegisterDispatch("test.list", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		callCount++
		cursor := ""
		if callCount == 1 {
			cursor = "page2"
		}
		return map[string]any{
			"items":       []any{callCount},
			"next_cursor": cursor,
		}, nil
	})

	method := registry.MethodDef{
		APIMethod:   "test.list",
		Paginated:   true,
		CursorParam: "cursor",
		CursorField: "next_cursor",
		ResponseKey: "items",
	}

	result, err := dispatch.Paginate(context.Background(), nil, method, map[string]any{}, true, 0, 10000)
	if err != nil {
		t.Fatalf("Paginate() error = %v", err)
	}
	items, ok := result.([]any)
	if !ok {
		t.Fatalf("result type = %T, want []any", result)
	}
	if diff := cmp.Diff(2, len(items)); diff != "" {
		t.Errorf("items count mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(2, callCount); diff != "" {
		t.Errorf("call count mismatch (-want +got):\n%s", diff)
	}
}

func TestPaginateRespectsLimit(t *testing.T) {
	dispatch.ClearDispatch()
	callCount := 0
	dispatch.RegisterDispatch("test.list", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		callCount++
		return map[string]any{
			"items":       []any{"item"},
			"next_cursor": "more",
		}, nil
	})

	method := registry.MethodDef{
		APIMethod:   "test.list",
		Paginated:   true,
		CursorParam: "cursor",
		CursorField: "next_cursor",
		ResponseKey: "items",
	}

	result, err := dispatch.Paginate(context.Background(), nil, method, map[string]any{}, true, 2, 10000)
	if err != nil {
		t.Fatalf("Paginate() error = %v", err)
	}
	items := result.([]any)
	if diff := cmp.Diff(2, len(items)); diff != "" {
		t.Errorf("items count mismatch (-want +got):\n%s", diff)
	}
}

func TestPaginateRespectsContextCancel(t *testing.T) {
	dispatch.ClearDispatch()
	callCount := 0
	ctx, cancel := context.WithCancel(context.Background())
	dispatch.RegisterDispatch("test.list", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		callCount++
		if callCount == 2 {
			cancel()
		}
		return map[string]any{
			"items":       []any{"item"},
			"next_cursor": "more",
		}, nil
	})

	method := registry.MethodDef{
		APIMethod:   "test.list",
		Paginated:   true,
		CursorParam: "cursor",
		CursorField: "next_cursor",
		ResponseKey: "items",
	}

	result, err := dispatch.Paginate(ctx, nil, method, map[string]any{}, true, 0, 10000)
	if err != nil {
		t.Fatalf("Paginate() error = %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected partial response map, got %T", result)
	}
	if m["partial"] != true {
		t.Error("expected partial=true in response")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/dispatch/... -v -run TestPaginate
```

Expected: compilation error.

- [ ] **Step 3: Implement pagination**

Create `internal/dispatch/pagination.go`:

```go
package dispatch

import (
	"context"

	"github.com/slack-go/slack"

	"github.com/poconnor/slack-cli/internal/registry"
)

func Paginate(
	ctx context.Context,
	client *slack.Client,
	method registry.MethodDef,
	flags map[string]any,
	fetchAll bool,
	limit int,
	maxResults int,
) (any, error) {
	if !fetchAll {
		return Execute(ctx, client, method.APIMethod, flags)
	}

	effectiveLimit := maxResults
	if limit > 0 && limit < maxResults {
		effectiveLimit = limit
	}

	var all []any
	cursor := ""

	for {
		select {
		case <-ctx.Done():
			return partialResponse(all, cursor, "interrupted"), nil
		default:
		}

		flags[method.CursorParam] = cursor
		result, err := Execute(ctx, client, method.APIMethod, flags)
		if err != nil {
			return nil, err
		}

		m, ok := result.(map[string]any)
		if !ok {
			return result, nil
		}

		if items, ok := m[method.ResponseKey].([]any); ok {
			all = append(all, items...)
		}

		if next, ok := m[method.CursorField].(string); ok && next != "" {
			cursor = next
		} else {
			break
		}

		if len(all) >= effectiveLimit {
			all = all[:effectiveLimit]
			break
		}
	}

	return all, nil
}

func partialResponse(items []any, cursor string, reason string) map[string]any {
	return map[string]any{
		"results":     items,
		"partial":     true,
		"next_cursor": cursor,
		"reason":      reason,
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/dispatch/... -v -run TestPaginate
```

Expected: all 4 pagination tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dispatch/pagination.go internal/dispatch/pagination_test.go
git commit -m "feat: add pagination with --all support, limits, and context cancellation"
```

---

## Task 9: Wire Builder to Executor and Main

**Files:**
- Modify: `internal/dispatch/builder.go`
- Modify: `cmd/slack-cli/main.go`
- Create: `internal/dispatch/integration_test.go`

- [ ] **Step 1: Write the integration test**

Create `internal/dispatch/integration_test.go`:

```go
package dispatch_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"

	"github.com/poconnor/slack-cli/internal/dispatch"
	"github.com/poconnor/slack-cli/internal/registry"
)

func TestBuilderIntegrationWithExecutor(t *testing.T) {
	dispatch.ClearDispatch()
	dispatch.RegisterDispatch("chat.postMessage", func(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {
		ch, _ := flags["channel"].(string)
		text, _ := flags["text"].(string)
		return map[string]string{"channel": ch, "text": text}, nil
	})

	reg := []registry.MethodDef{
		{
			APIMethod:   "chat.postMessage",
			Category:    "chat",
			Command:     "post-message",
			Description: "Post a message",
			Params: []registry.ParamDef{
				{Name: "channel", Type: "string", Required: true, Description: "Channel ID"},
				{Name: "text", Type: "string", Required: true, Description: "Message text"},
			},
		},
	}

	root := &cobra.Command{Use: "slack-cli"}
	root.PersistentFlags().Bool("pretty", false, "")
	var stdout bytes.Buffer
	dispatch.BuildCommandsWithClient(root, reg, nil, nil, &stdout)

	root.SetArgs([]string{"chat", "post-message", "--channel", "C123", "--text", "hello"})
	err := root.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	if diff := cmp.Diff(true, stdout.Len() > 0); diff != "" {
		t.Errorf("expected output, got empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/dispatch/... -v -run TestBuilderIntegration
```

Expected: compilation error -- `BuildCommandsWithClient` not found.

- [ ] **Step 3: Update builder to wire executor and output**

Update `internal/dispatch/builder.go` -- add `BuildCommandsWithClient`:

```go
func BuildCommandsWithClient(root *cobra.Command, reg []registry.MethodDef, overrides map[string]*cobra.Command, client *slack.Client, stdout io.Writer) {
	groups := registry.GroupByCategory(reg)
	categories := make([]string, 0, len(groups))
	for cat := range groups {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, category := range categories {
		methods := groups[category]
		groupCmd := &cobra.Command{
			Use:   category,
			Short: fmt.Sprintf("Slack %s API methods", category),
		}
		for _, m := range methods {
			if overrides != nil {
				if override, ok := overrides[m.APIMethod]; ok {
					groupCmd.AddCommand(override)
					continue
				}
			}
			groupCmd.AddCommand(buildWiredCommand(m, client, stdout))
		}
		root.AddCommand(groupCmd)
	}
}

func buildWiredCommand(m registry.MethodDef, client *slack.Client, stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     m.Command,
		Short:   m.Description,
		Aliases: m.Aliases,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := extractFlags(cmd, m)
			pretty, _ := cmd.Flags().GetBool("pretty")
			fetchAll, _ := cmd.Flags().GetBool("all")
			limit, _ := cmd.Flags().GetInt("limit")
			maxResults, _ := cmd.Flags().GetInt("max-results")

			var result any
			var err error
			if m.Paginated && fetchAll {
				result, err = Paginate(cmd.Context(), client, m, flags, true, limit, maxResults)
			} else {
				result, err = Execute(cmd.Context(), client, m.APIMethod, flags)
			}
			if err != nil {
				FormatError(os.Stderr, err.Error(), exitcode.Classify(err))
				os.Exit(exitcode.Classify(err))
			}
			return FormatOutput(stdout, result, pretty)
		},
	}
	for _, p := range m.Params {
		switch p.Type {
		case "int":
			cmd.Flags().Int(p.Name, 0, p.Description)
		case "bool":
			cmd.Flags().Bool(p.Name, false, p.Description)
		case "string-slice":
			cmd.Flags().StringSlice(p.Name, nil, p.Description)
		default:
			cmd.Flags().String(p.Name, p.Default, p.Description)
		}
		if p.Required {
			cmd.MarkFlagRequired(p.Name)
		}
	}
	return cmd
}

func extractFlags(cmd *cobra.Command, m registry.MethodDef) map[string]any {
	flags := make(map[string]any)
	for _, p := range m.Params {
		switch p.Type {
		case "int":
			v, _ := cmd.Flags().GetInt(p.Name)
			if v != 0 {
				flags[p.Name] = v
			}
		case "bool":
			v, _ := cmd.Flags().GetBool(p.Name)
			if v {
				flags[p.Name] = v
			}
		case "string-slice":
			v, _ := cmd.Flags().GetStringSlice(p.Name)
			if len(v) > 0 {
				flags[p.Name] = v
			}
		default:
			v, _ := cmd.Flags().GetString(p.Name)
			if v != "" {
				flags[p.Name] = v
			}
		}
	}
	return flags
}
```

Add imports for `io`, `os`, `github.com/slack-go/slack`, and `github.com/poconnor/slack-cli/internal/exitcode`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/dispatch/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Update main.go to use BuildCommandsWithClient**

Update `cmd/slack-cli/main.go` to:
1. Read `SLACK_TOKEN` from env
2. Create `slack.Client` (lazily, only when a command runs)
3. Call `dispatch.BuildCommandsWithClient`

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"

	"github.com/poconnor/slack-cli/internal/dispatch"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/poconnor/slack-cli/internal/override"
	"github.com/poconnor/slack-cli/internal/registry"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	root := &cobra.Command{
		Use:   "slack-cli",
		Short: "Slack Web API CLI",
		Long:  "CLI for the full Slack Web API. JSON output by default, --pretty for humans.",
	}

	root.PersistentFlags().Bool("pretty", false, "Human-readable output")
	root.PersistentFlags().Bool("all", false, "Auto-paginate all results")
	root.PersistentFlags().Int("limit", 0, "Max results with --all")
	root.PersistentFlags().String("cursor", "", "Pagination cursor")
	root.PersistentFlags().Duration("timeout", 30*time.Second, "Request timeout")
	root.PersistentFlags().Bool("debug", false, "Debug HTTP traffic to stderr (tokens redacted)")
	root.PersistentFlags().Bool("wait-on-rate-limit", false, "Sleep and retry on rate limit")
	root.PersistentFlags().Int("max-results", 10000, "Hard cap on --all pagination results")

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("slack-cli %s (commit %s, built %s)\n", version, commit, date)
		},
	})

	token := os.Getenv("SLACK_TOKEN")
	var client *slack.Client
	if token != "" {
		client = slack.New(token)
	}

	dispatch.BuildCommandsWithClient(root, registry.Registry, override.Overrides, client, os.Stdout)

	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(exitcode.InputError)
	}
}
```

- [ ] **Step 6: Verify it builds**

```bash
go build -o bin/slack-cli ./cmd/slack-cli
./bin/slack-cli --help
./bin/slack-cli version
```

Expected: builds and runs. No API categories shown yet (registry is empty).

- [ ] **Step 7: Commit**

```bash
git add internal/dispatch/builder.go internal/dispatch/integration_test.go cmd/slack-cli/main.go
git commit -m "feat: wire builder, executor, output, and pagination into main entry point"
```

---

## Task 10: Code Generator -- Introspect SDK

**Files:**
- Create: `cmd/introspect/main.go`
- Create: `cmd/introspect/templates.go`

This is the most complex task. The generator parses `slack-go/slack` and emits two files:
1. `internal/registry/generated.go` -- the method table
2. `internal/dispatch/generated_dispatch.go` -- type-safe dispatch functions

- [ ] **Step 1: Create the generator entry point**

Create `cmd/introspect/main.go`:

```go
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

type methodInfo struct {
	APIMethod   string
	Category    string
	Command     string
	SDKMethod   string
	Description string
	Params      []paramInfo
	Paginated   bool
	CursorParam string
	CursorField string
	CallStyle   string
	ResponseKey string
}

type paramInfo struct {
	Name     string
	SDKName  string
	Type     string
	Required bool
	Desc     string
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("introspect: ")

	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedName,
	}
	pkgs, err := packages.Load(cfg, "github.com/slack-go/slack")
	if err != nil {
		log.Fatalf("loading slack package: %v", err)
	}
	if len(pkgs) == 0 {
		log.Fatal("no packages loaded")
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		for _, e := range pkg.Errors {
			log.Printf("package error: %v", e)
		}
		log.Fatal("package has errors")
	}

	clientType := findClientType(pkg)
	if clientType == nil {
		log.Fatal("could not find *slack.Client type")
	}

	methods := extractMethods(clientType, pkg)
	methods = filterMethods(methods)
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].APIMethod < methods[j].APIMethod
	})

	log.Printf("found %d methods", len(methods))

	projectRoot := findProjectRoot()

	registryPath := filepath.Join(projectRoot, "internal", "registry", "generated.go")
	writeRegistry(registryPath, methods)

	dispatchPath := filepath.Join(projectRoot, "internal", "dispatch", "generated_dispatch.go")
	writeDispatch(dispatchPath, methods)

	log.Printf("generated %s and %s", registryPath, dispatchPath)
}

func findClientType(pkg *packages.Package) *types.Named {
	obj := pkg.Types.Scope().Lookup("Client")
	if obj == nil {
		return nil
	}
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil
	}
	return named
}

func extractMethods(named *types.Named, pkg *packages.Package) []methodInfo {
	ptr := types.NewPointer(named)
	mset := types.NewMethodSet(ptr)
	var methods []methodInfo

	for i := 0; i < mset.Len(); i++ {
		sel := mset.At(i)
		fn, ok := sel.Obj().(*types.Func)
		if !ok {
			continue
		}
		name := fn.Name()
		if !strings.HasSuffix(name, "Context") {
			continue
		}
		if !fn.Exported() {
			continue
		}

		baseName := strings.TrimSuffix(name, "Context")
		apiMethod := guessAPIMethod(baseName)
		if apiMethod == "" {
			continue
		}

		sig := fn.Type().(*types.Signature)
		params := extractParams(sig)
		paginated, cursorParam, cursorField := detectPagination(sig, params)
		callStyle := detectCallStyle(sig)

		parts := strings.SplitN(apiMethod, ".", 2)
		category := parts[0]
		action := ""
		if len(parts) > 1 {
			action = camelToKebab(parts[1])
		}

		methods = append(methods, methodInfo{
			APIMethod:   apiMethod,
			Category:    category,
			Command:     action,
			SDKMethod:   name,
			Description: fmt.Sprintf("%s (%s)", humanize(apiMethod), apiMethod),
			Params:      params,
			Paginated:   paginated,
			CursorParam: cursorParam,
			CursorField: cursorField,
			CallStyle:   callStyle,
			ResponseKey: guessResponseKey(apiMethod),
		})
	}
	return methods
}

// guessAPIMethod maps Go method names to Slack API method names.
// This is a heuristic; the supplementary mapping table handles edge cases.
func guessAPIMethod(baseName string) string {
	if v, ok := methodMapping[baseName]; ok {
		return v
	}
	return ""
}

func filterMethods(methods []methodInfo) []methodInfo {
	var filtered []methodInfo
	for _, m := range methods {
		if strings.HasPrefix(m.APIMethod, "admin.") {
			continue
		}
		filtered = append(filtered, m)
	}
	return filtered
}

func extractParams(sig *types.Signature) []paramInfo {
	var params []paramInfo
	tup := sig.Params()
	for i := 0; i < tup.Len(); i++ {
		p := tup.At(i)
		typStr := p.Type().String()
		if strings.Contains(typStr, "context.Context") {
			continue
		}
		pName := p.Name()
		if pName == "" {
			pName = fmt.Sprintf("arg%d", i)
		}
		paramType := goTypeToFlagType(p.Type())
		params = append(params, paramInfo{
			Name:    camelToKebab(pName),
			SDKName: pName,
			Type:    paramType,
			Desc:    fmt.Sprintf("%s parameter", pName),
		})
	}
	return params
}

func detectPagination(sig *types.Signature, params []paramInfo) (bool, string, string) {
	for _, p := range params {
		if strings.Contains(strings.ToLower(p.SDKName), "cursor") {
			return true, p.Name, "next_cursor"
		}
	}
	results := sig.Results()
	for i := 0; i < results.Len(); i++ {
		r := results.At(i)
		if r.Name() == "nextCursor" || strings.Contains(r.Type().String(), "string") {
			// Heuristic: if there's a string return that might be a cursor
		}
	}
	return false, "", ""
}

func detectCallStyle(sig *types.Signature) string {
	tup := sig.Params()
	for i := 0; i < tup.Len(); i++ {
		p := tup.At(i)
		if sig.Variadic() && i == tup.Len()-1 {
			typStr := p.Type().String()
			if strings.Contains(typStr, "MsgOption") {
				return "msgoption"
			}
		}
		typStr := p.Type().String()
		if strings.Contains(typStr, "*") && !strings.Contains(typStr, "context") {
			return "struct"
		}
	}
	return "positional"
}

func camelToKebab(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			result = append(result, '-')
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}

func humanize(apiMethod string) string {
	parts := strings.SplitN(apiMethod, ".", 2)
	if len(parts) < 2 {
		return apiMethod
	}
	action := parts[1]
	// Split camelCase
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	action = re.ReplaceAllString(action, "${1} ${2}")
	return strings.Title(strings.ToLower(action))
}

func goTypeToFlagType(t types.Type) string {
	underlying := t.Underlying().String()
	switch {
	case strings.Contains(underlying, "int"):
		return "int"
	case strings.Contains(underlying, "bool"):
		return "bool"
	case strings.Contains(underlying, "[]string"):
		return "string-slice"
	default:
		return "string"
	}
}

func guessResponseKey(apiMethod string) string {
	parts := strings.SplitN(apiMethod, ".", 2)
	category := parts[0]
	switch category {
	case "conversations":
		return "channels"
	case "users":
		return "members"
	case "files":
		return "files"
	case "reactions":
		return "items"
	default:
		return ""
	}
}

func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("cannot get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			log.Fatal("cannot find project root (no go.mod)")
		}
		dir = parent
	}
}

func writeFile(path string, content []byte) {
	formatted, err := format.Source(content)
	if err != nil {
		log.Printf("WARNING: generated code has formatting issues: %v", err)
		formatted = content
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatalf("creating directory: %v", err)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		log.Fatalf("writing %s: %v", path, err)
	}
}

func writeRegistry(path string, methods []methodInfo) {
	var buf bytes.Buffer
	buf.WriteString(registryHeader)
	for _, m := range methods {
		buf.WriteString(fmt.Sprintf("\t{\n"))
		buf.WriteString(fmt.Sprintf("\t\tAPIMethod:   %q,\n", m.APIMethod))
		buf.WriteString(fmt.Sprintf("\t\tCategory:    %q,\n", m.Category))
		buf.WriteString(fmt.Sprintf("\t\tCommand:     %q,\n", m.Command))
		buf.WriteString(fmt.Sprintf("\t\tSDKMethod:   %q,\n", m.SDKMethod))
		buf.WriteString(fmt.Sprintf("\t\tDescription: %q,\n", m.Description))
		if m.Paginated {
			buf.WriteString(fmt.Sprintf("\t\tPaginated:   true,\n"))
			buf.WriteString(fmt.Sprintf("\t\tCursorParam: %q,\n", m.CursorParam))
			buf.WriteString(fmt.Sprintf("\t\tCursorField: %q,\n", m.CursorField))
		}
		if m.CallStyle != "" {
			buf.WriteString(fmt.Sprintf("\t\tCallStyle:   %q,\n", m.CallStyle))
		}
		if m.ResponseKey != "" {
			buf.WriteString(fmt.Sprintf("\t\tResponseKey: %q,\n", m.ResponseKey))
		}
		if len(m.Params) > 0 {
			buf.WriteString("\t\tParams: []registry.ParamDef{\n")
			for _, p := range m.Params {
				buf.WriteString(fmt.Sprintf("\t\t\t{Name: %q, SDKName: %q, Type: %q, Required: %v, Description: %q},\n",
					p.Name, p.SDKName, p.Type, p.Required, p.Desc))
			}
			buf.WriteString("\t\t},\n")
		}
		buf.WriteString("\t},\n")
	}
	buf.WriteString("}\n")
	writeFile(path, buf.Bytes())
}

func writeDispatch(path string, methods []methodInfo) {
	var buf bytes.Buffer
	buf.WriteString(dispatchHeader)

	// Write dispatch map
	buf.WriteString("var generatedDispatch = map[string]DispatchFunc{\n")
	for _, m := range methods {
		funcName := "dispatch" + strings.ReplaceAll(m.SDKMethod, "Context", "")
		buf.WriteString(fmt.Sprintf("\t%q: %s,\n", m.APIMethod, funcName))
	}
	buf.WriteString("}\n\n")

	buf.WriteString("func init() {\n")
	buf.WriteString("\tfor k, v := range generatedDispatch {\n")
	buf.WriteString("\t\tRegisterDispatch(k, v)\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	// Write stub dispatch functions
	for _, m := range methods {
		funcName := "dispatch" + strings.ReplaceAll(m.SDKMethod, "Context", "")
		buf.WriteString(fmt.Sprintf("func %s(ctx context.Context, client *slack.Client, flags map[string]any) (any, error) {\n", funcName))
		buf.WriteString(fmt.Sprintf("\t// TODO: implement type-safe dispatch for %s\n", m.APIMethod))
		buf.WriteString(fmt.Sprintf("\treturn nil, fmt.Errorf(\"dispatch not yet implemented for %s\")\n", m.APIMethod))
		buf.WriteString("}\n\n")
	}
	writeFile(path, buf.Bytes())
}

const registryHeader = `// Code generated by cmd/introspect; DO NOT EDIT.

package registry

func init() {
	Registry = generatedRegistry
}

var generatedRegistry = []MethodDef{
`

const dispatchHeader = `// Code generated by cmd/introspect; DO NOT EDIT.

package dispatch

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
)

`
```

- [ ] **Step 2: Create the method mapping table**

Create `cmd/introspect/mapping.go`:

```go
package main

// methodMapping maps Go method base names (without "Context" suffix)
// to Slack API method names. This is the source of truth for mapping.
// Methods not in this map are skipped during generation.
var methodMapping = map[string]string{
	// api
	"AuthTest": "auth.test",

	// auth
	"RevokeToken": "auth.revoke",

	// bookmarks
	"AddBookmark":    "bookmarks.add",
	"EditBookmark":   "bookmarks.edit",
	"ListBookmarks":  "bookmarks.list",
	"RemoveBookmark": "bookmarks.remove",

	// bots
	"GetBotInfo": "bots.info",

	// chat
	"PostMessage":        "chat.postMessage",
	"PostEphemeral":      "chat.postEphemeral",
	"UpdateMessage":      "chat.update",
	"DeleteMessage":      "chat.delete",
	"GetPermalink":       "chat.getPermalink",
	"UnfurlMessage":      "chat.unfurl",
	"ScheduleMessage":    "chat.scheduleMessage",
	"SendMessage":        "chat.postMessage", // alias

	// conversations
	"ArchiveConversation":       "conversations.archive",
	"UnArchiveConversation":     "conversations.unarchive",
	"CloseConversation":         "conversations.close",
	"CreateConversation":        "conversations.create",
	"GetConversationHistory":    "conversations.history",
	"GetConversationInfo":       "conversations.info",
	"InviteUsersToConversation": "conversations.invite",
	"JoinConversation":          "conversations.join",
	"KickUserFromConversation":  "conversations.kick",
	"LeaveConversation":         "conversations.leave",
	"GetConversations":          "conversations.list",
	"OpenConversation":          "conversations.open",
	"RenameConversation":        "conversations.rename",
	"GetConversationReplies":    "conversations.replies",
	"SetPurposeOfConversation":  "conversations.setPurpose",
	"SetTopicOfConversation":    "conversations.setTopic",
	"MarkConversation":          "conversations.mark",
	"GetUsersInConversation":    "conversations.members",
	"InviteSharedToConversation": "conversations.inviteShared",

	// dnd
	"EndDND":      "dnd.endDnd",
	"EndSnooze":   "dnd.endSnooze",
	"GetDNDInfo":  "dnd.info",
	"SetSnooze":   "dnd.setSnooze",
	"GetDNDTeamInfo": "dnd.teamInfo",

	// emoji
	"GetEmoji": "emoji.list",

	// files
	"GetFiles":                  "files.list",
	"GetFileInfo":               "files.info",
	"DeleteFile":                "files.delete",
	"ShareFilePublicURL":        "files.sharedPublicURL",
	"RevokeFilePublicURL":       "files.revokePublicURL",
	"UploadFileV2":              "files.uploadV2",
	"GetUploadURLExternal":      "files.getUploadURLExternal",
	"CompleteUploadExternal":    "files.completeUploadExternal",
	"ListFiles":                 "files.list",

	// pins
	"AddPin":    "pins.add",
	"RemovePin": "pins.remove",
	"ListPins":  "pins.list",

	// reactions
	"AddReaction":    "reactions.add",
	"RemoveReaction": "reactions.remove",
	"GetReactions":   "reactions.get",
	"ListReactions":  "reactions.list",

	// reminders
	"AddReminder":      "reminders.add",
	"DeleteReminder":   "reminders.delete",
	"ListReminders":    "reminders.list",
	"CompleteReminder": "reminders.complete",

	// search
	"SearchMessages": "search.messages",
	"SearchFiles":    "search.files",

	// stars
	"AddStar":    "stars.add",
	"RemoveStar": "stars.remove",
	"ListStars":  "stars.list",

	// team
	"GetTeamInfo":   "team.info",
	"GetAccessLogs": "team.accessLogs",
	"GetBillableInfo": "team.billableInfo",

	// usergroups
	"CreateUserGroup":           "usergroups.create",
	"DisableUserGroup":          "usergroups.disable",
	"EnableUserGroup":           "usergroups.enable",
	"GetUserGroups":             "usergroups.list",
	"UpdateUserGroup":           "usergroups.update",
	"GetUserGroupMembers":       "usergroups.users.list",
	"UpdateUserGroupMembers":    "usergroups.users.update",

	// users
	"GetUserInfo":        "users.info",
	"GetUsers":           "users.list",
	"GetUserPresence":    "users.getPresence",
	"SetUserPresence":    "users.setPresence",
	"GetUserProfile":     "users.profile.get",
	"SetUserCustomStatus": "users.profile.set",
	"GetUserIdentity":    "users.identity",
	"SetUserPhoto":       "users.setPhoto",
	"DeleteUserPhoto":    "users.deletePhoto",
	"GetUsersConversations": "users.conversations",
	"LookupUserByEmail":  "users.lookupByEmail",

	// views
	"OpenView":   "views.open",
	"PushView":   "views.push",
	"UpdateView": "views.update",
	"PublishView": "views.publish",

	// dialog
	"OpenDialog": "dialog.open",
}
```

- [ ] **Step 3: Add go:generate directive**

Add to `internal/registry/method.go` at the top (after package declaration):

```go
//go:generate go run ../../cmd/introspect
```

- [ ] **Step 4: Run the generator**

```bash
go run ./cmd/introspect
```

Expected: generates `internal/registry/generated.go` and `internal/dispatch/generated_dispatch.go` with method entries.

- [ ] **Step 5: Verify the generated files compile**

```bash
go build ./...
```

Expected: clean compilation.

- [ ] **Step 6: Run full test suite**

```bash
go test -race -count=1 ./...
```

Expected: all tests PASS.

- [ ] **Step 7: Test the CLI**

```bash
go build -o bin/slack-cli ./cmd/slack-cli
./bin/slack-cli --help
./bin/slack-cli chat --help
./bin/slack-cli conversations --help
```

Expected: help text shows categories and commands generated from the SDK.

- [ ] **Step 8: Commit**

```bash
git add cmd/introspect/ internal/registry/generated.go internal/dispatch/generated_dispatch.go internal/registry/method.go
git commit -m "feat: add code generator that introspects slack-go/slack and emits registry + dispatch"
```

---

## Task 11: API List Override Command

**Files:**
- Modify: `internal/override/override.go`
- Create: `internal/override/api_list.go`
- Create: `internal/override/api_list_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/override/api_list_test.go`:

```go
package override_test

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"

	"github.com/poconnor/slack-cli/internal/override"
	"github.com/poconnor/slack-cli/internal/registry"
)

func TestAPIListCommand(t *testing.T) {
	registry.Registry = []registry.MethodDef{
		{APIMethod: "chat.postMessage", Category: "chat", Command: "post-message", Description: "Post a message"},
		{APIMethod: "conversations.list", Category: "conversations", Command: "list", Description: "List conversations"},
	}

	root := &cobra.Command{Use: "slack-cli"}
	override.RegisterBuiltins(root)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"api", "list"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got := out.String()
	if !bytes.Contains([]byte(got), []byte("chat.postMessage")) {
		t.Errorf("output missing chat.postMessage:\n%s", got)
	}
	if !bytes.Contains([]byte(got), []byte("conversations.list")) {
		t.Errorf("output missing conversations.list:\n%s", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/override/... -v
```

Expected: compilation error.

- [ ] **Step 3: Implement api list command**

Create `internal/override/api_list.go`:

```go
package override

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/poconnor/slack-cli/internal/registry"
)

func RegisterBuiltins(root *cobra.Command) {
	apiCmd := &cobra.Command{
		Use:   "api",
		Short: "API discovery commands",
	}
	apiCmd.AddCommand(apiListCmd())
	root.AddCommand(apiCmd)
}

func apiListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all available Slack API methods",
		RunE: func(cmd *cobra.Command, args []string) error {
			category, _ := cmd.Flags().GetString("category")
			pretty, _ := cmd.Flags().GetBool("pretty")
			asJSON, _ := cmd.Flags().GetBool("json")

			methods := registry.Registry
			if category != "" {
				var filtered []registry.MethodDef
				for _, m := range methods {
					if m.Category == category {
						filtered = append(filtered, m)
					}
				}
				methods = filtered
			}

			sort.Slice(methods, func(i, j int) bool {
				return methods[i].APIMethod < methods[j].APIMethod
			})

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(methods)
			}

			if pretty {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(tw, "COMMAND\tAPI METHOD\tDESCRIPTION")
				for _, m := range methods {
					fmt.Fprintf(tw, "%s %s\t%s\t%s\n", m.Category, m.Command, m.APIMethod, m.Description)
				}
				return tw.Flush()
			}

			for _, m := range methods {
				fmt.Fprintln(cmd.OutOrStdout(), m.APIMethod)
			}
			return nil
		},
	}
	cmd.Flags().String("category", "", "Filter by category")
	cmd.Flags().Bool("pretty", false, "Pretty table output")
	cmd.Flags().Bool("json", false, "JSON output")
	return cmd
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/override/... -v
```

Expected: PASS.

- [ ] **Step 5: Wire into main.go**

Add `override.RegisterBuiltins(root)` to `cmd/slack-cli/main.go` before `BuildCommandsWithClient`.

- [ ] **Step 6: Test it**

```bash
go build -o bin/slack-cli ./cmd/slack-cli
./bin/slack-cli api list
./bin/slack-cli api list --pretty
```

Expected: lists all generated API methods.

- [ ] **Step 7: Commit**

```bash
git add internal/override/ cmd/slack-cli/main.go
git commit -m "feat: add api list discovery command"
```

---

## Task 12: End-to-End Smoke Test

**Files:**
- Create: `e2e_test.go`

- [ ] **Step 1: Write E2E test**

Create `e2e_test.go` at project root:

```go
//go:build e2e

package main_test

import (
	"encoding/json"
	"os/exec"
	"testing"
)

func TestE2EVersion(t *testing.T) {
	out, err := exec.Command("./bin/slack-cli", "version").Output()
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("version produced no output")
	}
	t.Logf("version output: %s", out)
}

func TestE2EHelp(t *testing.T) {
	out, err := exec.Command("./bin/slack-cli", "--help").Output()
	if err != nil {
		t.Fatalf("help command failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("help produced no output")
	}
}

func TestE2EAPIList(t *testing.T) {
	out, err := exec.Command("./bin/slack-cli", "api", "list", "--json").Output()
	if err != nil {
		t.Fatalf("api list failed: %v", err)
	}
	var methods []any
	if err := json.Unmarshal(out, &methods); err != nil {
		t.Fatalf("api list output is not valid JSON: %v", err)
	}
	if len(methods) == 0 {
		t.Fatal("api list returned empty array")
	}
	t.Logf("api list returned %d methods", len(methods))
}

func TestE2EMissingToken(t *testing.T) {
	cmd := exec.Command("./bin/slack-cli", "chat", "post-message", "--channel", "C123", "--text", "test")
	cmd.Env = []string{} // empty env, no SLACK_TOKEN
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when SLACK_TOKEN is missing")
	}
	t.Logf("missing token output: %s", out)
}
```

- [ ] **Step 2: Build and run E2E tests**

```bash
make build
go test -tags e2e -v -run TestE2E ./...
```

Expected: version, help, and api list tests PASS. Missing token test may need adjustment based on actual error behavior.

- [ ] **Step 3: Commit**

```bash
git add e2e_test.go
git commit -m "test: add end-to-end smoke tests for CLI binary"
```

---

## Task 13: Final Cleanup and Documentation

**Files:**
- Create: `README.md`
- Modify: `Makefile` (add e2e target)

- [ ] **Step 1: Create README**

Create `README.md`:

```markdown
# slack-cli

A CLI for the full Slack Web API. JSON output by default, `--pretty` for humans.

Built for AI agents and automation. Wraps the [slack-go/slack](https://github.com/slack-go/slack) SDK with auto-generated commands covering 100+ API methods.

## Installation

```bash
go install github.com/poconnor/slack-cli/cmd/slack-cli@latest
```

Or build from source:

```bash
make build
# Binary at bin/slack-cli
```

## Quick Start

```bash
export SLACK_TOKEN=xoxb-your-token-here

# List channels
slack-cli conversations list --pretty

# Send a message
slack-cli chat post-message --channel C01ABC23DEF --text "Hello from CLI"

# Get user info
slack-cli users info --user U01ABC23DEF
```

## Authentication

Set `SLACK_TOKEN` environment variable:

```bash
export SLACK_TOKEN=xoxb-your-bot-token
```

Or pipe from a secret manager:

```bash
vault read -field=token secret/slack | slack-cli conversations list
```

## Command Structure

Commands map directly to Slack API methods:

```
slack-cli <category> <action> [flags]
```

Discover available commands:

```bash
slack-cli --help              # List categories
slack-cli chat --help         # List chat commands
slack-cli api list            # List all API methods
slack-cli api list --pretty   # Pretty table format
```

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | false | Human-readable table output |
| `--all` | false | Auto-paginate all results |
| `--limit` | 0 | Max results with `--all` |
| `--cursor` | "" | Manual pagination cursor |
| `--timeout` | 30s | Request timeout |
| `--debug` | false | HTTP debug to stderr (tokens redacted) |
| `--max-results` | 10000 | Hard cap on `--all` results |
| `--wait-on-rate-limit` | false | Auto-retry on rate limit |

## Output

JSON by default (for agents and piping to `jq`):

```bash
slack-cli conversations list | jq '.channels[].name'
```

Human-readable with `--pretty`:

```bash
slack-cli conversations list --all --pretty
```

## Error Handling

| Exit Code | Meaning |
|-----------|---------|
| 0 | Success |
| 1 | Slack API error |
| 2 | Authentication error |
| 3 | Invalid input |
| 4 | Network error |

Errors are written to stderr as JSON. stdout contains only response data.

## Development

```bash
make generate   # Regenerate from SDK
make build      # Build binary
make test       # Run tests
make lint       # Run linter
```
```

- [ ] **Step 2: Update Makefile with e2e target**

Add to `Makefile`:

```makefile
e2e: build
	go test -tags e2e -v -run TestE2E ./...
```

- [ ] **Step 3: Run full test suite one last time**

```bash
make test
```

Expected: all tests PASS.

- [ ] **Step 4: Commit**

```bash
git add README.md Makefile
git commit -m "docs: add README and e2e Makefile target"
```
