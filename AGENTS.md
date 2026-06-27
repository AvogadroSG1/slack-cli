# slack-cli

CLI for the Slack Web API. 73 methods across 18 categories, auto-generated from
slack-go SDK introspection, built with Go + Cobra.

> Module path: `github.com/poconnor/slack-cli`. Requires Go 1.26+.
> `CLAUDE.md` is a symlink to this file (`AGENTS.md`) — edit `AGENTS.md`.

## Commands

```bash
make build          # go generate + compile to bin/slack-cli (version via ldflags)
make test           # go test -race -count=1 ./...
make lint           # golangci-lint run ./...
make generate       # go generate ./... — regenerate registry from SDK introspection
make install        # build + copy bin to ~/.local/bin/slack-cli
make e2e            # build + go test -tags e2e -run TestE2E (spawns the built binary)
make clean          # rm -rf bin/
```

## Architecture

```
cmd/slack-cli/        Entrypoint: signal handling, root command, global flags, version cmd
cmd/introspect/       Code generator: reflects on slack-go SDK → registry (main.go + mapping.go)
internal/registry/    MethodDef/ParamDef types + generated.go (73 methods, DO NOT EDIT)
internal/dispatch/    Command builder, executor, pagination, output formatting
internal/dispatch/impl_*.go   Per-category dispatch funcs (chat, users, files, conversations, …)
internal/override/    Hand-written commands: generated overrides + top-level builtins
internal/cache/       File-based name/ID cache (channels, people, usergroups, id-to-name)
internal/validate/    Input validation (channel IDs, user IDs, timestamps, JSON)
internal/exitcode/    Exit code classification from Slack API errors
docs/superpowers/     Design specs and implementation plans (history/rationale, dated)
skill/                Claude skill packaging (SKILL.md + references/)
```

Startup wiring (`cmd/slack-cli/main.go`): build root cmd → create `*slack.Client`
only if `SLACK_TOKEN` is set (nil client otherwise) → `override.RegisterBuiltins`
→ `dispatch.BuildCommandsWithClient(root, registry.Registry, override.Overrides, client, os.Stdout)`
→ `root.ExecuteContext`, exiting with `dispatch.ExitCode(err)`.

## Key Patterns

- **Code generation**: `cmd/introspect` reflects on `slack-go` types →
  `internal/registry/generated.go` (marked `DO NOT EDIT`). Run `make generate`
  after SDK upgrades; never hand-edit `generated.go`.
- **Call styles**: each `MethodDef` has a `CallStyle` — `positional` (simple
  args), `struct` (params packed into an SDK request struct), or `msgoption`
  (slack-go `MsgOption` builders, e.g. chat methods). Dispatch branches on this.
- **Dispatch pipeline**: registry → builder (one Cobra cmd per method, flags from
  `ParamDef`) → executor (invokes SDK method) → output (JSON / pagination).
- **Override mechanism**: two kinds of hand-written commands in `internal/override/`:
  - `override.Register(apiMethod, cmd)` — replaces the generated command for a
    specific API method (`Overrides` map, keyed by e.g. `"chat.postMessage"`).
  - `RegisterBuiltins(root, client)` — adds top-level commands not derived from
    the registry: `api list`, `cache` (`warm`/`info`/`clear`), `resolve`
    (`channel`/`user`/`usergroup`), `thread-read`, `message-read`.
- **Cache**: file-based store in `~/.slack-cli/` (`DefaultDir`), format
  `CurrentVersion = 3`; staleness ~24h, refreshed explicitly via `cache warm`.
  `id-to-name.json` is a reverse index used by `thread-read`/`message-read` to
  resolve user IDs → display names. Migration logic in `internal/cache/migrate.go`.
- **Output**: JSON by default (compact); `--pretty` indents and renders top-level
  maps as aligned key/value columns via `text/tabwriter`. Errors emit a JSON
  object with `ok`, `error`, `exit_code`.
- **Exit codes** (`internal/exitcode`): 0=OK, 1=API error, 2=auth error,
  3=input error, 4=network error.
- **Auth**: `SLACK_TOKEN` env var required; a missing token yields a nil client
  that fails at command invocation, not at startup.

## Global Flags

Persistent flags on the root command (apply to all subcommands):

| Flag | Default | Purpose |
|------|---------|---------|
| `--pretty` | false | Pretty-print JSON output |
| `--all` | false | Auto-fetch all pages of results |
| `--limit` | 0 | Max items per API request (0 = API default) |
| `--cursor` | "" | Pagination cursor to resume a request |
| `--timeout` | 30s | HTTP timeout for API calls |
| `--debug` | false | Debug logging to stderr |
| `--wait-on-rate-limit` | false | Wait + retry on rate limit instead of failing |
| `--max-results` | 10000 | Cap on total results when using `--all` |

## Testing

- Standard library `testing` + `github.com/google/go-cmp/cmp` for comparisons.
  No testify.
- Table-driven tests with `t.Run` subtests are the norm.
- Run with `make test` (race detector, `-count=1` to disable caching).
- End-to-end tests are gated behind the `e2e` build tag (`e2e_test.go`) and run
  the built binary as a subprocess — invoke via `make e2e`.

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/slack-go/slack` | Slack Web API client |
| `github.com/google/go-cmp` | Test comparisons |
| `golang.org/x/tools` | SDK introspection for code generation |

## Conventions

- Keep `internal/registry/generated.go` machine-generated; behavior changes for a
  specific method go through an `override`, not a hand-edit.
- New SDK methods appear automatically after `make generate`; add an `impl_*.go`
  branch or an override only when generic dispatch can't express the call.
- See `CONTRIBUTING.md` for the full contributor workflow and `docs/superpowers/`
  for design specs and implementation plans behind major features.
