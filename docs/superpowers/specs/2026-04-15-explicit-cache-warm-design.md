# Explicit Cache Warm Design

**Date:** 2026-04-15
**Status:** Approved
**Iteration:** 1 of 2

## Goal

Remove all automatic cache warming from the resolve/read command paths. Cache warming becomes an explicit, user-initiated action via `slack-cli cache warm`. Commands that depend on cache data warn on stderr but never block.

## Motivation

Auto-warming interrupts workflows. When a user runs `resolve` or `thread-read` and the cache is stale, the current system transparently fires off Slack API calls that can take seconds. This is surprising, blocks the terminal, and consumes API tokens without the user asking for it. Users should control when network I/O happens.

## Scope

**In scope (iteration 1):**

- Remove auto-warm from all cache-dependent command paths
- Replace `ensureCacheReady` with `warnIfCacheNotReady` — warns on stderr, never blocks
- Apply to all 5 cache-dependent commands
- Local-only migrations still run (v1/v2/v3 are file operations)

**Out of scope:**

- Live API fallback when cache is empty (iteration 2)
- Changes to `cache warm`, `cache info`, `cache clear`
- Changes to the cache data layer (`internal/cache/`)

## Design

### Delete `ensureCacheReady`

The function at `internal/override/resolve_cmd.go:131-150` is removed entirely. It currently calls `cache.EnsureReady` and then `cache.Warm` if the cache is stale. Both behaviors are removed from the command path.

### New function: `warnIfCacheNotReady`

Replaces `ensureCacheReady` in `resolve_cmd.go`:

```go
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

Key design decisions:

- **Passes `nil` as fetcher to `EnsureReady`.** This guarantees no Slack API calls. The `enrichAndReturn` path in `migrate.go:104-106` already handles `nil` fetcher gracefully by skipping enrichment.
- **Returns nothing.** This is a best-effort helper — it warns and proceeds, never blocks.
- **Logs migration errors to stderr.** A half-completed migration leaving broken cache files would produce confusing downstream errors. Surfacing the error lets users diagnose.
- **No ANSI color codes.** `cmd.ErrOrStderr()` returns `io.Writer` not `*os.File`, making TTY detection awkward. Plain `Warning:` prefix is portable and grep-friendly.
- **No `client` parameter.** Since we pass `nil` to `EnsureReady`, the Slack client is not needed. Simpler signature, simpler call sites.

### Call sites (5 commands)

Each command replaces an error-checked `ensureCacheReady(cmd, client)` call with a fire-and-forget `warnIfCacheNotReady(cmd)`:

| Command | File | Current code | New code |
|---------|------|-------------|----------|
| `resolve channel` | `resolve_cmd.go:34` | `if err := ensureCacheReady(cmd, client); err != nil { return err }` | `warnIfCacheNotReady(cmd)` |
| `resolve user` | `resolve_cmd.go:73` | same pattern | same |
| `resolve usergroup` | `resolve_cmd.go:103` | same pattern | same |
| `thread-read` | `thread_read_cmd.go:50` | same pattern | same |
| `message-read` | `message_read_cmd.go:57` | same pattern | same |

### Comment update

`StaleDuration` constant in `cache.go:36` — update comment to reflect that staleness triggers a warning, not an automatic warm. Note that 24 hours was a deliberate choice for the manual-warm workflow: frequent enough to nudge, not so long that data goes stale without notice.

## Behavior Matrix

| Cache state | What happens | User sees |
|-------------|-------------|-----------|
| Never warmed (no files) | Migration runs (no-op for fresh install), `IsStale` returns true | Warning on stderr, resolve fails with "not found" |
| Warmed and fresh (<24h) | Migration runs (no-op for v3), `IsStale` returns false | No warning, resolve succeeds |
| Warmed but stale (>24h) | Migration runs (no-op for v3), `IsStale` returns true | Warning on stderr, resolve succeeds from stale data |
| v1 legacy cache | Migration splits files (local), enrichment skipped (nil fetcher) | Warning on stderr (stale after migration), resolve works from flat data |
| v2 cache | Migration derives id-to-name.json (local), staleness check | Warning if stale, resolve works |
| Corrupted meta file | `EnsureReady` returns error | Migration failure warning on stderr |

## Testing

New file: `internal/override/warn_test.go`

| Test case | Setup | Assert |
|-----------|-------|--------|
| Stale cache (no meta file) | Empty temp cache dir | stderr contains staleness warning |
| Fresh cache | Write `cache-meta.json` with current mtime | stderr is empty |
| Stale cache (old mtime) | Write `cache-meta.json`, set mtime 25h ago | stderr contains staleness warning |
| Corrupted meta | Write invalid JSON to `cache-meta.json` | stderr contains migration warning |

Existing tests unchanged:

- `thread_read_cmd_test.go` — tests flag contracts and nil-client errors, not warm behavior
- `message_read_cmd_test.go` — same
- `internal/cache/*_test.go` — cache data layer untouched

## Files Changed

| File | Action | What |
|------|--------|------|
| `internal/override/resolve_cmd.go` | Modify | Delete `ensureCacheReady`, add `warnIfCacheNotReady`, update 3 call sites |
| `internal/override/thread_read_cmd.go` | Modify | Replace `ensureCacheReady` call with `warnIfCacheNotReady` |
| `internal/override/message_read_cmd.go` | Modify | Replace `ensureCacheReady` call with `warnIfCacheNotReady` |
| `internal/cache/cache.go` | Modify | Update `StaleDuration` comment |
| `internal/override/warn_test.go` | New | 4 table-driven tests for the warning function |

5 files touched, 1 new. No changes to the cache data layer. No changes to `cache warm`, `cache info`, or `cache clear`.

## Future Work (Iteration 2)

When the cache is empty, instead of failing with "not found", fall back to live Slack API lookups. Warn the user that this is token-inefficient and they should run `cache warm`. This is a separate design.

## Review Notes

Design reviewed against `/golang-design-patterns`, `/golang-data-structures`, and `/golang-error-handling` skills. Key findings addressed:

1. **Critical:** `EnsureReady` makes API calls in v0/v1 enrichment paths — fixed by passing `nil` fetcher
2. **Important:** Discarding `EnsureReady` error masks corruption — fixed by logging to stderr
3. **Important:** `IsStale` error discard safe by accident — documented with inline comment
4. **Suggestion:** ANSI escapes break on non-TTY — removed, plain text warnings
5. **Suggestion:** `client` parameter unused after fix 1 — removed from signature
6. **Suggestion:** `StaleDuration` threshold — kept at 24h, deliberate choice documented
7. **Nit:** Missing test for migration error — added 4th test case
8. **Nit (out of scope):** `unwrapPathError` should use `errors.As` — separate cleanup
