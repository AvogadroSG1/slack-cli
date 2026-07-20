package override

import (
	"errors"
	"fmt"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

type threadReadDependencies struct {
	client          threadClient
	warnCache       func(*cobra.Command)
	loadIDToNameMap func() (map[string]string, error)
	wait            threadWaitFunc
}

func newThreadReadCmd(client *slack.Client) *cobra.Command {
	var threadAPI threadClient
	if client != nil {
		threadAPI = client
	}
	return newThreadReadCmdWithDependencies(threadReadDependencies{
		client:          threadAPI,
		warnCache:       warnIfCacheNotReady,
		loadIDToNameMap: cache.LoadIDToNameMap,
		wait:            waitForRetry,
	})
}

func newThreadReadCmdWithDependencies(dependencies threadReadDependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread-read [permalink]",
		Short: "Read a complete Slack thread with reactions as text or JSON",
		Long: `Read a Slack thread from its parent through its final reply.

Pass one Slack permalink directly, use --url, or use --channel together with --ts.
All cursor pages are retrieved by default; --all is accepted but redundant.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThreadRead(cmd, args, dependencies)
		},
	}
	cmd.Flags().String("url", "", "Slack thread permalink (alternative to positional permalink)")
	cmd.Flags().String("channel", "", "Conversation ID (requires --ts)")
	cmd.Flags().String("ts", "", "Parent thread timestamp (requires --channel)")
	cmd.Flags().String("oldest", "", "Exclude messages before this Slack timestamp")
	cmd.Flags().String("latest", "", "Exclude messages after this Slack timestamp")
	cmd.Flags().Bool("inclusive", false, "Include messages matching --oldest or --latest")
	cmd.Flags().Bool("include-all-metadata", false, "Request metadata and include it in JSON when present")
	cmd.Flags().Bool("json", false, "Output the stable JSON message array")
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return formatAndExit(cmd, err, exitcode.InputError)
	})
	return cmd
}

func runThreadRead(
	cmd *cobra.Command,
	args []string,
	dependencies threadReadDependencies,
) error {
	rawURL, _ := cmd.Flags().GetString("url")
	channel, _ := cmd.Flags().GetString("channel")
	ts, _ := cmd.Flags().GetString("ts")
	reference, err := resolveThreadReference(args, rawURL, channel, ts)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	options := threadFetchOptions{}
	options.Cursor, _ = cmd.Flags().GetString("cursor")
	options.Oldest, _ = cmd.Flags().GetString("oldest")
	options.Latest, _ = cmd.Flags().GetString("latest")
	options.Inclusive, _ = cmd.Flags().GetBool("inclusive")
	options.Limit, _ = cmd.Flags().GetInt("limit")
	options.MaxResults, _ = cmd.Flags().GetInt("max-results")
	options.IncludeAllMetadata, _ = cmd.Flags().GetBool("include-all-metadata")
	options.WaitOnRateLimit, _ = cmd.Flags().GetBool("wait-on-rate-limit")
	if err := validateThreadFilters(
		options.Oldest,
		options.Latest,
		options.Limit,
		options.MaxResults,
	); err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	if dependencies.client == nil {
		return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
	}
	dependencies.warnCache(cmd)
	idMap, _ := dependencies.loadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	result, err := fetchThread(
		cmd.Context(),
		dependencies.client,
		reference,
		options,
		dependencies.wait,
	)
	if err != nil {
		return formatAndExit(cmd, err, classifyThreadReadError(err))
	}
	if len(result.Messages) == 0 {
		return formatAndExit(
			cmd,
			fmt.Errorf("no thread found in %s at %s", reference.ConversationID, reference.ThreadTS),
			exitcode.InputError,
		)
	}

	asJSON, _ := cmd.Flags().GetBool("json")
	messages := normalizeThreadMessages(result.Messages, idMap, options.IncludeAllMetadata)
	if err := formatThreadMessages(messages, asJSON, cmd.OutOrStdout()); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	if !result.Complete {
		if err := writeThreadIncompleteStatus(
			cmd.ErrOrStderr(),
			asJSON,
			result.NextCursor,
		); err != nil {
			return formatAndExit(cmd, err, exitcode.NetError)
		}
	}
	return nil
}

func classifyThreadReadError(err error) int {
	if errors.Is(err, errRepeatedThreadCursor) {
		return exitcode.APIError
	}
	return exitcode.Classify(err)
}
