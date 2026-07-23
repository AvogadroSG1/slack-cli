package override

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// readClient is the API surface the read command needs: history/replies
// fetching plus IM opening for @user targets.
type readClient interface {
	historyRepliesFetcher
	OpenConversationContext(ctx context.Context, params *slack.OpenConversationParameters) (*slack.Channel, bool, bool, error)
}

// newReadCmd builds the semantic "read" command: fetch recent messages from
// a channel or direct message.
func newReadCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <channel|@user>",
		Short: "Read recent messages from a channel or DM",
		Long: `Fetch recent messages from a channel (name, #name, or ID) or a
direct message (@user, username, or user ID), oldest first. Use --since for
a time window and --include-threads to expand thread replies inline (one
extra API call per active thread).`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runRead(cmd, client, args[0])
		},
	}

	cmd.Flags().String("since", "", "Only messages after this time (2h, 3d, 2026-07-01, …)")
	cmd.Flags().Int("limit", 50, "Maximum messages to return")
	cmd.Flags().Bool("include-threads", false, "Expand thread replies inline")
	addOutputFlags(cmd)

	return cmd
}

func runRead(cmd *cobra.Command, client readClient, target string) error {
	since, _ := cmd.Flags().GetString("since")
	limit, _ := cmd.Flags().GetInt("limit")
	includeThreads, _ := cmd.Flags().GetBool("include-threads")
	opts := getOutputOpts(cmd)

	warnIfCacheNotReady(cmd)

	channelID, channelName, err := readTargetChannel(cmd.Context(), client, target)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.InputError)
	}

	oldest := ""
	if since != "" {
		oldest, err = parseTimeSpec(since, time.Now())
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
	}

	msgs, err := fetchChannelMessages(cmd.Context(), client, channelID, oldest, limit, includeThreads)
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}
	rows := toSemMessages(msgs, channelName, idMap)

	err = renderOutput(cmd.OutOrStdout(), opts, rows, func(w io.Writer) error {
		return formatSemMessagesText(rows, includeThreads, opts.Plain, idMap, w)
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}

// readTargetChannel resolves a read/tail target to a channel ID. User
// targets are resolved to their IM channel via conversations.open. The
// returned name is "#name" for named channels, "" otherwise.
func readTargetChannel(ctx context.Context, client readClient, target string) (id, name string, err error) {
	targetID, kind, err := resolveTarget(target)
	if err != nil {
		return "", "", err
	}
	switch kind {
	case targetUser:
		ch, _, _, err := client.OpenConversationContext(ctx, &slack.OpenConversationParameters{
			Users:    []string{targetID},
			ReturnIM: true,
		})
		if err != nil {
			return "", "", fmt.Errorf("open DM with %s: %w", target, err)
		}
		return ch.ID, "", nil
	case targetUsergroup:
		return "", "", fmt.Errorf("%q is a usergroup; read needs a channel or user", target)
	default:
		name := ""
		if target != targetID {
			// A name was resolved; report it with the # convention.
			if target[0] == '#' {
				name = target
			} else {
				name = "#" + target
			}
		}
		return targetID, name, nil
	}
}
