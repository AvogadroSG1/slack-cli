package override

import (
	"fmt"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/dispatch"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// newResolveCmd builds the "resolve" parent command with channel, user,
// and usergroup subcommands.
func newResolveCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve a Slack name to its ID using the local cache",
	}

	cmd.AddCommand(newResolveChannelCmd(client))
	cmd.AddCommand(newResolveUserCmd(client))
	cmd.AddCommand(newResolveUsergroupCmd(client))

	return cmd
}

func newResolveChannelCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channel <name>",
		Short: "Resolve a channel name to its Slack ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			warnIfCacheNotReady(cmd)

			field, _ := cmd.Flags().GetString("field")

			lock, err := cache.AcquireShared()
			if err != nil {
				return formatAndExit(cmd, err, exitcode.NetError)
			}
			defer lock.Close()

			id, err := cache.ResolveChannel(args[0])
			if err != nil {
				return formatAndExit(cmd, err, exitcode.InputError)
			}

			if field == "all" {
				fmt.Fprintf(cmd.OutOrStdout(), "{\n  \"id\": %q\n}\n", id)
			} else if field != "" && field != "id" {
				return formatAndExit(cmd,
					fmt.Errorf("unknown field %q for channel (valid: id, all)", field),
					exitcode.InputError)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), id)
			}
			return nil
		},
	}
	cmd.Flags().String("field", "", "Field to return (id, all)")
	return cmd
}

func newResolveUserCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user <name>",
		Short: "Resolve a user name to its Slack ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			warnIfCacheNotReady(cmd)

			field, _ := cmd.Flags().GetString("field")

			lock, err := cache.AcquireShared()
			if err != nil {
				return formatAndExit(cmd, err, exitcode.NetError)
			}
			defer lock.Close()

			result, err := cache.ResolveUser(args[0], field)
			if err != nil {
				return formatAndExit(cmd, err, exitcode.InputError)
			}

			fmt.Fprintln(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().String("field", "", "Field to return (id, email, display_name, title, all)")
	return cmd
}

func newResolveUsergroupCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usergroup <handle>",
		Short: "Resolve a usergroup handle to its Slack ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			warnIfCacheNotReady(cmd)

			field, _ := cmd.Flags().GetString("field")

			lock, err := cache.AcquireShared()
			if err != nil {
				return formatAndExit(cmd, err, exitcode.NetError)
			}
			defer lock.Close()

			result, err := cache.ResolveUsergroup(args[0], field)
			if err != nil {
				return formatAndExit(cmd, err, exitcode.InputError)
			}

			fmt.Fprintln(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().String("field", "", "Field to return (id, description, members, all)")
	return cmd
}

// warnIfCacheNotReady runs local-only migrations and prints a warning
// to stderr if the cache is stale or empty. It never blocks or errors.
func warnIfCacheNotReady(cmd *cobra.Command) {
	// Run local-only migrations (pass nil fetcher to guarantee no API calls).
	_, err := cache.EnsureReady(cmd.Context(), nil)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: cache migration failed: %v\n", err)
		return
	}

	// IsStale returns true on error, so discarding err still produces a correct warning.
	stale, _ := cache.IsStale()
	if stale {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Warning: cache not warmed. Run \"slack-cli cache warm\" for faster lookups.\n")
	}
}

// formatAndExit writes a JSON error to stderr and returns an exit error.
func formatAndExit(cmd *cobra.Command, err error, code int) error {
	_ = dispatch.FormatError(cmd.ErrOrStderr(), err.Error(), code)
	return dispatch.NewExitError(code)
}
