package override

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/poconnor/slack-cli/internal/cache"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// searchPageLimit is the per-request result count for search pagination.
const searchPageLimit = 100

// searchClient abstracts the Slack search calls so tests can provide a fake.
type searchClient interface {
	SearchMessagesContext(ctx context.Context, query string, params slack.SearchParameters) (*slack.SearchMessages, error)
	SearchFilesContext(ctx context.Context, query string, params slack.SearchParameters) (*slack.SearchFiles, error)
}

// searchFileRow is the JSON row for file search results.
type searchFileRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Filetype  string `json:"filetype"`
	Size      int    `json:"size"`
	Permalink string `json:"permalink"`
	Created   string `json:"created"` // RFC3339, local
}

// newSearchCmd builds the semantic "search" command. The dispatch builder
// attaches the generated search.messages/search.files subcommands to it.
func newSearchCmd(client *slack.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>...",
		Short: "Search messages and files across the workspace",
		Long: `Search messages or files using Slack's query syntax (from:@user,
in:#channel, has:link, …). Query words are joined with spaces, so quoting is
optional. Requires a user token (xoxp-…); bot tokens cannot call search.

Note: a query whose first word matches a subcommand name (messages, files,
all) is dispatched to that subcommand — use a search modifier or quote a
longer phrase to search for those words literally.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if client == nil {
				return formatAndExit(cmd, fmt.Errorf("SLACK_TOKEN is not set"), exitcode.AuthError)
			}
			return runSearch(cmd, client, args)
		},
	}

	cmd.Flags().String("after", "", "Only results after this time (2h, 3d, 2026-07-01, …)")
	cmd.Flags().String("before", "", "Only results before this time (2h, 3d, 2026-07-01, …)")
	cmd.Flags().String("type", "messages", "What to search: messages or files")
	cmd.Flags().String("sort", "score", "Sort order: score or timestamp")
	cmd.Flags().Int("limit", 20, "Maximum results to return")
	addOutputFlags(cmd)

	return cmd
}

func runSearch(cmd *cobra.Command, client searchClient, args []string) error {
	query := strings.Join(args, " ")

	after, _ := cmd.Flags().GetString("after")
	before, _ := cmd.Flags().GetString("before")
	searchType, _ := cmd.Flags().GetString("type")
	sortOrder, _ := cmd.Flags().GetString("sort")
	limit, _ := cmd.Flags().GetInt("limit")
	opts := getOutputOpts(cmd)

	now := time.Now()
	if after != "" {
		date, err := parseTimeSpecAsDate(after, now)
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
		query += " after:" + date
	}
	if before != "" {
		date, err := parseTimeSpecAsDate(before, now)
		if err != nil {
			return formatAndExit(cmd, err, exitcode.InputError)
		}
		query += " before:" + date
	}
	if limit <= 0 {
		return formatAndExit(cmd, fmt.Errorf("--limit must be positive"), exitcode.InputError)
	}
	if searchType != "messages" && searchType != "files" {
		return formatAndExit(cmd, fmt.Errorf("invalid --type %q (valid: messages, files)", searchType), exitcode.InputError)
	}
	if sortOrder != "score" && sortOrder != "timestamp" {
		return formatAndExit(cmd, fmt.Errorf("invalid --sort %q (valid: score, timestamp)", sortOrder), exitcode.InputError)
	}

	warnIfCacheNotReady(cmd)
	idMap, _ := cache.LoadIDToNameMap()
	if idMap == nil {
		idMap = map[string]string{}
	}

	if searchType == "files" {
		return runSearchFiles(cmd, client, query, sortOrder, limit, opts)
	}
	return runSearchMessages(cmd, client, query, sortOrder, limit, opts, idMap)
}

func runSearchMessages(cmd *cobra.Command, client searchClient, query, sortOrder string, limit int, opts outputOpts, idMap map[string]string) error {
	var rows []semMessage
	total := 0
	page := 1
	for len(rows) < limit {
		count := searchPageLimit
		if remaining := limit - len(rows); remaining < count {
			count = remaining
		}
		resp, err := client.SearchMessagesContext(cmd.Context(), query, slack.SearchParameters{
			Sort:          sortOrder,
			SortDirection: "desc",
			Count:         count,
			Page:          page,
		})
		if err != nil {
			return formatAndExit(cmd, searchErr(err), searchErrCode(err))
		}
		total = resp.Total
		for _, m := range resp.Matches {
			rows = append(rows, semMessage{
				Channel:   "#" + m.Channel.Name,
				User:      searchUserName(m, idMap),
				Ts:        m.Timestamp,
				Time:      parseSlackTimestamp(m.Timestamp).Format(time.RFC3339),
				Text:      m.Text,
				Permalink: m.Permalink,
			})
		}
		if page >= resp.Pagination.PageCount || len(resp.Matches) == 0 {
			break
		}
		page++
	}

	err := renderOutput(cmd.OutOrStdout(), opts, rows, func(w io.Writer) error {
		for _, r := range rows {
			text := r.Text
			if opts.Plain {
				text = stripMrkdwn(text, idMap)
			}
			timeStr := ""
			if t, terr := time.Parse(time.RFC3339, r.Time); terr == nil {
				timeStr = t.Local().Format("2006-01-02 15:04")
			}
			if _, werr := fmt.Fprintf(w, "%s  %s [%s]: %s\n", r.Channel, r.User, timeStr, text); werr != nil {
				return werr
			}
		}
		return nil
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	if !opts.JSON && opts.Template == "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Showing %d of %d results\n", len(rows), total)
	}
	return nil
}

func runSearchFiles(cmd *cobra.Command, client searchClient, query, sortOrder string, limit int, opts outputOpts) error {
	var rows []searchFileRow
	total := 0
	page := 1
	for len(rows) < limit {
		count := searchPageLimit
		if remaining := limit - len(rows); remaining < count {
			count = remaining
		}
		resp, err := client.SearchFilesContext(cmd.Context(), query, slack.SearchParameters{
			Sort:          sortOrder,
			SortDirection: "desc",
			Count:         count,
			Page:          page,
		})
		if err != nil {
			return formatAndExit(cmd, searchErr(err), searchErrCode(err))
		}
		total = resp.Total
		for _, f := range resp.Matches {
			rows = append(rows, searchFileRow{
				ID:        f.ID,
				Name:      f.Name,
				Filetype:  f.Filetype,
				Size:      f.Size,
				Permalink: f.Permalink,
				Created:   f.Created.Time().Format(time.RFC3339),
			})
		}
		if page >= resp.Pagination.PageCount || len(resp.Matches) == 0 {
			break
		}
		page++
	}

	err := renderOutput(cmd.OutOrStdout(), opts, rows, func(w io.Writer) error {
		for _, r := range rows {
			if _, werr := fmt.Fprintf(w, "%s  %s  %d bytes  %s\n", r.Name, r.Filetype, r.Size, r.Permalink); werr != nil {
				return werr
			}
		}
		return nil
	})
	if err != nil {
		return formatAndExit(cmd, err, exitcode.NetError)
	}
	if !opts.JSON && opts.Template == "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Showing %d of %d results\n", len(rows), total)
	}
	return nil
}

// searchUserName picks the best display name for a search match.
func searchUserName(m slack.SearchMessage, idMap map[string]string) string {
	if name, ok := idMap[m.User]; ok {
		return name
	}
	if m.Username != "" {
		return m.Username
	}
	return resolveUser(m.User, "", idMap)
}

// isNotAllowedTokenType reports whether err is Slack's bot-token rejection
// for search endpoints.
func isNotAllowedTokenType(err error) bool {
	var serr slack.SlackErrorResponse
	return errors.As(err, &serr) && serr.Err == "not_allowed_token_type"
}

// searchErr rewrites the bot-token error into an actionable message.
func searchErr(err error) error {
	if isNotAllowedTokenType(err) {
		return fmt.Errorf("search requires a user token (xoxp-…); bot tokens cannot call the search API")
	}
	return err
}

// searchErrCode maps search errors to exit codes.
func searchErrCode(err error) int {
	if isNotAllowedTokenType(err) {
		return exitcode.AuthError
	}
	return exitcode.Classify(err)
}
