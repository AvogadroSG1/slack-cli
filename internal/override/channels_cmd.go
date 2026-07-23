package override

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// channelRow is the JSON row for the semantic channels listing.
type channelRow struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	IsPrivate  bool   `json:"is_private"`
	IsArchived bool   `json:"is_archived"`
	NumMembers int    `json:"num_members"`
	Topic      string `json:"topic,omitempty"`
	Purpose    string `json:"purpose,omitempty"`
}

// newChannelsCmd builds the semantic "channels" listing command.
func newChannelsCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "channels",
		Short: "List workspace channels with filters",
		Long: `List channels in the workspace. Public channels are listed by
default; add --private for private channels the token can see, --archived to
include archived channels, and --match to filter by name substring.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runChannels(cmd, client)
		},
	}

	cmd.Flags().Bool("private", false, "Include private channels")
	cmd.Flags().Bool("archived", false, "Include archived channels")
	cmd.Flags().String("match", "", "Only channels whose name contains this substring")
	cmd.Flags().Int("limit", 0, "Maximum channels to list (0 = all)")
	addOutputFlags(cmd)

	return cmd
}

func runChannels(cmd *cobra.Command, client cache.SlackFetcher) error {
	private, _ := cmd.Flags().GetBool("private")
	archived, _ := cmd.Flags().GetBool("archived")
	match, _ := cmd.Flags().GetString("match")
	limit, _ := cmd.Flags().GetInt("limit")
	opts := getOutputOpts(cmd)

	types := []string{"public_channel"}
	if private {
		types = append(types, "private_channel")
	}

	var rows []channelRow
	cursor := ""
	for {
		channels, next, err := client.GetConversationsContext(cmd.Context(), &slack.GetConversationsParameters{
			Types:           types,
			ExcludeArchived: !archived,
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
		for _, ch := range channels {
			if match != "" && !strings.Contains(ch.Name, match) {
				continue
			}
			rows = append(rows, channelRow{
				Name:       ch.Name,
				ID:         ch.ID,
				IsPrivate:  ch.IsPrivate,
				IsArchived: ch.IsArchived,
				NumMembers: ch.NumMembers,
				Topic:      ch.Topic.Value,
				Purpose:    ch.Purpose.Value,
			})
			if limit > 0 && len(rows) >= limit {
				break
			}
		}
		cursor = next
		if cursor == "" || (limit > 0 && len(rows) >= limit) {
			break
		}
	}

	err := renderOutput(cmd.OutOrStdout(), opts, rows, func(w io.Writer) error {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, r := range rows {
			flags := ""
			if r.IsPrivate {
				flags += " private"
			}
			if r.IsArchived {
				flags += " archived"
			}
			if _, err := fmt.Fprintf(tw, "#%s\t%s\t%d members%s\t%s\n", r.Name, r.ID, r.NumMembers, flags, r.Topic); err != nil {
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
