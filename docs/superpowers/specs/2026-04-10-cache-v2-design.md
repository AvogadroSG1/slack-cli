# Cache V2: Split Files + Enriched People and Usergroups

**Date**: 2026-04-10
**Author**: Peter O'Connor with Claude Code
**Status**: Approved
**Reviewer**: Go Expert Agent (code-reviewer)

## Problem Statement

The single `cache.json` file (213KB, 4,848 lines) is read in full for every resolve operation, even when only one entity type is needed. People entries are flat name→ID maps with no profile data, forcing agents to make additional API calls for email, display name, or title. Usergroups lack member lists, preventing "who's in this group?" queries.

## Solution

Split the cache into three entity-specific files, enrich people with profile data and usergroups with member lists, and auto-migrate from v1 without re-fetching channels.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| File split | channels.json, people.json, usergroups.json | Resolve reads only the file it needs |
| People enrichment | email, display_name, title | Covers identity lookup use cases |
| Usergroup enrichment | description, member IDs | Enables "who's in this group?" queries |
| Channel enrichment | None (flat name→ID) | Channels don't need metadata for agent use |
| Migration | Auto on first access | Zero friction, no manual command |
| Version tracking | cache-meta.json with version field | Deterministic, testable, future-proof |
| Staleness | cache-meta.json mtime | Single file to check, written on every warm |
| MemberCount | Method on struct, not stored field | Single source of truth from len(Members) |
| Naming | UserEntry struct, User EntityType | Avoids collision, CLI surface unchanged |
| Generic Load/Save | LoadEntity[T] / SaveEntity[T] | Eliminates per-type duplication |
| Retry helper | withRetry[T] generic | Deduplicates rate-limit retry across fetchers |

## File Layout

```
~/.slack-cli/
├── cache-meta.json    # {"version": 2} - staleness via mtime
├── channels.json      # flat name→ID (ChannelCache)
├── people.json        # enriched name→UserEntry (PeopleCache)
├── usergroups.json    # enriched handle→Usergroup (UsergroupCache)
├── cache.lock         # single lock protecting all files
└── cache.json         # DELETED after v1→v2 migration
```

## Data Types

```go
// cache-meta.json
type CacheMeta struct {
    Version int `json:"version"`
}

// channels.json - flat map, unchanged from v1 shape
type ChannelCache map[string]string

// people.json - enriched entries
type UserEntry struct {
    ID          string `json:"id"`
    Email       string `json:"email"`
    DisplayName string `json:"display_name"`
    Title       string `json:"title"`
}
type PeopleCache map[string]UserEntry

// usergroups.json - enriched entries
type Usergroup struct {
    ID          string   `json:"id"`
    Description string   `json:"description"`
    Members     []string `json:"members"`
}
// MemberCount is a method: func (ug Usergroup) MemberCount() int
type UsergroupCache map[string]Usergroup
```

## Generic Load/Save

```go
func LoadEntity[T any](filename string) (T, error)
func SaveEntity[T any](filename string, data T) error
```

Both resolve the full path via Dir(), acquire the appropriate lock, and handle atomic writes (temp + rename for Save).

## Migration Flow

```
Any cache access (resolve, warm, info)
    │
    ├─ No cache-meta.json?
    │   ├─ cache.json exists? → V1 MIGRATION
    │   │   1. Read cache.json
    │   │   2. Write channels.json from .channels
    │   │   3. Write people.json from .users (flat: name→{id: "..."})
    │   │   4. Write usergroups.json from .usergroups (flat: handle→{id: "..."})
    │   │   5. Write cache-meta.json {"version": 1}
    │   │   6. Delete cache.json
    │   │   7. Proceed to enrichment check
    │   └─ No cache.json? → fresh install, full warm
    │
    ├─ cache-meta.json version 1? → ENRICHMENT PASS
    │   1. Fetch users.list → enrich people.json with email/display_name/title
    │   2. Fetch usergroups.list (include_users) → enrich usergroups.json
    │   3. Channels.json untouched
    │   4. Update cache-meta.json to version 2
    │
    ├─ cache-meta.json version 2? → CURRENT
    │   Check mtime of cache-meta.json for staleness (>24h → warm)
    │
    └─ Continue with resolve/warm/info
```

## Resolve Changes

Default behavior unchanged: `resolve` prints the ID to stdout.

New `--field` flag for enriched entities:

```bash
slack-cli resolve user poconnor                # prints ID (default)
slack-cli resolve user poconnor --field email   # prints email
slack-cli resolve user poconnor --field title   # prints title
slack-cli resolve user poconnor --field all     # prints JSON of full entry

slack-cli resolve usergroup platform-team                # prints ID
slack-cli resolve usergroup platform-team --field members # prints JSON array of member IDs
slack-cli resolve usergroup platform-team --field all     # prints JSON of full entry

slack-cli resolve channel general              # prints ID (no --field, channels are flat)
slack-cli resolve channel general --field all   # prints JSON {"id": "C01..."}
```

`--field` on channels only supports `all` (emits `{"id": "C01..."}`). Unknown fields return exit code 3 with a message listing valid fields for that entity type.

## Warm Changes

`Warm` writes all three files plus updates cache-meta.json mtime. The fetchers now extract enriched fields:

- `fetchAllChannels`: unchanged (returns name→ID)
- `fetchAllUsers`: extracts Name, ID, Email, DisplayName (profile.display_name), Title (profile.title)
- `fetchAllUsergroups`: uses `include_users` option, extracts Handle, ID, Description, Members

Rate-limit retry uses generic `withRetry[T]` helper to eliminate duplication.

## Cache Info Changes

```
$ slack-cli cache info
Path: /Users/poconnor/.slack-cli/
Version: 2
Status: fresh
Channels: 4155
People: 550
Usergroups: 135
```

## File Locking

Single `cache.lock` file protects all data files atomically. No change from v1 - the lock is held during any read or write operation across all files. This invariant is documented in a comment on the lock constant.

## Error Handling

| Scenario | Behavior | Exit Code |
|----------|----------|-----------|
| Resolve hit | Print field value to stdout | 0 |
| Resolve miss | `no channel named "foo"` to stderr | 3 |
| Unknown --field | `unknown field "xyz" for user (valid: id, email, display_name, title, all)` | 3 |
| Migration fails | Error to stderr, old cache.json preserved | 4 |
| Enrichment fails (network) | Leave at version 1, retry next access | 4 |
| --field on flat channel | Only `all` supported, otherwise error | 3 |

## Code Changes Summary

| File | Change |
|------|--------|
| `internal/cache/cache.go` | Replace Data struct with typed caches, generic Load/Save, CacheMeta, migration detection |
| `internal/cache/resolve.go` | Per-entity file loading, --field support |
| `internal/cache/warm.go` | Enriched fetchers, withRetry[T] generic, write three files + meta |
| `internal/cache/migrate.go` | NEW: v1→v2 migration, enrichment pass |
| `internal/cache/cache_test.go` | Updated for new types |
| `internal/cache/resolve_test.go` | --field tests, enriched data |
| `internal/cache/warm_test.go` | Enriched mock data |
| `internal/cache/migrate_test.go` | NEW: migration + enrichment tests |
| `internal/override/resolve_cmd.go` | --field flag, per-entity loading |
| `internal/override/cache_cmd.go` | Updated info output |

## Testing Strategy

- **Migration**: Write v1 cache.json to temp dir, run migration, verify three files + meta
- **Enrichment**: Write version 1 meta + flat files, run enrichment with mock fetcher, verify version 2
- **Load/Save**: Generic round-trip for each entity type
- **Resolve --field**: Each field for people and usergroups, `all` for JSON output, unknown field error
- **Warm**: Mock fetcher returns enriched data, verify file contents
- **Backwards compat**: Flat people/usergroups files still resolve ID correctly
- **Staleness**: Uses cache-meta.json mtime, not individual file mtimes

## Skill Update

After implementation, update `~/.claude/skills/slack-cli/SKILL.md`:
- Document `--field` flag on resolve
- Update people/usergroup resolve examples
- Note cache v2 format in troubleshooting
