# Design: `thread-read` and `message-read` commands

**Date:** 2026-04-13
**Author:** Peter O'Connor with Claude Code (claude-sonnet-4-6)
**Status:** Approved

---

## Problem

Reading a Slack thread or single message via the CLI currently requires:

1. Running `--help` to discover the right subcommand hierarchy
2. Making two separate API calls for a thread (root + replies are different endpoints)
3. Piping raw JSON through Python to extract human-readable text
4. Making N additional `users.info` calls to resolve raw user IDs to names
5. Dealing with `grep -P` platform failures and opaque nested API metadata

The result for a single thread: ~8 tool calls, two of which return no useful output.

The desired interface is a single command that returns chronological, name-resolved, text-only output:

```
Peter O'Connor [2026-04-10 09:18]: This ODR affects y'all...
Brendan Rosage [2026-04-10 09:32]: This doc provides context...
```

---

## Approach

Two new top-level override commands — `thread-read` and `message-read` — following the existing `cache` / `resolve` / `api` pattern in `internal/override/`. Hand-written Cobra commands for anything that composes multiple concerns or needs custom presentation.

User name resolution uses a new `id-to-name.json` cache file (reverse index of the existing `people.json`), populated at warm time and derived in-place during a v2→v3 schema migration with no API calls.

---

## Cache changes (v2 → v3)

### New file: `id-to-name.json`

A flat `map[string]string` of `userID → displayName`. Prefers `profile.DisplayName`, falls back to `profile.Name`.

### Constants (`cache.go`)

```go
const IDToNameFileName = "id-to-name.json"
const CurrentVersion    = 3
```

### New function: `ResolveUserByID`

```go
func ResolveUserByID(id string) (name string, found bool, err error)
```

- Returns `(displayName, true, nil)` on hit
- Returns `("", false, nil)` on miss — not found is not an error; the caller decides the fallback policy
- Returns `("", false, err)` only on file I/O or parse failure

Callers (`thread-read`, `message-read`) treat `found == false` as "show raw ID". This policy lives in the command layer, not the cache layer.

For bulk resolution (thread with N messages), commands use a separate loader:

```go
func LoadIDToNameMap() (map[string]string, error)
```

This loads `id-to-name.json` **once per command invocation** and returns the full map. Commands do direct map lookups per message — no repeated file I/O. `ResolveUserByID` is for single ad-hoc lookups (e.g., future commands needing one resolution).

### Migration v2 → v3 (`migrate.go`)

New `migrateV2toV3` function:

1. Read existing `people.json`
2. Build reverse map: `make(map[string]string, len(people))` (preallocated — size is known)
3. Write `id-to-name.json` via `SaveEntity` (atomic rename)
4. Call `SaveMeta(CacheMeta{Version: 3})`

**Write order is mandatory:** `id-to-name.json` MUST be written before `SaveMeta` bumps the version. If the meta bump succeeds but the file write failed, the cache would claim v3 without the file. The existing `SaveEntity` atomic rename pattern handles this correctly if order is respected.

### `EnsureReady` — correctness fix

`EnsureReady` must add an explicit `case 2:` branch. Without it, v2 caches fall through to `default` (staleness check) and skip the migration entirely.

```
case 0, 1: → enrichAndReturn (existing)
case 2:    → migrateV2toV3, then staleness check  ← NEW
case 3:    → staleness check (CurrentVersion)
default:   → treat as stale
```

### `cache warm`

Writes `id-to-name.json` alongside the three existing files. Map preallocated with `make(map[string]string, len(people))`.

### `cache clear`

References the `IDToNameFileName` constant (not a bare string literal) in the delete list.

### `cache info`

Loads `id-to-name.json` inside the existing shared lock block. Prints `ID mappings: N` only when the load succeeds — a missing file is not an error in `cache info`.

---

## URL parsing

Slack URL form:

```
https://<workspace>.slack.com/archives/<channelID>/p<ts_no_dot>
```

Timestamp reconstruction uses **string manipulation only** — no float round-trip (which introduces precision loss on 6-decimal timestamps). Strip the `p` prefix, insert `.` at `len(segment) - 6`:

```
p1775827095264229 → 1775827095.264229
```

### Function signature

**File:** `internal/override/slack_url.go`

```go
func parseSlackURL(rawURL string) (channel, ts string, err error)
```

Unexported. Tests are in `package override` (white-box, same package).

### Validation rules

- Path must have at least two segments after `/archives/`
- Channel segment must start with `C` (public/private channel) or `D` (DM)
- Timestamp segment must start with `p` and have length > 7

### Error messages (lowercase, no trailing punctuation)

- `"invalid slack url: missing /archives/ path"`
- `"invalid slack url: channel must start with C or D"`
- `"invalid slack url: missing timestamp segment"`
- `"invalid slack url: timestamp segment too short"`

`parseSlackURL` returns plain `error` values. The command's `RunE` handler calls `formatAndExit(cmd, err, exitcode.InputError)` — consistent with the established pattern throughout the codebase.

If any regexp is used, it MUST be a package-level compiled `var`. String manipulation is sufficient here; no regexp is needed.

---

## `thread-read` command

**File:** `internal/override/thread_read_cmd.go`

### Interface

```bash
slack-cli thread-read --url "https://stackexchange.slack.com/archives/C0AFM69EB1B/p1775827095264229"
slack-cli thread-read --channel C0AFM69EB1B --ts 1775827095.264229
slack-cli thread-read --url "..." --json
```

### Flag contract

Enforced at the framework layer — Cobra prevents `RunE` from being called when constraints are violated:

```go
cmd.MarkFlagsMutuallyExclusive("url", "channel")
cmd.MarkFlagsMutuallyExclusive("url", "ts")
cmd.MarkFlagsRequiredTogether("channel", "ts")
```

### Flow

1. Check `client != nil` — if nil: `formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)`
2. Parse `--url` or assemble from `--channel` + `--ts`; on parse error: `formatAndExit(..., exitcode.InputError)`
3. Call `ensureCacheReady` (handles migration and auto-warm if stale)
4. Call `cache.LoadIDToNameMap()` once; store result as local `idMap`
5. Call `conversations.replies` with `channel` + `ts` via `*slack.Client` directly — returns the full thread including the root message as the first element (one API call, not two)
6. For each message:
   - Bot detection: `if msg.BotID != "" || msg.User == ""` → display name is `"[bot]"`
   - Otherwise: look up `msg.User` in `idMap`; if not found, use the raw ID
   - Parse `msg.Timestamp` as `time.Unix` (UTC); timezone rendering is the formatter's responsibility
7. Call `formatMessages(msgs, asJSON, cmd.OutOrStdout())`; propagate write errors

### Default output

```
Peter O'Connor [2026-04-10 09:18]: This ODR affects y'all...
Brendan Rosage [2026-04-10 09:32]: This doc provides context...
```

Timestamp format: `2006-01-02 15:04` (local time).

### `--json` output

```json
[
  {"user": "Peter O'Connor", "ts": "2026-04-10T09:18:15-05:00", "text": "This ODR affects y'all..."}
]
```

- Timestamp format: `time.RFC3339` (not `time.RFC3339Nano`)
- `"user"` field: resolved display name when found; raw Slack user ID (e.g. `U03B00M8EKZ`) when not resolvable — callers must handle both

---

## `message-read` command

**File:** `internal/override/message_read_cmd.go`

### Interface

```bash
slack-cli message-read --url "https://stackexchange.slack.com/archives/C09C0KHRF9B/p1776101206614149"
slack-cli message-read --channel C09C0KHRF9B --ts 1776101206.614149
slack-cli message-read --url "..." --json
```

### Flag contract

Identical to `thread-read`: same `MarkFlagsMutuallyExclusive` / `MarkFlagsRequiredTogether` wiring, same `client == nil` guard.

### Flow

1. Check `client != nil`
2. Parse `--url` or flags; on parse error: `formatAndExit(..., exitcode.InputError)`
3. `ensureCacheReady`
4. Call `cache.LoadIDToNameMap()` once; store result as local `idMap`
5. Call `conversations.history` with `channel`, `latest=ts`, `oldest=ts`, `inclusive=true`, `limit=1` via `*slack.Client`
6. If no messages returned: `formatAndExit(cmd, fmt.Errorf("no message found in %s at %s", channel, ts), exitcode.InputError)`
7. Resolve user, detect bots, format — identical path to `thread-read`

### Output

Same format as `thread-read`. Single message.

---

## Shared formatting

**File:** `internal/override/read_format.go` (package `override`)

Co-located in `override` because it has no Cobra or Slack SDK dependencies and mirrors how `output.go` lives in `dispatch` for that package's formatting. Moving it to a sub-package would add overhead without meaningful gain at this project size.

```go
type readMessage struct {
    User string    // display name, "[bot]", or raw Slack user ID
    Time time.Time // value from time.Unix (UTC); formatter applies local timezone
    Text string
}

func formatMessages(msgs []readMessage, asJSON bool, w io.Writer) error
```

All write errors from `formatMessages` MUST be propagated to the caller.

Tests for `readMessage` and `formatMessages` are in `package override` (white-box, same package).

---

## New files

| File | Purpose |
|------|---------|
| `internal/override/thread_read_cmd.go` | `thread-read` Cobra command |
| `internal/override/message_read_cmd.go` | `message-read` Cobra command |
| `internal/override/slack_url.go` | URL parsing (unexported, white-box tested) |
| `internal/override/read_format.go` | `readMessage` struct + `formatMessages` |

## Modified files

| File | Change |
|------|--------|
| `internal/cache/cache.go` | `IDToNameFileName` constant, `CurrentVersion = 3` |
| `internal/cache/warm.go` | Write `id-to-name.json` (preallocated map) |
| `internal/cache/resolve.go` | Add `ResolveUserByID(id string) (string, bool, error)` |
| `internal/cache/migrate.go` | `migrateV2toV3`; `EnsureReady` gets explicit `case 2:` |
| `internal/override/api_list.go` | Register `thread-read` and `message-read` in `RegisterBuiltins` |

---

## Testing

| File | Cases |
|------|-------|
| `internal/override/slack_url_test.go` | Valid URL; missing `/archives/`; channel not `C`/`D`; missing timestamp segment; segment too short; DM URL (`D`-prefix) |
| `internal/cache/resolve_test.go` | `ResolveUserByID`: found; not found (`found=false`, no error); I/O failure (file missing). `LoadIDToNameMap`: happy path; missing file returns error |
| `internal/cache/migrate_test.go` | v2→v3: `id-to-name.json` derived from `people.json`; preallocated map; meta bumped to 3; write order (`id-to-name.json` before meta); existing data untouched |
| `internal/override/read_format_test.go` | Plain text: resolved name, `[bot]`, unresolved ID fallback; JSON: `time.RFC3339`, `"user"` fallback; write error propagation |

---

## Non-goals

- Reactions, attachments, block kit content — not included in default output
- Pagination for threads under ~50 messages — not required
- API metadata (ts, thread_ts, client_msg_id) — not shown unless `--json`
- Summaries — raw human text only, resolved and readable
