package override

import (
	"fmt"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// newMessageReadCmd builds the "message-read" command that fetches a single
// top-level channel message and outputs it as name-resolved plain text or JSON.
// Note: this command reads channel-timeline messages (thread roots and
// standalone messages) only. Thread replies are not returned by
// conversations.history — use thread-read to read a full thread.
func newMessageReadCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message-read",
		Short: "Read a single Slack channel message as name-resolved plain text or JSON",
		Long: `Read a single top-level Slack message and print it as name-resolved text or JSON.

Reads channel-timeline messages (thread roots and standalone messages).
Thread replies are not surfaced by this command — use thread-read instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMessageRead(cmd, client)
		},
	}

	cmd.Flags().String("url", "", "Slack message URL (e.g. https://…/archives/CXXX/pYYY)")
	cmd.Flags().String("channel", "", "Channel ID (alternative to --url)")
	cmd.Flags().String("ts", "", "Message timestamp, e.g. 1776101206.614149 (alternative to --url)")
	cmd.Flags().Bool("json", false, "Output as JSON array")

	cmd.MarkFlagsMutuallyExclusive("url", "channel")
	cmd.MarkFlagsMutuallyExclusive("url", "ts")
	cmd.MarkFlagsRequiredTogether("channel", "ts")

	return cmd
}

func runMessageRead(cmd *cobra.Command, client *slack.Client) error {
	if client == nil {
		return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
	}

	rawURL, _ := cmd.Flags().GetString("url")
	channelFlag, _ := cmd.Flags().GetString("channel")
	tsFlag, _ := cmd.Flags().GetString("ts")
	asJSON, _ := cmd.Flags().GetBool("json")

	channel, ts, err := resolveChannelTSFromValues(rawURL, channelFlag, tsFlag)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	warnIfCacheNotReady(cmd)

	// Load the full id→name map once; fall back to raw IDs on error.
	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	params := &slack.GetConversationHistoryParameters{
		ChannelID: channel,
		Latest:    ts,
		Oldest:    ts,
		Inclusive: true,
		Limit:     1,
	}
	resp, err := client.GetConversationHistoryContext(cmd.Context(), params)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	if len(resp.Messages) == 0 {
		return formatAndExit(cmd,
			fmt.Errorf("no message found in %s at %s (if this is a thread reply, use thread-read)", channel, ts),
			exitcode.InputError)
	}

	msg := resp.Messages[0]
	readMsgs := []readMessage{
		{
			User: resolveUser(msg.User, msg.BotID, idMap),
			Time: parseSlackTimestamp(msg.Timestamp),
			Text: msg.Text,
		},
	}

	if err := formatMessages(readMsgs, asJSON, cmd.OutOrStdout()); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}
