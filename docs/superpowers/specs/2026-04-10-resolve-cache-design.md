# Resolve/Cache Subsystem Design

**Date**: 2026-04-10
**Author**: Peter O'Connor with Claude Code
**Status**: Approved

## Problem Statement

Every Slack CLI operation that targets a channel, user, or usergroup requires an ID (e.g., `C01ABCDEF`). Callers must first look up the ID by name via `conversations list --all | jq`, which makes an API call on every invocation. This adds latency, wastes API quota, and complicates scripting.

## Solution

Add a transparent file-based cache that stores name-to-ID mappings for channels, users, and usergroups. A `resolve` subcommand provides exact-match lookups. The cache warms automatically on first stale access each day and can be warmed explicitly.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Cache location | `~/.slack-cli/cache.json`, override via `SLACK_CLI_CACHE_DIR` | Simple default with escape hatch |
| Entities | Channels + Users + Usergroups | Covers primary use cases; emoji rarely needed |
| Resolution | Exact match only | Predictable for scripts/agents; discovery via `conversations list` |
| Warm strategy | Transparent on first stale run + explicit `cache warm` | Zero friction daily + manual/cron option |
| Integration | Separate `resolve` subcommand | No changes to existing dispatch pipeline |
| Architecture | Single JSON file, file-lock protected | Safe for concurrent access, human-readable, zero dependencies |
| Staleness | File `mtime` | Simpler than an internal timestamp field |

## Cache File Format

Path: `${SLACK_CLI_CACHE_DIR:-~/.slack-cli}/cache.json`

```json
{
  "channels": {
    "general": "C01ABCDEF",
    "platform-engineering": "C02GHIJKL"
  },
  "users": {
    "poconnor": "U03B00M8EKZ",
    "jsmith": "U04XYZABC"
  },
  "usergroups": {
    "platform-team": "S01ABCDEF",
    "oncall-eng": "S02GHIJKL"
  }
}
```

- Channels keyed by `name` field
- Users keyed by `name` field (Slack username, not display name)
- Usergroups keyed by `handle` field
- No metadata fields; staleness determined by file `mtime`

## Package Structure

```
internal/cache/
  cache.go        Cache type, Load, Save, file locking, staleness, path resolution
  warm.go         Warm function (fetches all channels, users, usergroups via Slack client)
  resolve.go      Resolve function (exact match lookup by entity type)
  cache_test.go   Unit tests
```

Follows existing convention: one package per concern under `internal/`.

## Command Structure

Two new top-level command groups registered via the override package:

```
slack-cli cache warm              Full cache warm
slack-cli cache warm --force      Warm even if cache is fresh
slack-cli cache info              Show cache path, mtime, entry counts
slack-cli cache clear             Delete cache file

slack-cli resolve channel <name>          Returns channel ID
slack-cli resolve user <name>             Returns user ID
slack-cli resolve usergroup <handle>      Returns usergroup ID
```

Output: bare ID string to stdout (e.g., `C01ABCDEF`), suitable for `$(...)` substitution.

## Data Flow

### Resolve

```
slack-cli resolve channel general
  1. Stat cache file
     - Missing or mtime > 24h: trigger warm (step 2a)
     - Fresh: skip to step 3
  2a. Warm: acquire LOCK_EX, fetch all entities, write cache atomically, release lock
  3. Acquire LOCK_SH on cache file
  4. Read + deserialize cache.json
  5. Release LOCK_SH
  6. Exact match: channels["general"]
     - Hit: print ID to stdout, exit 0
     - Miss: print error to stderr, exit 3
```

### Warm

```
slack-cli cache warm
  1. Acquire LOCK_EX on lock file
  2. Fetch conversations.list --all (paginated, all types)
  3. Fetch users.list --all (paginated)
  4. Fetch usergroups.list
  5. Build name-to-ID maps
  6. Write to temp file, rename to cache.json (atomic)
  7. Release LOCK_EX
  8. Print summary to stdout
```

## Integration Boundaries

- **No changes to existing packages**: dispatch, builder, registry, validate, exitcode are untouched
- **New commands registered via override pattern**: same as `api list`
- **Warm uses the Slack client directly**: same `*slack.Client` created in `main.go`
- **Resolve does not call Slack API**: reads only from cache file

## File Locking

- Lock file: `${cacheDir}/cache.lock` (separate from data file for atomic rename safety)
- Warm: exclusive lock (`LOCK_EX`) with 10s timeout
- Read: shared lock (`LOCK_SH`) with 10s timeout
- Implementation: `syscall.Flock` (macOS/Linux)

## Error Handling

| Scenario | Behavior | Exit Code |
|----------|----------|-----------|
| Resolve hit | Print bare ID to stdout | 0 (OK) |
| Resolve miss, fresh cache | `no channel named "foo"` to stderr | 3 (InputError) |
| Warm fails, no token | Auth error to stderr | 2 (AuthError) |
| Warm fails, network | Network error to stderr | 4 (NetError) |
| Cache file corrupt | Delete and re-warm | 0 if re-warm succeeds |
| Lock timeout (10s) | Timeout error to stderr | 4 (NetError) |

Error output uses the existing `FormatError` JSON envelope to stderr.

## Staleness Policy

- Cache is stale if `mtime` is older than 24 hours
- `resolve` auto-warms on stale cache
- `cache warm` always warms regardless of staleness
- `cache warm --force` is an alias for clarity (same behavior as `cache warm`)
- No incremental updates: warm replaces the entire cache

## Testing Strategy

- **Unit tests** (`cache_test.go`):
  - Load/Save round-trip with temp files
  - Staleness detection (mock mtime)
  - Resolve: hit, miss, empty cache, each entity type
  - Path resolution: default, env override, directory creation
  - Corrupt cache handling
- **Warm tests** (`warm_test.go`):
  - Mock Slack client (interface extraction)
  - Verify cache contents after warm
  - Pagination handling
  - Error propagation
- **E2E tests** (requires `SLACK_TOKEN`):
  - `slack-cli cache warm` succeeds
  - `slack-cli resolve channel general` returns valid ID
  - `slack-cli resolve user poconnor` returns valid ID
- **Conventions**: table-driven tests with `t.Run`, `go-cmp/cmp` for comparisons, `testing` stdlib only (no testify)

## Usage After Implementation

```bash
# Old pattern (2 API calls, jq dependency)
CHANNEL_ID=$(slack-cli conversations list --all | jq -r '.[] | select(.name=="general") | .id')
slack-cli chat post-message --channel "$CHANNEL_ID" --text "Hello"

# New pattern (0-1 API calls, no jq)
slack-cli chat post-message --channel "$(slack-cli resolve channel general)" --text "Hello"
```

## Skill Update

After implementation, update `~/.claude/skills/slack-cli/SKILL.md` to:
- Document `resolve` and `cache` commands
- Replace the "Find Channel by Name" pattern with `resolve`
- Add cache management to troubleshooting section
