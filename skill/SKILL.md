---
name: slack-cli
description: Interact with Slack via the slack-cli command-line tool. Use for posting messages, searching channels, fetching history, managing users/channels, and all Slack Web API operations. Replaces Slack MCP tools. Triggers on Slack operations, channel lookups, message posting, or any mcp__slack__* tool usage.
license: MIT
user-invocable: true
metadata:
  version: 1.1.0
  author: Peter O'Connor
  domains:
    - slack
    - messaging
    - chat
---

# Slack CLI

Command-line interface for the Slack Web API. Use `slack-cli` via Bash for all Slack operations instead of MCP tools.

## Quick Start

```bash
# Verify authentication
slack-cli auth test

# List channels
slack-cli conversations list --limit 10

# Post a message
slack-cli chat post-message --channel C01ABCDEF --text "Hello from Claude"

# Search messages
slack-cli search messages --query "deploy failed"
```

## When to Use This Skill

| Use This Skill | Instead Of |
|----------------|------------|
| Any Slack operation | `mcp__slack__*` tools |
| Posting messages | `mcp__slack__conversations_add_message` |
| Listing channels | `mcp__slack__channels_list` |
| Fetching history | `mcp__slack__conversations_history` |
| Searching messages | `mcp__slack__conversations_search_messages` |
| Thread replies | `mcp__slack__conversations_replies` |

**Always use `slack-cli` via Bash.** The MCP tools are deprecated.

## Quick Reference

| Task | Command |
|------|---------|
| **Read a full thread (human-readable)** | `slack-cli thread-read --url "https://...slack.com/archives/CXXX/pYYY"` |
| **Read a single message (human-readable)** | `slack-cli message-read --url "https://...slack.com/archives/CXXX/pYYY"` |
| Resolve channel name to ID | `slack-cli resolve channel general` |
| Resolve user name to ID | `slack-cli resolve user poconnor` |
| Post message (by name) | `slack-cli chat post-message --channel "$(slack-cli resolve channel general)" --text "msg"` |
| Thread reply | `slack-cli chat post-message --channel C01... --text "msg" --thread-ts 1234...` |
| List channels | `slack-cli conversations list --limit 20` |
| Get history | `slack-cli conversations history --channel C01... --limit 50` |
| Search messages | `slack-cli search messages --query "keyword"` |
| User info | `slack-cli users info --user U01...` |
| Add reaction | `slack-cli reactions add --channel C01... --timestamp 1234... --name thumbsup` |
| Warm cache | `slack-cli cache warm` |
| Cache status | `slack-cli cache info` |

## Authentication

The CLI requires `SLACK_TOKEN` environment variable. This is already configured in the shell environment.

```bash
# Verify token is working
slack-cli auth test
```

## Output Formats

| Format | Flag | Use Case |
|--------|------|----------|
| JSON (default) | none | Parsing with `jq`, programmatic use |
| Pretty tables | `--pretty` | Human readability |

```bash
# JSON output (default) - pipe to jq
slack-cli conversations list --limit 5 | jq '.[].name'

# Pretty output for review
slack-cli users info --user U01ABCDEF --pretty
```

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--pretty` | false | Pretty-print output |
| `--all` | false | Fetch all pages automatically |
| `--limit` | 0 | Items per API request (0 = API default) |
| `--cursor` | "" | Pagination cursor |
| `--timeout` | 30s | HTTP timeout |
| `--debug` | false | Debug logging to stderr |
| `--max-results` | 10000 | Max results with `--all` |
| `--wait-on-rate-limit` | false | Retry on rate limit |

## Exit Codes

| Code | Meaning | Action |
|------|---------|--------|
| 0 | Success | Continue |
| 1 | Slack API error | Check error message, may need different params |
| 2 | Auth failure | Token invalid or missing scopes |
| 3 | Invalid input | Missing required flag or malformed value |
| 4 | Network error | Timeout or connectivity issue |

```bash
slack-cli conversations list
if [ $? -eq 2 ]; then
  echo "Authentication failed - check SLACK_TOKEN"
fi
```

## Name Resolution (resolve)

Channels, users, and usergroups require IDs. Use `resolve` to look up IDs by name:

```bash
# Resolve channel name to ID
slack-cli resolve channel general           # prints: C01ABCDEF

# Resolve user name to ID
slack-cli resolve user poconnor             # prints: U03B00M8EKZ

# Resolve usergroup handle to ID
slack-cli resolve usergroup platform-team   # prints: S01ABCDEF
```

### Enriched Lookups (--field)

People and usergroups have enriched data. Use `--field` to access specific fields:

```bash
# People fields: id, email, display_name, title, all
slack-cli resolve user poconnor --field email         # prints email
slack-cli resolve user poconnor --field display_name  # prints display name
slack-cli resolve user poconnor --field title          # prints job title
slack-cli resolve user poconnor --field all            # prints full JSON entry

# Usergroup fields: id, description, members, all
slack-cli resolve usergroup platform-team --field members      # JSON array of member IDs
slack-cli resolve usergroup platform-team --field description  # prints description
slack-cli resolve usergroup platform-team --field all          # prints full JSON entry
```

### Cache Management

The cache warms automatically on first use each day. Four separate files for fast lookups:

```bash
slack-cli cache warm     # Fetch all channels, people, usergroups (also builds id-to-name index)
slack-cli cache info     # Show cache version, status, and counts
slack-cli cache clear    # Delete all cache files
```

Sample `cache info` output:
```
Path: /Users/peter/.slack-cli
Version: 3
Status: fresh
Channels: 342
People: 187
Usergroups: 12
ID mappings: 187
```

Cache location: `~/.slack-cli/` (override with `SLACK_CLI_CACHE_DIR` env var).

| File | Contents |
|------|----------|
| `channels.json` | Channel name → ID |
| `people.json` | Slack username → enriched user data (ID, email, display name, title) |
| `usergroups.json` | Usergroup handle → enriched data |
| `id-to-name.json` | User ID → display name (reverse index, used by thread-read/message-read) |

**Auto-migrations (no manual steps required):**
- v1 → v2: splits `cache.json` into separate files and enriches with profile data
- v2 → v3: derives `id-to-name.json` from existing `people.json` — zero API calls

## Reading Threads and Messages

These commands return name-resolved, human-readable output in one call — no jq pipelines, no separate user-info calls, no manual pagination.

### thread-read

Reads a full Slack thread (root + all replies) in chronological order. One API call.

```bash
# From a Slack URL (copy link from Slack)
slack-cli thread-read --url "https://stackexchange.slack.com/archives/C0AFM69EB1B/p1775827095264229"

# From explicit channel + timestamp
slack-cli thread-read --channel C0AFM69EB1B --ts 1775827095.264229

# JSON output (RFC3339 timestamps, array of objects)
slack-cli thread-read --url "..." --json
```

**Default output:**
```
Peter O'Connor [2026-04-10 09:18]: This ODR affects y'all...
Brendan Rosage [2026-04-10 09:32]: This doc provides context...
Alex Lato [2026-04-10 09:35]: This is explicitly something we need to rethink...
```

**JSON output:**
```json
[
  {"user": "Peter O'Connor", "ts": "2026-04-10T09:18:15-05:00", "text": "This ODR affects y'all..."},
  {"user": "Brendan Rosage", "ts": "2026-04-10T09:32:41-05:00", "text": "This doc provides context..."}
]
```

Timestamps are shown in local time (default) or RFC3339 with offset (`--json`). Bot messages display as `[bot]`. Unresolved user IDs fall back to the raw ID.

### message-read

Reads a single top-level channel message (thread root or standalone). Does not surface thread replies — use `thread-read` for those.

```bash
# From a Slack URL
slack-cli message-read --url "https://stackexchange.slack.com/archives/D09C0KHRF9B/p1776101206614149"

# From explicit channel + timestamp
slack-cli message-read --channel D09C0KHRF9B --ts 1776101206.614149

# JSON output
slack-cli message-read --url "..." --json
```

### Flag Contract (both commands)

| Flags | Behavior |
|-------|----------|
| `--url` | Parse channel and timestamp from a Slack URL |
| `--channel` + `--ts` | Provide channel ID and timestamp directly |
| `--url` + `--channel` | Error — mutually exclusive |
| `--channel` without `--ts` | Error — must be provided together |

### Name Resolution

Both commands resolve user IDs to display names automatically using the local cache (`~/.slack-cli/id-to-name.json`). The cache is warmed on first use each day. If a user ID is not in the cache, the raw ID is shown as a fallback.

## Common Patterns

### Post to Channel by Name

```bash
# Use resolve for name-to-ID lookup (preferred)
slack-cli chat post-message --channel "$(slack-cli resolve channel platform-engineering)" --text "Deployment complete"

# Or if you already know the ID
slack-cli chat post-message --channel C01ABCDEF --text "Deployment complete"
```

### Thread Reply

```bash
# Reply to a specific message (use parent message timestamp)
slack-cli chat post-message \
  --channel C01ABCDEF \
  --text "This is a threaded reply" \
  --thread-ts 1234567890.123456
```

### Broadcast Thread Reply

```bash
# Reply to thread AND post to channel
slack-cli chat post-message \
  --channel C01ABCDEF \
  --text "Important update" \
  --thread-ts 1234567890.123456 \
  --reply-broadcast
```

### Fetch All Channel Members

```bash
# Automatic pagination
slack-cli conversations members --channel C01ABCDEF --all

# With limit
slack-cli conversations members --channel C01ABCDEF --all --max-results 500
```

### Search and Extract

```bash
# Search messages and extract links
slack-cli search messages --query "production incident" | jq -r '.messages.matches[].permalink'

# Search in specific channel (use Slack search syntax)
slack-cli search messages --query "in:#platform-engineering deploy"
```

### Get Recent Messages

```bash
# Last 10 messages in a channel
slack-cli conversations history --channel C01ABCDEF --limit 10

# Messages in time range (Unix timestamps)
slack-cli conversations history --channel C01ABCDEF --oldest 1704067200 --latest 1704153600
```

### Look Up User

```bash
# By user ID
slack-cli users info --user U01ABCDEF --pretty

# List all users (paginated)
slack-cli users list --all --max-results 1000
```

### Add Reactions

```bash
# React to a message
slack-cli reactions add --channel C01ABCDEF --timestamp 1234567890.123456 --name thumbsup

# Multiple reactions
slack-cli reactions add --channel C01ABCDEF --timestamp 1234567890.123456 --name rocket
slack-cli reactions add --channel C01ABCDEF --timestamp 1234567890.123456 --name eyes
```

## API Categories

The CLI covers 73 Slack API methods across these categories:

| Category | Methods | Common Use |
|----------|---------|------------|
| chat | 6 | Post, update, delete messages |
| conversations | 18 | Channels, history, members |
| users | 5 | User info, presence |
| search | 2 | Search messages and files |
| reactions | 4 | Add/remove reactions |
| files | 5 | File operations |
| usergroups | 6 | User groups management |
| pins | 3 | Pin messages |
| bookmarks | 4 | Channel bookmarks |
| stars | 3 | Starred items |
| reminders | 2 | Reminders |
| views | 4 | Modal views |
| dnd | 4 | Do not disturb |
| team | 1 | Team info |
| emoji | 1 | Custom emoji list |

See [references/api-categories.md](references/api-categories.md) for the complete method reference.

## Discovering Commands

```bash
# List all available methods
slack-cli api list

# Filter by category
slack-cli api list --category conversations

# Get help for specific command
slack-cli conversations history --help
```

## Anti-Patterns

| Avoid | Why | Instead |
|-------|-----|---------|
| Using `mcp__slack__*` tools | Deprecated, limited functionality | Use `slack-cli` via Bash |
| Piping `conversations list` through `jq` for ID lookup | Slow, makes API call every time | Use `slack-cli resolve channel <name>` |
| Hardcoding channel IDs | Brittle, not self-documenting | Use `$(slack-cli resolve channel <name>)` |
| Fetching all without `--max-results` | May hit rate limits | Set reasonable `--max-results` |
| Ignoring exit codes | Miss API errors silently | Check `$?` after calls |
| Multiple calls for pagination | Inefficient | Use `--all` flag |

## Integration with Other Skills

| Skill | Integration |
|-------|-------------|
| `morning-slack-broadcast` | Uses slack-cli for posting |
| `public-recognition-broadcast` | Uses slack-cli for posting |
| `peter-copy-editor` | Draft messages, then post via slack-cli |

## Verification

After Slack operations, verify:

- [ ] Exit code is 0 (check `$?`)
- [ ] JSON output is valid (if piping to `jq`)
- [ ] Message posted to correct channel
- [ ] Thread timestamp correct for replies

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "missing required scope" | Token needs additional OAuth scopes |
| "channel_not_found" | Channel ID wrong or bot not in channel |
| "not_in_channel" | Invite bot to private channel first |
| Exit code 4 | Network issue, check connectivity |
| Rate limited | Use `--wait-on-rate-limit` or reduce request frequency |
| `resolve` returns "no channel named..." | Channel may be new; run `slack-cli cache warm` to refresh |
| Cache stale / slow first resolve of day | Normal - auto-warm fetches all entities on first use |
| Override cache location | Set `SLACK_CLI_CACHE_DIR` env var |
