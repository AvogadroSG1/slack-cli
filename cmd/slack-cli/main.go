// Package main is the entrypoint for the slack-cli tool.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/poconnor/slack-cli/internal/dispatch"
	"github.com/poconnor/slack-cli/internal/override"
	"github.com/poconnor/slack-cli/internal/registry"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// Build-time variables injected via ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	root := newRootCmd()

	// Create the Slack client only when a token is available. Commands will
	// report an auth error at invocation time if the client is nil.
	var client *slack.Client
	if token := os.Getenv("SLACK_TOKEN"); token != "" {
		client = slack.New(token)
	}

	override.RegisterBuiltins(root)
	dispatch.BuildCommandsWithClient(root, registry.Registry, override.Overrides, client, os.Stdout)

	if err := root.ExecuteContext(ctx); err != nil {
		os.Exit(dispatch.ExitCode(err))
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "slack-cli",
		Short:         "CLI for the Slack Web API",
		Long:          "slack-cli provides a command-line interface for querying and interacting with the Slack Web API.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	pf := root.PersistentFlags()
	pf.Bool("pretty", false, "Pretty-print JSON output")
	pf.Bool("all", false, "Fetch all pages of results automatically")
	pf.Int("limit", 0, "Maximum items per API request (0 uses API default)")
	pf.String("cursor", "", "Pagination cursor for resuming a previous request")
	pf.Duration("timeout", 30*time.Second, "HTTP timeout for API calls")
	pf.Bool("debug", false, "Enable debug logging to stderr")
	pf.Bool("wait-on-rate-limit", false, "Wait and retry when rate-limited instead of failing")
	pf.Int("max-results", 10000, "Maximum total results when using --all")

	root.AddCommand(newVersionCmd())

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "slack-cli %s (commit: %s, built: %s)\n", version, commit, date)
		},
	}
}
