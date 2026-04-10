# slack-cli

Command-line interface for the Slack Web API, built in Go with Cobra.

## Build

```bash
make build          # compile to bin/slack-cli (injects version via ldflags)
make test           # run tests with -race -count=1
make lint           # golangci-lint
make clean          # remove bin/
make e2e            # end-to-end tests (placeholder)
```

## Architecture

```
cmd/slack-cli/      Entrypoint: signal handling, root Cobra command, global flags
internal/slackapi/  (planned) Slack API client wrapper with pagination, rate-limit backoff
internal/output/    (planned) JSON/table formatting, --pretty support
```

Key patterns:

- **Context propagation**: `signal.NotifyContext` in main, passed via `root.ExecuteContext(ctx)` to all subcommands.
- **Global flags**: `--pretty`, `--all`, `--limit`, `--cursor`, `--timeout`, `--debug`, `--wait-on-rate-limit`, `--max-results` on the root command's PersistentFlags.
- **Exit codes**: 0 = success, 3 = error.
- **Version injection**: `version`, `commit`, `date` set via `-ldflags` at build time.

## Testing

- Standard `testing` package + `github.com/google/go-cmp/cmp` for comparisons.
- No assertion libraries (testify, etc.).
- Table-driven tests with `t.Run` subtests.
- Run: `make test`

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/slack-go/slack` | Slack Web API client |
| `github.com/google/go-cmp` | Test comparisons |
| `golang.org/x/tools/go/packages` | Code generation tooling |
