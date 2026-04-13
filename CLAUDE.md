# slack-cli

CLI for the Slack Web API. 73 methods auto-generated from SDK introspection, built with Go + Cobra.

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
internal/override/    Hand-written commands replacing generated ones (e.g., api list)
internal/validate/    Input validation (channel IDs, user IDs, timestamps, JSON)
internal/exitcode/    Exit code classification from Slack API errors
```

## Key Patterns

- **Code generation**: `cmd/introspect` reflects on `slack-go` types → `internal/registry/generated.go`
- **Dispatch pipeline**: registry → builder → executor → output (flag extraction, pagination, JSON formatting)
- **Override mechanism**: hand-written Cobra commands replace generated ones via `override.Register`
- **Exit codes**: 0=OK, 1=API error, 2=auth error, 3=input error, 4=network error
- **Auth**: `SLACK_TOKEN` env var required; nil client fails at invocation, not startup

## Testing

- Standard `testing` + `go-cmp/cmp` for comparisons. No testify.
- Table-driven tests with `t.Run` subtests.

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/slack-go/slack` | Slack Web API client |
| `github.com/google/go-cmp` | Test comparisons |
| `golang.org/x/tools` | SDK introspection for code generation |
