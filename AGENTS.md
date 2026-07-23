# slack-cli

CLI for the Slack Web API. 73 methods auto-generated from SDK introspection, built with Go + Cobra. Module path `github.com/poconnor/slack-cli`.

> `CLAUDE.md` is a symlink to this file — edit `AGENTS.md`.

## Commands

```bash
make build          # go generate + compile to bin/slack-cli (version via ldflags)
make test           # go test -race -count=1 ./...
make lint           # golangci-lint run ./...
make generate       # regenerate registry from SDK introspection (go generate ./...)
make install        # build, then cp bin/slack-cli ~/.local/bin/
make e2e            # build, then go test -tags e2e -run TestE2E ./...
make clean          # rm -rf bin/
```

## Architecture

```
cmd/slack-cli/        Entrypoint: signal handling, root command, global flags, client wiring
cmd/introspect/       Code generator: reflects on slack-go SDK → generated registry (main.go + mapping.go)
internal/registry/    MethodDef/ParamDef types + generated.go (73 methods, go:generate)
internal/dispatch/    Command builder, executor, pagination, output formatting, flag extraction
internal/dispatch/impl_*.go   Per-category dispatch functions (chat, users, files, conversations, etc.)
internal/override/    Hand-written commands: generated overrides + top-level builtins
internal/cache/       File-based name/ID cache + warming (channels, people, usergroups, id-to-name)
internal/validate/    Input validation (channel IDs, user IDs, timestamps, JSON)
internal/exitcode/    Exit code classification from Slack API errors
skill/                Claude Code skill (SKILL.md) wrapping the CLI to replace Slack MCP tools
docs/superpowers/     Design specs and implementation plans
```

## Key Patterns

- **Code generation**: `cmd/introspect` reflects on `slack-go` types → `internal/registry/generated.go` (marked `DO NOT EDIT`). Each `MethodDef` has an `APIMethod`, `Category`, `Command`, `SDKMethod`, `CallStyle` (`positional` or `struct`), and `Params`.
- **Dispatch pipeline**: registry → builder → executor → output (flag extraction, pagination, JSON formatting). `dispatch.BuildCommandsWithClient` wires registry entries into Cobra commands.
- **Override mechanism**: two kinds of hand-written commands in `internal/override/`:
  - `override.Register(apiMethod, cmd)` — replaces a generated command for a specific API method (see `override.Overrides`)
  - `RegisterBuiltins(root, client)` — adds top-level commands: `api` (list/describe methods), `cache`, `resolve`, `thread-read`, `message-read`, `install-daemon`
- **Cache**: file-based store in `~/.slack-cli/` at `CurrentVersion = 3`. Separate files (`channels.json`, `people.json`, `usergroups.json`, `id-to-name.json`, `cache-meta.json`) guarded by a single `cache.lock` for atomic reads/writes. `id-to-name.json` is a reverse index used by `thread-read`/`message-read` for user ID → display name resolution. v1 `cache.json` migrates forward.
- **Cache warming**: `cache.Warm` (via `internal/cache/warm.go`) fetches all channels/users/usergroups with rate-limit retries; exposed as `slack-cli cache warm`. `install-daemon` installs a macOS launchd agent to warm hourly (macOS only).
- **Global flags** (persistent, `cmd/slack-cli/main.go`): `--pretty` (pretty-print JSON), `--all` (auto-paginate all pages), `--limit` (items per request, 0 = API default), `--max-results` (cap total with `--all`, default 10000).
- **Exit codes**: 0=OK, 1=API error, 2=auth error, 3=input error, 4=network error (see `internal/exitcode`).
- **Auth**: `SLACK_TOKEN` env var required; client is nil when unset and fails at invocation, not startup.

## Testing

- Standard `testing` + `go-cmp/cmp` for comparisons. No testify.
- Table-driven tests with `t.Run` subtests.
- E2E tests are behind the `e2e` build tag (`e2e_test.go`, run via `make e2e`).

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/slack-go/slack` | Slack Web API client |
| `github.com/google/go-cmp` | Test comparisons |
| `golang.org/x/tools` | SDK introspection for code generation |
