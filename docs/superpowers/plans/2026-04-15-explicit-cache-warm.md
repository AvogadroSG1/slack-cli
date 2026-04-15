# Explicit Cache Warm Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove automatic cache warming from resolve/read command paths; warn on stderr instead of blocking.

**Architecture:** Replace `ensureCacheReady` (which calls `cache.EnsureReady` + `cache.Warm`) with `warnIfCacheNotReady` (which calls `cache.EnsureReady` with nil fetcher for local-only migrations, then checks staleness and prints a warning). Five call sites updated, one new test file.

**Tech Stack:** Go, Cobra, standard `testing` + `go-cmp`

**Spec:** `docs/superpowers/specs/2026-04-15-explicit-cache-warm-design.md`

---

### Task 1: Write failing tests for `warnIfCacheNotReady`

**Files:**
- Create: `internal/override/warn_test.go`

- [ ] **Step 1: Create the test file with all 4 test cases**

```go
package override

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/spf13/cobra"
)

func TestWarnIfCacheNotReady(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string)
		wantStderr string
		wantEmpty  bool
	}{
		{
			name:       "stale_no_meta_file",
			setup:      func(t *testing.T, dir string) {},
			wantStderr: `cache not warmed`,
		},
		{
			name: "fresh_cache",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeMeta(t, dir, cache.CacheMeta{Version: cache.CurrentVersion})
			},
			wantEmpty: true,
		},
		{
			name: "stale_old_mtime",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeMeta(t, dir, cache.CacheMeta{Version: cache.CurrentVersion})
				p := filepath.Join(dir, cache.MetaFileName)
				old := time.Now().Add(-25 * time.Hour)
				if err := os.Chtimes(p, old, old); err != nil {
					t.Fatal(err)
				}
			},
			wantStderr: `cache not warmed`,
		},
		{
			name: "corrupted_meta",
			setup: func(t *testing.T, dir string) {
				t.Helper()
				p := filepath.Join(dir, cache.MetaFileName)
				if err := os.WriteFile(p, []byte(`{corrupt`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantStderr: `cache migration failed`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("SLACK_CLI_CACHE_DIR", dir)
			tt.setup(t, dir)

			cmd := &cobra.Command{Use: "test"}
			var errBuf bytes.Buffer
			cmd.SetErr(&errBuf)

			warnIfCacheNotReady(cmd)

			stderr := errBuf.String()
			if tt.wantEmpty {
				if stderr != "" {
					t.Errorf("want empty stderr, got %q", stderr)
				}
				return
			}
			if !strings.Contains(stderr, tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr, tt.wantStderr)
			}
		})
	}
}

// writeMeta writes a CacheMeta JSON file to dir.
func writeMeta(t *testing.T, dir string, m cache.CacheMeta) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, cache.MetaFileName), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -race -count=1 -run TestWarnIfCacheNotReady ./internal/override/`
Expected: FAIL — `warnIfCacheNotReady` is not defined (compilation error)

---

### Task 2: Implement `warnIfCacheNotReady` and delete `ensureCacheReady`

**Files:**
- Modify: `internal/override/resolve_cmd.go`

- [ ] **Step 1: Replace `ensureCacheReady` with `warnIfCacheNotReady`**

Delete lines 129-156 (the `ensureCacheReady` and `formatAndExit` functions). Replace `ensureCacheReady` with:

```go
// warnIfCacheNotReady runs local-only migrations and prints a warning
// to stderr if the cache is stale or empty. It never blocks or errors.
func warnIfCacheNotReady(cmd *cobra.Command) {
	// Run local-only migrations (pass nil fetcher to guarantee no API calls).
	_, err := cache.EnsureReady(cmd.Context(), nil)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: cache migration failed: %v\n", err)
		return
	}

	// IsStale returns true on error, so discarding err still produces a correct warning.
	stale, _ := cache.IsStale()
	if stale {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: cache not warmed. Run \"slack-cli cache warm\" for faster lookups.\n")
	}
}
```

Keep `formatAndExit` — it is still used by the resolve subcommands for their own errors.

- [ ] **Step 2: Update the 3 resolve call sites**

In `newResolveChannelCmd` (line 34), replace:
```go
			if err := ensureCacheReady(cmd, client); err != nil {
				return err
			}
```
with:
```go
			warnIfCacheNotReady(cmd)
```

In `newResolveUserCmd` (line 73), same replacement.

In `newResolveUsergroupCmd` (line 103), same replacement.

- [ ] **Step 3: Remove unused `slack` import if needed**

After removing all `ensureCacheReady` calls, the `client *slack.Client` parameter is still used by `newResolveCmd` to pass to subcommand constructors, but `warnIfCacheNotReady` no longer takes it. Check that the `"github.com/slack-go/slack"` import is still needed (it is — the subcommand constructors still accept `*slack.Client` for the resolve lock/load flow). No import changes needed in this file.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -count=1 -run TestWarnIfCacheNotReady ./internal/override/`
Expected: PASS — all 4 test cases green

- [ ] **Step 5: Run full test suite**

Run: `make test`
Expected: PASS — no regressions

- [ ] **Step 6: Commit**

```bash
git add internal/override/resolve_cmd.go internal/override/warn_test.go
git commit -m "feat(cache): replace auto-warm with stderr warning in resolve commands

Remove ensureCacheReady (which triggered automatic Slack API warming).
Add warnIfCacheNotReady which runs local-only migrations and prints a
warning when the cache is stale. Never blocks, never makes API calls.

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 3: Update `thread-read` and `message-read` commands

**Files:**
- Modify: `internal/override/thread_read_cmd.go`
- Modify: `internal/override/message_read_cmd.go`

- [ ] **Step 1: Update `thread_read_cmd.go`**

In `runThreadRead` (line 50), replace:
```go
	if err := ensureCacheReady(cmd, client); err != nil {
		return err
	}
```
with:
```go
	warnIfCacheNotReady(cmd)
```

- [ ] **Step 2: Update `message_read_cmd.go`**

In `runMessageRead` (line 57), replace:
```go
	if err := ensureCacheReady(cmd, client); err != nil {
		return err
	}
```
with:
```go
	warnIfCacheNotReady(cmd)
```

- [ ] **Step 3: Run existing tests**

Run: `go test -race -count=1 ./internal/override/`
Expected: PASS — existing tests for thread-read and message-read still pass (they test flag contracts and nil-client errors, not warming)

- [ ] **Step 4: Run full test suite**

Run: `make test`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/override/thread_read_cmd.go internal/override/message_read_cmd.go
git commit -m "feat(cache): replace auto-warm with warning in thread-read and message-read

Consistent with resolve commands: warn on stderr when cache is stale,
never block with automatic API calls.

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 4: Update `StaleDuration` comment

**Files:**
- Modify: `internal/cache/cache.go:34-36`

- [ ] **Step 1: Update the comment**

Replace lines 34-36:
```go
// StaleDuration is how long before the cache is considered stale and
// triggers an automatic warm on the next resolve.
const StaleDuration = 24 * time.Hour
```
with:
```go
// StaleDuration is how long before the cache is considered stale and
// triggers a warning on the next cache-dependent command. Warming is
// explicit via "slack-cli cache warm". 24 hours was chosen to nudge
// frequently enough without letting data go silently stale.
const StaleDuration = 24 * time.Hour
```

- [ ] **Step 2: Run tests**

Run: `make test`
Expected: PASS — comment-only change

- [ ] **Step 3: Commit**

```bash
git add internal/cache/cache.go
git commit -m "docs(cache): update StaleDuration comment for explicit warm workflow

Staleness now triggers a warning, not an automatic warm. Document the
24-hour threshold as a deliberate choice for the manual-warm workflow.

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>"
```

---

### Task 5: Build and install

**Files:**
- No source changes

- [ ] **Step 1: Build the binary**

Run: `make build`
Expected: Binary at `bin/slack-cli`

- [ ] **Step 2: Copy to project root**

Run: `cp bin/slack-cli ./slack-cli`

- [ ] **Step 3: Verify the binary works**

Run: `./slack-cli cache info`
Expected: Shows cache path, version, status (no crashes, no auto-warm)

Run: `./slack-cli resolve channel general 2>&1 >/dev/null || true`
Expected: If cache is stale, stderr shows `Warning: cache not warmed. Run "slack-cli cache warm" for faster lookups.` — no blocking API calls.

- [ ] **Step 4: Commit binary**

```bash
git add slack-cli
git commit -m "build: update slack-cli binary with explicit cache warm

Co-Authored-By: Peter O'Connor <poconnor@stackoverflow.com>
Co-Authored-By: Claude <noreply@anthropic.com>"
```
