# slack-cli

CLI for the Slack Web API. 73 methods auto-generated from SDK introspection, plus hand-written semantic commands (search, thread, read, tail, unread, inspect, channels, users, summarize, export, download). Built with Go + Cobra.

## Commands

```bash
make build          # compile to bin/slack-cli (version via ldflags)
make test           # go test -race -count=1 ./...
make lint           # golangci-lint run ./...
make generate       # regenerate registry from SDK introspection
```

## Architecture

```
cmd/slack-cli/        Entrypoint: signal handling, root command, global flags
cmd/introspect/       Code generator: reflects on slack-go SDK → generated registry
internal/registry/    MethodDef types + generated.go (73 methods, go:generate)
internal/dispatch/    Command builder, executor, pagination, output formatting
internal/dispatch/impl_*.go   Per-category dispatch functions (chat, users, files, etc.)
internal/override/    Hand-written commands: builtins + semantic commands + shared helpers
                      (resolver, timeparse, output, fetch)
internal/cache/       File-based name/ID cache (channels, people, usergroups, id-to-name)
internal/validate/    Input validation (channel IDs, user IDs, timestamps, JSON)
internal/exitcode/    Exit code classification from Slack API errors
```

## Key Patterns

- **Code generation**: `cmd/introspect` reflects on `slack-go` types → `internal/registry/generated.go`
- **Dispatch pipeline**: registry → builder → executor → output (flag extraction, pagination, JSON formatting)
- **Override mechanism**: two kinds of hand-written commands in `internal/override/`:
  - `override.Register(apiMethod, cmd)` — replaces a generated command for a specific API method
  - `RegisterBuiltins(root, client)` — adds top-level commands (`cache`, `resolve`, `thread-read`, `message-read`, `api`) and the semantic commands (`search`, `thread`, `read`, `tail`, `unread`, `inspect`, `channels`, `users`, `summarize`, `export`, `download`)
- **Category merge**: the dispatch builder attaches generated method subcommands to a pre-registered root command of the same name (`dispatch.categoryParent`) — this is how the semantic `search`/`users` commands coexist with the generated `search.*`/`users.*` children
- **Output convention**: generated commands emit JSON (machine-first, unchanged); semantic commands are human-first — readable text by default with shared `--json`/`--plain`/`--template` flags (`addOutputFlags`/`renderOutput` in `internal/override/output.go`). Semantic commands register a *local* `--limit` flag that shadows the global persistent one
- **Semantic command helpers** in `internal/override/`: `resolveChannelArg`/`resolveUserArg`/`resolveTarget` (name-or-ID → ID via cache), `parseTimeSpec` (2h/3d/1w/dates → Slack ts), `fetchChannelMessages`/`fetchThreadMessages` (cursor pagination behind narrow fetcher interfaces for testability), `stripMrkdwn`
- **Cache**: file-based store in `~/.slack-cli/` at v3; `id-to-name.json` is a reverse index used by `thread-read`/`message-read` for user ID → display name resolution
- **Exit codes**: 0=OK, 1=API error, 2=auth error, 3=input error, 4=network error
- **Auth**: `SLACK_TOKEN` env var required; nil client fails at invocation, not startup. `summarize` additionally requires `ANTHROPIC_API_KEY`; `search`/`unread` need a user token (`xoxp-`)

## Testing

- Standard `testing` + `go-cmp/cmp` for comparisons. No testify.
- Table-driven tests with `t.Run` subtests.

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/slack-go/slack` | Slack Web API client |
| `github.com/anthropics/anthropic-sdk-go` | Claude API client (`summarize`) |
| `github.com/google/go-cmp` | Test comparisons |
| `golang.org/x/tools` | SDK introspection for code generation |
