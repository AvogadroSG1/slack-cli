package override

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fakeSearchClient returns canned pages of message matches.
type fakeSearchClient struct {
	pages   [][]slack.SearchMessage
	total   int
	err     error
	queries []string
	counts  []int
}

func (f *fakeSearchClient) SearchMessagesContext(ctx context.Context, query string, params slack.SearchParameters) (*slack.SearchMessages, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.queries = append(f.queries, query)
	f.counts = append(f.counts, params.Count)
	idx := params.Page - 1
	var matches []slack.SearchMessage
	if idx < len(f.pages) {
		matches = f.pages[idx]
	}
	resp := &slack.SearchMessages{Matches: matches, Total: f.total}
	resp.Pagination.PageCount = len(f.pages)
	return resp, nil
}

func (f *fakeSearchClient) SearchFilesContext(ctx context.Context, query string, params slack.SearchParameters) (*slack.SearchFiles, error) {
	return &slack.SearchFiles{}, f.err
}

// execSearch builds the search command around a fake and executes it.
func execSearch(t *testing.T, client searchClient, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Setenv("SLACK_CLI_CACHE_DIR", t.TempDir())

	cmd := &cobra.Command{Use: "search", Args: cobra.MinimumNArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runSearch(cmd, client, a)
	}
	cmd.Flags().String("after", "", "")
	cmd.Flags().String("before", "", "")
	cmd.Flags().String("type", "messages", "")
	cmd.Flags().String("sort", "score", "")
	cmd.Flags().Int("limit", 20, "")
	addOutputFlags(cmd)

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func searchMatch(ts, channel, user, text string) slack.SearchMessage {
	return slack.SearchMessage{
		Channel:   slack.CtxChannel{ID: "C01", Name: channel},
		Username:  user,
		Timestamp: ts,
		Text:      text,
		Permalink: "https://x.slack.com/archives/C01/p" + strings.ReplaceAll(ts, ".", ""),
	}
}

func TestSearchTextOutput(t *testing.T) {
	f := &fakeSearchClient{
		pages: [][]slack.SearchMessage{{searchMatch("1775827095.264229", "general", "alice", "hello world")}},
		total: 1,
	}

	stdout, stderr, err := execSearch(t, f, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "#general") || !strings.Contains(stdout, "alice") || !strings.Contains(stdout, "hello world") {
		t.Errorf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "Showing 1 of 1 results") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestSearchJSONOutput(t *testing.T) {
	f := &fakeSearchClient{
		pages: [][]slack.SearchMessage{{searchMatch("1775827095.264229", "general", "alice", "hello")}},
		total: 1,
	}

	stdout, stderr, err := execSearch(t, f, "hello", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"permalink"`) {
		t.Errorf("json missing permalink: %q", stdout)
	}
	if strings.Contains(stderr, "Showing") {
		t.Errorf("json mode should not print the trailer: %q", stderr)
	}
}

func TestSearchPagination(t *testing.T) {
	page1 := make([]slack.SearchMessage, 100)
	for i := range page1 {
		page1[i] = searchMatch("1775827095.264229", "general", "alice", "x")
	}
	page2 := []slack.SearchMessage{
		searchMatch("1775827095.264229", "general", "alice", "y"),
		searchMatch("1775827095.264229", "general", "alice", "z"),
	}
	f := &fakeSearchClient{pages: [][]slack.SearchMessage{page1, page2}, total: 150}

	stdout, _, err := execSearch(t, f, "q", "--limit", "150", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(stdout, `"ts"`); got != 102 {
		t.Errorf("got %d rows, want 102", got)
	}
	if len(f.counts) != 2 || f.counts[0] != 100 || f.counts[1] != 50 {
		t.Errorf("page counts = %v, want [100 50]", f.counts)
	}
}

func TestSearchAppendsTimeModifiers(t *testing.T) {
	f := &fakeSearchClient{pages: [][]slack.SearchMessage{{}}, total: 0}

	_, _, err := execSearch(t, f, "deploy", "--after", "2026-07-01")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.queries) == 0 || !strings.Contains(f.queries[0], "deploy after:2026-07-01") {
		t.Errorf("queries = %v", f.queries)
	}
}

func TestSearchBotTokenError(t *testing.T) {
	f := &fakeSearchClient{err: slack.SlackErrorResponse{Err: "not_allowed_token_type"}}

	_, stderr, err := execSearch(t, f, "q")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(stderr, "user token") {
		t.Errorf("stderr = %q, want user-token hint", stderr)
	}
}

func TestSearchInvalidFlags(t *testing.T) {
	f := &fakeSearchClient{}
	for _, args := range [][]string{
		{"q", "--type", "nope"},
		{"q", "--sort", "nope"},
		{"q", "--after", "garbage"},
		{"q", "--limit", "0"},
		{"q", "--limit", "-5"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			if _, _, err := execSearch(t, f, args...); err == nil {
				t.Error("expected input error")
			}
		})
	}
}

func TestSearchNilClientAuthError(t *testing.T) {
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newSearchCmd(nil))
	var errBuf bytes.Buffer
	root.SetOut(&errBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"search", "q"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for nil client")
	}
	if !strings.Contains(errBuf.String(), "SLACK_TOKEN") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}
