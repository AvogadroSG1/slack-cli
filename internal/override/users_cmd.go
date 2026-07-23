package override

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// usersLister abstracts the users.list call.
type usersLister interface {
	GetUsersContext(ctx context.Context, options ...slack.GetUsersOption) ([]slack.User, error)
}

// userRow is the JSON row for the semantic users listing.
type userRow struct {
	Username    string `json:"username"`
	ID          string `json:"id"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name,omitempty"`
	Email       string `json:"email,omitempty"`
	Title       string `json:"title,omitempty"`
	Deleted     bool   `json:"deleted"`
	IsBot       bool   `json:"is_bot"`
}

// newUsersCmd builds the semantic "users" listing command. The dispatch
// builder attaches the generated users.* subcommands (list, info, …) to it.
func newUsersCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "Query the user directory with filters",
		Long: `List workspace users. Deleted users and bots are excluded by
default; use --deleted / --bots to include them, --match to filter by
username or real name, and --email to filter by email substring (e.g. a
domain). The raw API subcommands (users list, users info, …) are unchanged.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runUsers(cmd, client)
		},
	}

	cmd.Flags().String("email", "", "Only users whose email contains this substring")
	cmd.Flags().String("match", "", "Only users whose username or real name contains this substring")
	cmd.Flags().Bool("deleted", false, "Include deactivated users")
	cmd.Flags().Bool("bots", false, "Include bot users")
	cmd.Flags().Int("limit", 0, "Maximum users to list (0 = all)")
	addOutputFlags(cmd)

	return cmd
}

func runUsers(cmd *cobra.Command, client usersLister) error {
	email, _ := cmd.Flags().GetString("email")
	match, _ := cmd.Flags().GetString("match")
	includeDeleted, _ := cmd.Flags().GetBool("deleted")
	includeBots, _ := cmd.Flags().GetBool("bots")
	limit, _ := cmd.Flags().GetInt("limit")
	opts := getOutputOpts(cmd)

	users, err := client.GetUsersContext(cmd.Context())
	if err != nil {
		return formatAndExit(cmd, err, exitcode.Classify(err))
	}

	var rows []userRow
	for _, u := range users {
		if u.Deleted && !includeDeleted {
			continue
		}
		if u.IsBot && !includeBots {
			continue
		}
		if match != "" {
			m := strings.ToLower(match)
			if !strings.Contains(strings.ToLower(u.Name), m) && !strings.Contains(strings.ToLower(u.RealName), m) {
				continue
			}
		}
		if email != "" && !strings.Contains(strings.ToLower(u.Profile.Email), strings.ToLower(email)) {
			continue
		}
		rows = append(rows, userRow{
			Username:    u.Name,
			ID:          u.ID,
			RealName:    u.RealName,
			DisplayName: u.Profile.DisplayName,
			Email:       u.Profile.Email,
			Title:       u.Profile.Title,
			Deleted:     u.Deleted,
			IsBot:       u.IsBot,
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}

	err = renderOutput(cmd.OutOrStdout(), opts, rows, func(w io.Writer) error {
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, r := range rows {
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Username, r.ID, r.RealName, r.Title); err != nil {
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
