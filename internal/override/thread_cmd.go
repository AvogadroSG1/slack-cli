package override

import (
	"fmt"
	"io"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/poconnor/slack-cli/internal/validate"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// newThreadCmd builds the semantic "thread" command: fetch a full discussion
// thread from a permalink or a channel + timestamp pair.
func newThreadCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread <permalink | channel ts>",
		Short: "Read a full Slack thread with its replies",
		Long: `Fetch an entire thread given a message permalink, or a channel
(name, #name, or ID) plus the thread timestamp. Replies are indented under
the root message; all pages of long threads are fetched.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runThread(cmd, client, args)
		},
	}

	addOutputFlags(cmd)
	return cmd
}

func runThread(cmd *cobra.Command, client repliesFetcher, args []string) error {
	opts := getOutputOpts(cmd)
	warnIfCacheNotReady(cmd)

	var channel, ts string
	if len(args) == 1 {
		var err error
		channel, ts, err = parseSlackURL(args[0])
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
	} else {
		var err error
		channel, err = resolveChannelArg(args[0])
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
		if err := validate.Timestamp(args[1]); err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
		ts = args[1]
	}

	msgs, err := fetchThreadMessages(cmd.Context(), client, channel, ts)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}
	if len(msgs) == 0 {
		return formatAndExit(cmd, fmt.Errorf("no thread found in %s at %s", channel, ts), exitcode.InputError)
	}

	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}
	rows := toSemMessages(msgs, "", idMap)

	err = renderOutput(cmd.OutOrStdout(), opts, rows, func(w io.Writer) error {
		return formatSemMessagesText(rows, true, opts.Plain, idMap, w)
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}
