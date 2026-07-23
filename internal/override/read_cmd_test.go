package override

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fakeReadClient wraps fakeFetcher with an IM opener.
type fakeReadClient struct {
	fakeFetcher
	imChannelID string
	openedUsers []string
}

func (f *fakeReadClient) OpenConversationContext(ctx context.Context, params *slack.OpenConversationParameters) (*slack.Channel, bool, bool, error) {
	f.openedUsers = append(f.openedUsers, params.Users...)
	ch := &slack.Channel{}
	ch.ID = f.imChannelID
	return ch, false, false, nil
}

// execRead builds the read command around a fake and executes it.
func execRead(t *testing.T, client readClient, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "read", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runRead(cmd, client, a[0])
	}
	cmd.Flags().String("since", "", "")
	cmd.Flags().Int("limit", 50, "")
	cmd.Flags().Bool("include-threads", false, "")
	addOutputFlags(cmd)

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestReadChannelByName(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{fakeFetcher: fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{msg("2.0", "second"), msg("1.0", "first")}},
	}}}

	stdout, _, err := execRead(t, f, "#general")
	if err != nil {
		t.Fatal(err)
	}
	first := strings.Index(stdout, "first")
	second := strings.Index(stdout, "second")
	if first == -1 || second == -1 || first > second {
		t.Errorf("expected chronological order, got %q", stdout)
	}
}

func TestReadUserOpensIM(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{
		imChannelID: "D0AAAAAAAAA",
		fakeFetcher: fakeFetcher{historyPages: []historyPage{
			{messages: []slack.Message{msg("1.0", "dm hello")}},
		}},
	}

	stdout, _, err := execRead(t, f, "@alice")
	if err != nil {
		t.Fatal(err)
	}
	if len(f.openedUsers) != 1 || f.openedUsers[0] != "U0AAAAAAAAA" {
		t.Errorf("opened users = %v", f.openedUsers)
	}
	if !strings.Contains(stdout, "dm hello") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestReadUsergroupRejected(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{}
	_, stderr, err := execRead(t, f, "oncall")
	if err == nil {
		t.Fatal("expected error for usergroup target")
	}
	if !strings.Contains(stderr, "usergroup") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestReadSinceInvalid(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{}
	if _, _, err := execRead(t, f, "#general", "--since", "garbage"); err == nil {
		t.Error("expected input error for bad --since")
	}
}

func TestReadEmptyChannelExitsZero(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{}
	stdout, _, err := execRead(t, f, "#general")
	if err != nil {
		t.Fatalf("empty channel should not error: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestReadJSON(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{fakeFetcher: fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{msg("1.0", "hello")}},
	}}}

	stdout, _, err := execRead(t, f, "#general", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"text": "hello"`) || !strings.Contains(stdout, `"channel": "#general"`) {
		t.Errorf("json = %q", stdout)
	}
}
