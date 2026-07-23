package override

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// unreadClient abstracts the API calls unread needs.
type unreadClient interface {
	AuthTestContext(ctx context.Context) (*slack.AuthTestResponse, error)
	GetConversationsForUserContext(ctx context.Context, params *slack.GetConversationsForUserParameters) ([]slack.Channel, string, error)
	GetConversationInfoContext(ctx context.Context, input *slack.GetConversationInfoInput) (*slack.Channel, error)
}

// unreadRow is the JSON row for the unread listing.
type unreadRow struct {
	Channel       string `json:"channel"`
	ID            string `json:"id"`
	Unread        int    `json:"unread"`
	UnreadDisplay int    `json:"unread_display"`
	LastRead      string `json:"last_read,omitempty"`
}

// newUnreadCmd builds the semantic "unread" command.
func newUnreadCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unread",
		Short: "List conversations with unread activity",
		Long: `List the conversations you are in that have unread messages.

Requires a user token (xoxp-…): Slack only reports unread counts for the
token's own user, and bot tokens always see zero. There is no bulk unread
API, so this makes one conversations.info call per conversation checked —
expect it to take a while in large workspaces (progress is reported on
stderr, and rate limits are waited out). Mention-only filtering is not
available from the Slack API.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runUnread(cmd, client)
		},
	}

	cmd.Flags().String("types", "public_channel,private_channel,im,mpim", "Conversation types to check")
	cmd.Flags().Int("min", 1, "Only show conversations with at least this many unread messages")
	cmd.Flags().Int("limit", 200, "Maximum conversations to check")
	addOutputFlags(cmd)

	return cmd
}

func runUnread(cmd *cobra.Command, client unreadClient) error {
	typesFlag, _ := cmd.Flags().GetString("types")
	minUnread, _ := cmd.Flags().GetInt("min")
	limit, _ := cmd.Flags().GetInt("limit")
	opts := getOutputOpts(cmd)

	if strings.HasPrefix(os.Getenv("SLACK_TOKEN"), "xoxb-") {
		return formatAndExit(cmd,
			fmt.Errorf("unread requires a user token (xoxp-…); unread counts are not visible to bot tokens"),
			exitcode.AuthError)
	}

	auth, err := client.AuthTestContext(cmd.Context())
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	warnIfCacheNotReady(cmd)
	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	// Enumerate the user's conversations.
	var conversations []slack.Channel
	cursor := ""
	for {
		channels, next, err := client.GetConversationsForUserContext(cmd.Context(), &slack.GetConversationsForUserParameters{
			UserID:          auth.UserID,
			Types:           strings.Split(typesFlag, ","),
			ExcludeArchived: true,
			Cursor:          cursor,
			Limit:           historyPageLimit,
		})
		if err != nil {
			if retried, rerr := sleepOnRateLimit(cmd.Context(), err); retried {
				continue
			} else if rerr != nil {
				return formatAndExit(cmd, rerr, exitcode.NetError)
			}
			return formatAndExit(cmd, err, exitcode.Classify(err))
		}
		conversations = append(conversations, channels...)
		cursor = next
		if cursor == "" || (limit > 0 && len(conversations) >= limit) {
			break
		}
	}
	if limit > 0 && len(conversations) > limit {
		conversations = conversations[:limit]
	}

	// Check each conversation's unread count.
	var rows []unreadRow
	for i, ch := range conversations {
		fmt.Fprintf(cmd.ErrOrStderr(), "\rchecked %d/%d…", i+1, len(conversations))
		info, err := unreadInfo(cmd.Context(), client, ch.ID)
		if err != nil {
			fmt.Fprintln(cmd.ErrOrStderr())
			return formatAndExit(cmd, err, exitcode.Classify(err))
		}
		if info.UnreadCountDisplay < minUnread {
			continue
		}
		name := "#" + info.Name
		if info.IsIM {
			name = "@" + resolveUser(info.User, "", idMap)
		}
		rows = append(rows, unreadRow{
			Channel:       name,
			ID:            info.ID,
			Unread:        info.UnreadCount,
			UnreadDisplay: info.UnreadCountDisplay,
			LastRead:      info.LastRead,
		})
	}
	if len(conversations) > 0 {
		fmt.Fprintln(cmd.ErrOrStderr())
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].UnreadDisplay > rows[j].UnreadDisplay })

	err = renderOutput(cmd.OutOrStdout(), opts, rows, func(w io.Writer) error {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, r := range rows {
			if _, err := fmt.Fprintf(tw, "%s\t%d unread\n", r.Channel, r.UnreadDisplay); err != nil {
				return err
			}
		}
		return tw.Flush()
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	return nil
}

// unreadInfo fetches conversation info, waiting out rate limits.
func unreadInfo(ctx context.Context, client unreadClient, channelID string) (*slack.Channel, error) {
	for {
		info, err := client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{ChannelID: channelID})
		if err != nil {
			if retried, rerr := sleepOnRateLimit(ctx, err); retried {
				continue
			} else if rerr != nil {
				return nil, rerr
			}
			return nil, err
		}
		return info, nil
	}
}
