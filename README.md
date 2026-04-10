# slack-cli

Go CLI for the Slack Web API, JSON-first for agents, `--pretty` for humans.

## Installation

```bash
# From source
go install github.com/poconnor/slack-cli/cmd/slack-cli@latest

# Or clone and build
git clone https://github.com/poconnor/slack-cli.git
cd slack-cli
make build
# Binary lands in bin/slack-cli
```

## Quick Start

```bash
# Set your Slack token (Bot or User token)
export SLACK_TOKEN="<your-slack-bot-or-user-token>"

# List channels
slack-cli conversations list --limit 5

# Post a message
slack-cli chat post-message --channel C01ABCDEF --text "Hello from slack-cli"

# Look up a user
slack-cli users info --user U01ABCDEF
```

## Authentication

`slack-cli` reads the Slack Bot or User token from the `SLACK_TOKEN` environment variable.

```bash
# Direct export
export SLACK_TOKEN="<your-token>"

# Pipe from a secret manager
export SLACK_TOKEN="$(vault kv get -field=token secret/slack)"
```

If `SLACK_TOKEN` is not set, every command exits with code 2 and a JSON error on stderr.

## Command Structure

```
slack-cli <category> <action> [flags]
```

Categories are derived from the Slack API namespace (e.g., `chat`, `conversations`, `users`). Each action maps to a specific API method. The full list is generated automatically from the `slack-go/slack` SDK.

## Discovery

```bash
# Top-level help
slack-cli --help

# Category help
slack-cli chat --help

# List every registered API method (one per line)
slack-cli api list

# Formatted table with descriptions
slack-cli api list --pretty

# Filter by category
slack-cli api list --category conversations
```

## Global Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--pretty` | bool | `false` | Pretty-print output (tables for maps, indented JSON for lists) |
| `--all` | bool | `false` | Fetch all pages of results automatically |
| `--limit` | int | `0` | Maximum items per API request (0 uses API default) |
| `--cursor` | string | `""` | Pagination cursor for resuming a previous request |
| `--timeout` | duration | `30s` | HTTP timeout for API calls |
| `--debug` | bool | `false` | Enable debug logging to stderr |
| `--max-results` | int | `10000` | Maximum total results when using `--all` |
| `--wait-on-rate-limit` | bool | `false` | Wait and retry when rate-limited instead of failing |

## Output

JSON is the default output format, designed for piping to `jq` or consumption by agents.

```bash
# Default: indented JSON to stdout
slack-cli conversations list --limit 3

# Pretty: aligned key-value tables for maps
slack-cli users info --user U01ABCDEF --pretty

# Pipe to jq
slack-cli conversations list --limit 10 | jq '.[].name'
```

Errors are always written to stderr as a JSON envelope:

```json
{"ok": false, "error": "not_authed", "exit_code": 2}
```

## Pagination

Paginated methods support cursor-based traversal. Use `--all` to follow pages automatically.

```bash
# Single page (default)
slack-cli conversations list --limit 20

# Fetch all pages, stop at 500 results
slack-cli conversations list --all --max-results 500

# Resume from a cursor
slack-cli conversations list --cursor "dGVhbTpDMD..."
```

- `--all` enables automatic page traversal.
- `--limit` controls items per API request.
- `--max-results` caps the total number of results when using `--all` (default 10000).
- `--cursor` resumes pagination from a specific point.

Context cancellation (Ctrl-C) between pages returns a partial result set with `"partial": true` and the `next_cursor` for resumption.

## Error Handling

All errors are written to stderr as JSON. The process exit code indicates the error category:

| Exit Code | Constant | Meaning |
|-----------|----------|---------|
| 0 | `OK` | Success |
| 1 | `APIError` | Slack API returned an error or rate limit |
| 2 | `AuthError` | Authentication failure (invalid/missing/revoked token) |
| 3 | `InputError` | Invalid input (missing required flags, bad values) |
| 4 | `NetError` | Network error, timeout, or context cancellation |

```bash
slack-cli conversations list
echo $?  # 0 on success, non-zero on failure
```

## Development

```bash
make generate    # Run go generate (introspects slack-go SDK, emits registry)
make build       # Compile to bin/slack-cli with version ldflags
make test        # Run tests with -race -count=1
make lint        # Run golangci-lint
make e2e         # Run end-to-end tests
make clean       # Remove bin/
```

### Project Layout

```
cmd/slack-cli/         Entrypoint, root command, global flags, signal handling
cmd/introspect/        Code generator that introspects slack-go/slack SDK
internal/registry/     MethodDef and ParamDef types, global Registry
internal/dispatch/     Command builder, executor, output formatting, pagination
internal/override/     Hand-written command overrides (api list, etc.)
internal/exitcode/     Exit code constants and error classifier
internal/validate/     Input validation helpers
```

## Shell Completion

```bash
# Zsh
eval "$(slack-cli completion zsh)"

# Bash
eval "$(slack-cli completion bash)"

# Fish
slack-cli completion fish | source
```

Add the appropriate line to your shell profile for persistent completion.

## License

See [LICENSE](LICENSE) for details.
