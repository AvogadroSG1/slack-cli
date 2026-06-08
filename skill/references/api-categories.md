# Slack CLI API Reference

Complete reference for all 73 Slack API methods available in `slack-cli`, plus top-level builtin commands.

## Top-Level Builtin Commands

These commands are not part of the 73 generated API methods. They are hand-written commands that provide high-level, agent-friendly UX.

### thread-read

Reads a full Slack thread as name-resolved plain text or JSON. One API call.

```bash
# From URL (preferred)
slack-cli thread-read --url "https://stackexchange.slack.com/archives/C0AFM69EB1B/p1775827095264229"

# From explicit channel + timestamp
slack-cli thread-read --channel C0AFM69EB1B --ts 1775827095.264229

# JSON output (RFC3339 timestamps)
slack-cli thread-read --url "..." --json
```

**Flags:** `--url`, `--channel`, `--ts` (mutually exclusive with `--url`), `--json`

**Output:** `Name [YYYY-MM-DD HH:MM]: text` per message in local time. Bot messages show as `[bot]`. Unresolved user IDs fall back to raw ID.

### message-read

Reads a single top-level channel message (thread root or standalone). Does **not** surface thread replies — use `thread-read` for those.

```bash
# From URL
slack-cli message-read --url "https://stackexchange.slack.com/archives/D09C0KHRF9B/p1776101206614149"

# From explicit channel + timestamp
slack-cli message-read --channel D09C0KHRF9B --ts 1776101206.614149

# JSON output
slack-cli message-read --url "..." --json
```

**Flags:** `--url`, `--channel`, `--ts`, `--json` (same contract as `thread-read`)

### resolve

Resolves Slack names to IDs using the local cache.

```bash
slack-cli resolve channel general          # → C01ABCDEF
slack-cli resolve user poconnor            # → U03B00M8EKZ
slack-cli resolve usergroup platform-team  # → S01ABCDEF
slack-cli resolve user poconnor --field email
```

### cache

Manages the local name/ID cache at `~/.slack-cli/` (v3 format).

```bash
slack-cli cache warm   # Fetch all channels, people, usergroups; build id-to-name index
slack-cli cache info   # Show version, status, Channels/People/Usergroups/ID mappings counts
slack-cli cache clear  # Delete all cache files
```

### api

Discovers available generated API methods.

```bash
slack-cli api list                        # All methods
slack-cli api list --category chat        # Filter by category
slack-cli api list --json                 # JSON output
```

## auth

| Command | Description |
|---------|-------------|
| `auth test` | Test authentication and get identity |

```bash
slack-cli auth test
# Returns: user_id, team_id, user name, team name
```

## bookmarks

| Command | Description |
|---------|-------------|
| `bookmarks add` | Add a bookmark to a channel |
| `bookmarks edit` | Edit an existing bookmark |
| `bookmarks list` | List bookmarks in a channel |
| `bookmarks remove` | Remove a bookmark |

```bash
# List bookmarks
slack-cli bookmarks list --channel C01ABCDEF

# Add bookmark
slack-cli bookmarks add --channel C01ABCDEF --title "Wiki" --link "https://wiki.example.com"
```

## bots

| Command | Description |
|---------|-------------|
| `bots info` | Get info about a bot |

```bash
slack-cli bots info --bot B01ABCDEF
```

## chat

| Command | Description |
|---------|-------------|
| `chat delete` | Delete a message |
| `chat getPermalink` | Get permalink to a message |
| `chat postEphemeral` | Post ephemeral message (visible only to one user) |
| `chat postMessage` | Post a message to a channel |
| `chat scheduleMessage` | Schedule a message for later |
| `chat update` | Update an existing message |

### chat post-message

```bash
# Basic message
slack-cli chat post-message --channel C01ABCDEF --text "Hello world"

# With formatting
slack-cli chat post-message --channel C01ABCDEF --text "*Bold* and _italic_"

# Thread reply
slack-cli chat post-message --channel C01ABCDEF --text "Reply" --thread-ts 1234567890.123456

# Broadcast reply (thread + channel)
slack-cli chat post-message --channel C01ABCDEF --text "Important" --thread-ts 1234... --reply-broadcast

# Custom username/emoji
slack-cli chat post-message --channel C01ABCDEF --text "Deploy bot" --username "DeployBot" --icon-emoji ":rocket:"

# With blocks (JSON)
slack-cli chat post-message --channel C01ABCDEF --blocks '[{"type":"section","text":{"type":"mrkdwn","text":"*Header*"}}]'
```

**Flags:**
- `--channel` (required): Channel ID
- `--text`: Message text
- `--thread-ts`: Parent message timestamp for threading
- `--reply-broadcast`: Also post to channel when replying to thread
- `--blocks`: JSON Block Kit blocks
- `--username`: Custom username
- `--icon-emoji`: Custom emoji (e.g., `:robot:`)
- `--icon-url`: Custom icon URL
- `--unfurl-links`: Unfurl URLs
- `--unfurl-media`: Unfurl media

### chat update

```bash
slack-cli chat update --channel C01ABCDEF --ts 1234567890.123456 --text "Updated message"
```

### chat delete

```bash
slack-cli chat delete --channel C01ABCDEF --ts 1234567890.123456
```

### chat getPermalink

```bash
slack-cli chat get-permalink --channel C01ABCDEF --ts 1234567890.123456
# Returns: {"permalink": "https://team.slack.com/archives/..."}
```

### chat postEphemeral

```bash
slack-cli chat post-ephemeral --channel C01ABCDEF --user U01ABCDEF --text "Only you can see this"
```

### chat scheduleMessage

```bash
# post-at is Unix timestamp
slack-cli chat schedule-message --channel C01ABCDEF --text "Future message" --post-at 1704153600
```

## conversations

| Command | Description |
|---------|-------------|
| `conversations archive` | Archive a channel |
| `conversations close` | Close a DM/group |
| `conversations create` | Create a channel |
| `conversations history` | Get messages from a channel |
| `conversations info` | Get channel info |
| `conversations invite` | Invite users to a channel |
| `conversations join` | Join a channel |
| `conversations kick` | Remove a user from a channel |
| `conversations leave` | Leave a channel |
| `conversations list` | List channels |
| `conversations mark` | Mark channel as read |
| `conversations members` | List channel members |
| `conversations open` | Open/create a DM |
| `conversations rename` | Rename a channel |
| `conversations replies` | Get thread replies |
| `conversations setPurpose` | Set channel purpose |
| `conversations setTopic` | Set channel topic |
| `conversations unarchive` | Unarchive a channel |

### conversations list

```bash
# Basic list
slack-cli conversations list --limit 20

# All channels (paginated automatically)
slack-cli conversations list --all

# Filter by type
slack-cli conversations list --types public_channel,private_channel

# Exclude archived
slack-cli conversations list --exclude-archived

# Get channel names only
slack-cli conversations list --all | jq -r '.[].name'

# Find channel by name
slack-cli conversations list --all | jq -r '.[] | select(.name=="general") | .id'
```

**Flags:**
- `--limit`: Items per page
- `--cursor`: Pagination cursor
- `--types`: Channel types (public_channel, private_channel, mpim, im)
- `--exclude-archived`: Exclude archived channels
- `--team-id`: Team ID for Enterprise Grid

### conversations history

```bash
# Recent messages
slack-cli conversations history --channel C01ABCDEF --limit 50

# All messages (paginated)
slack-cli conversations history --channel C01ABCDEF --all --max-results 1000

# Time range (Unix timestamps)
slack-cli conversations history --channel C01ABCDEF --oldest 1704067200 --latest 1704153600

# Include message metadata
slack-cli conversations history --channel C01ABCDEF --include-all-metadata
```

**Flags:**
- `--channel` (required): Channel ID
- `--limit`: Messages per page
- `--oldest`: Start of time range (Unix timestamp)
- `--latest`: End of time range (Unix timestamp)
- `--inclusive`: Include boundary timestamps
- `--include-all-metadata`: Include all message metadata

### conversations replies

```bash
# Get all replies in a thread
slack-cli conversations replies --channel C01ABCDEF --ts 1234567890.123456

# All replies (paginated)
slack-cli conversations replies --channel C01ABCDEF --ts 1234567890.123456 --all
```

### conversations info

```bash
slack-cli conversations info --channel C01ABCDEF
slack-cli conversations info --channel C01ABCDEF --include-num-members
```

### conversations members

```bash
# Get all members
slack-cli conversations members --channel C01ABCDEF --all

# With limit
slack-cli conversations members --channel C01ABCDEF --limit 100
```

### conversations create

```bash
# Public channel
slack-cli conversations create --name new-channel

# Private channel
slack-cli conversations create --name private-channel --is-private
```

### conversations invite/kick

```bash
# Invite users (comma-separated)
slack-cli conversations invite --channel C01ABCDEF --users U01,U02,U03

# Remove user
slack-cli conversations kick --channel C01ABCDEF --user U01ABCDEF
```

### conversations join/leave

```bash
slack-cli conversations join --channel C01ABCDEF
slack-cli conversations leave --channel C01ABCDEF
```

### conversations rename

```bash
slack-cli conversations rename --channel C01ABCDEF --name new-name
```

### conversations setPurpose/setTopic

```bash
slack-cli conversations set-purpose --channel C01ABCDEF --purpose "Channel for deployments"
slack-cli conversations set-topic --channel C01ABCDEF --topic "Current sprint: 2024-Q1"
```

### conversations archive/unarchive

```bash
slack-cli conversations archive --channel C01ABCDEF
slack-cli conversations unarchive --channel C01ABCDEF
```

### conversations mark

```bash
# Mark as read up to timestamp
slack-cli conversations mark --channel C01ABCDEF --ts 1234567890.123456
```

### conversations open/close

```bash
# Open DM with user
slack-cli conversations open --users U01ABCDEF

# Open group DM
slack-cli conversations open --users U01,U02,U03

# Close DM
slack-cli conversations close --channel D01ABCDEF
```

## dialog

| Command | Description |
|---------|-------------|
| `dialog open` | Open a dialog |

```bash
# Dialog JSON required
slack-cli dialog open --trigger-id T01... --dialog '{"title":"Form",...}'
```

## dnd

| Command | Description |
|---------|-------------|
| `dnd endDnd` | End Do Not Disturb |
| `dnd endSnooze` | End snooze |
| `dnd info` | Get DND status |
| `dnd setSnooze` | Set snooze |
| `dnd teamInfo` | Get team DND info |

```bash
# Check DND status
slack-cli dnd info --user U01ABCDEF

# Snooze for 60 minutes
slack-cli dnd set-snooze --num-minutes 60

# End snooze
slack-cli dnd end-snooze
```

## emoji

| Command | Description |
|---------|-------------|
| `emoji list` | List custom emoji |

```bash
slack-cli emoji list
# Returns map of emoji name -> URL
```

## files

| Command | Description |
|---------|-------------|
| `files delete` | Delete a file |
| `files info` | Get file info |
| `files list` | List files |
| `files revokePublicURL` | Revoke public URL |
| `files sharedPublicURL` | Create public URL |

```bash
# List files
slack-cli files list --limit 20

# List files by user
slack-cli files list --user U01ABCDEF

# List files in channel
slack-cli files list --channel C01ABCDEF

# Get file info
slack-cli files info --file F01ABCDEF

# Delete file
slack-cli files delete --file F01ABCDEF
```

## pins

| Command | Description |
|---------|-------------|
| `pins add` | Pin a message |
| `pins list` | List pinned items |
| `pins remove` | Unpin a message |

```bash
# Pin a message
slack-cli pins add --channel C01ABCDEF --timestamp 1234567890.123456

# List pins
slack-cli pins list --channel C01ABCDEF

# Unpin
slack-cli pins remove --channel C01ABCDEF --timestamp 1234567890.123456
```

## reactions

| Command | Description |
|---------|-------------|
| `reactions add` | Add reaction |
| `reactions get` | Get reactions on a message |
| `reactions list` | List reactions by user |
| `reactions remove` | Remove reaction |

```bash
# Add reaction (no colons)
slack-cli reactions add --channel C01ABCDEF --timestamp 1234... --name thumbsup

# Multiple reactions
slack-cli reactions add --channel C01ABCDEF --timestamp 1234... --name rocket
slack-cli reactions add --channel C01ABCDEF --timestamp 1234... --name eyes

# Get reactions on a message
slack-cli reactions get --channel C01ABCDEF --timestamp 1234...

# Remove reaction
slack-cli reactions remove --channel C01ABCDEF --timestamp 1234... --name thumbsup
```

## reminders

| Command | Description |
|---------|-------------|
| `reminders delete` | Delete a reminder |
| `reminders list` | List reminders |

```bash
slack-cli reminders list
slack-cli reminders delete --reminder R01ABCDEF
```

## search

| Command | Description |
|---------|-------------|
| `search files` | Search files |
| `search messages` | Search messages |

### search messages

```bash
# Basic search
slack-cli search messages --query "deploy failed"

# Search in channel (use Slack search syntax)
slack-cli search messages --query "in:#platform-engineering error"

# Search from user
slack-cli search messages --query "from:@peter bug"

# Search with date
slack-cli search messages --query "after:2024-01-01 incident"

# Extract permalinks
slack-cli search messages --query "deploy" | jq -r '.messages.matches[].permalink'
```

### search files

```bash
slack-cli search files --query "architecture diagram"
```

## stars

| Command | Description |
|---------|-------------|
| `stars add` | Star an item |
| `stars list` | List starred items |
| `stars remove` | Unstar an item |

```bash
# Star a message
slack-cli stars add --channel C01ABCDEF --timestamp 1234...

# List stars
slack-cli stars list

# Unstar
slack-cli stars remove --channel C01ABCDEF --timestamp 1234...
```

## team

| Command | Description |
|---------|-------------|
| `team info` | Get team info |

```bash
slack-cli team info
```

## usergroups

| Command | Description |
|---------|-------------|
| `usergroups create` | Create a user group |
| `usergroups disable` | Disable a user group |
| `usergroups enable` | Enable a user group |
| `usergroups list` | List user groups |
| `usergroups update` | Update a user group |
| `usergroups users list` | List users in a group |
| `usergroups users update` | Update users in a group |

```bash
# List user groups
slack-cli usergroups list

# List users in a group
slack-cli usergroups users list --usergroup S01ABCDEF

# Create user group
slack-cli usergroups create --name "Platform Team" --handle platform-team

# Update members
slack-cli usergroups users update --usergroup S01ABCDEF --users U01,U02,U03
```

## users

| Command | Description |
|---------|-------------|
| `users getPresence` | Get user presence |
| `users info` | Get user info |
| `users list` | List all users |
| `users profile get` | Get user profile |
| `users setPresence` | Set your presence |

```bash
# Get user info
slack-cli users info --user U01ABCDEF
slack-cli users info --user U01ABCDEF --pretty

# List all users (paginated)
slack-cli users list --all

# Get user presence
slack-cli users get-presence --user U01ABCDEF

# Get profile
slack-cli users profile get --user U01ABCDEF

# Set your presence
slack-cli users set-presence --presence auto  # or "away"
```

## views

| Command | Description |
|---------|-------------|
| `views open` | Open a modal |
| `views publish` | Publish App Home view |
| `views push` | Push a view onto stack |
| `views update` | Update a view |

```bash
# These require JSON view payloads
slack-cli views open --trigger-id T01... --view '{"type":"modal",...}'
slack-cli views update --view-id V01... --view '{"type":"modal",...}'
```
