package override

import (
	"fmt"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// newThreadReadCmd builds the "thread-read" command that fetches a full Slack
// thread and outputs it as name-resolved plain text or JSON.
func newThreadReadCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread-read",
		Short: "Read a Slack thread as name-resolved plain text or JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runThreadRead(cmd, client)
		},
	}

	cmd.Flags().String("url", "", "Slack thread URL (e.g. https://…/archives/CXXX/pYYY)")
	cmd.Flags().String("channel", "", "Channel ID (alternative to --url)")
	cmd.Flags().String("ts", "", "Thread timestamp, e.g. 1775827095.264229 (alternative to --url)")
	cmd.Flags().Bool("json", false, "Output as JSON array")

	cmd.MarkFlagsMutuallyExclusive("url", "channel")
	cmd.MarkFlagsMutuallyExclusive("url", "ts")
	cmd.MarkFlagsRequiredTogether("channel", "ts")

	return cmd
}

func runThreadRead(cmd *cobra.Command, client *slack.Client) error {
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

	if err := ensureCacheReady(cmd, client); err != nil {
		return err
	}

	// Load the full id→name map once; fall back to raw IDs on error.
	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	params := &slack.GetConversationRepliesParameters{
		ChannelID: channel,
		Timestamp: ts,
	}
	msgs, _, _, err := client.GetConversationRepliesContext(cmd.Context(), params)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	if len(msgs) == 0 {
		return formatAndExit(cmd,
			fmt.Errorf("no thread found in %s at %s", channel, ts),
			exitcode.InputError)
	}

	readMsgs := make([]readMessage, 0, len(msgs))
	for _, msg := range msgs {
		readMsgs = append(readMsgs, readMessage{
			User: resolveUser(msg.User, msg.BotID, idMap),
			Time: parseSlackTimestamp(msg.Timestamp),
			Text: msg.Text,
		})
	}

	if err := formatMessages(readMsgs, asJSON, cmd.OutOrStdout()); err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}
