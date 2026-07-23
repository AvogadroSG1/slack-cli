package override

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fakeUnreadClient serves canned conversations and per-channel info.
type fakeUnreadClient struct {
	conversations []slack.Channel
	// unread maps channel ID to unread_count_display.
	unread map[string]int
}

func (f *fakeUnreadClient) AuthTestContext(ctx context.Context) (*slack.AuthTestResponse, error) {
	return &slack.AuthTestResponse{UserID: "U0MEMEMEMEM"}, nil
}

func (f *fakeUnreadClient) GetConversationsForUserContext(ctx context.Context, params *slack.GetConversationsForUserParameters) ([]slack.Channel, string, error) {
	return f.conversations, "", nil
}

func (f *fakeUnreadClient) GetConversationInfoContext(ctx context.Context, input *slack.GetConversationInfoInput) (*slack.Channel, error) {
	for _, ch := range f.conversations {
		if ch.ID == input.ChannelID {
			out := ch
			out.UnreadCount = f.unread[ch.ID]
			out.UnreadCountDisplay = f.unread[ch.ID]
			return &out, nil
		}
	}
	return &slack.Channel{}, nil
}

func unreadChannel(id, name string) slack.Channel {
	ch := slack.Channel{}
	ch.ID = id
	ch.Name = name
	return ch
}

func execUnread(t *testing.T, client unreadClient, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	t.Setenv("SLACK_CLI_CACHE_DIR", t.TempDir())

	cmd := &cobra.Command{Use: "unread", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runUnread(cmd, client)
	}
	cmd.Flags().String("types", "public_channel,private_channel,im,mpim", "")
	cmd.Flags().Int("min", 1, "")
	cmd.Flags().Int("limit", 200, "")
	addOutputFlags(cmd)

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestUnreadFiltersAndSorts(t *testing.T) {
	t.Setenv("SLACK_TOKEN", "xoxp-fake")
	f := &fakeUnreadClient{
		conversations: []slack.Channel{
			unreadChannel("C01AAAAAAAA", "quiet"),
			unreadChannel("C02AAAAAAAA", "busy"),
			unreadChannel("C03AAAAAAAA", "busier"),
		},
		unread: map[string]int{"C02AAAAAAAA": 3, "C03AAAAAAAA": 9},
	}

	stdout, _, err := execUnread(t, f)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "quiet") {
		t.Errorf("zero-unread channel listed: %q", stdout)
	}
	busier := strings.Index(stdout, "#busier")
	busy := strings.Index(stdout, "#busy ")
	if busier == -1 || busy == -1 || busier > busy {
		t.Errorf("expected busier before busy (sorted desc): %q", stdout)
	}
}

func TestUnreadMinFlag(t *testing.T) {
	t.Setenv("SLACK_TOKEN", "xoxp-fake")
	f := &fakeUnreadClient{
		conversations: []slack.Channel{
			unreadChannel("C02AAAAAAAA", "busy"),
			unreadChannel("C03AAAAAAAA", "busier"),
		},
		unread: map[string]int{"C02AAAAAAAA": 3, "C03AAAAAAAA": 9},
	}

	stdout, _, err := execUnread(t, f, "--min", "5")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "#busy ") || !strings.Contains(stdout, "#busier") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestUnreadBotTokenFastFail(t *testing.T) {
	t.Setenv("SLACK_TOKEN", "xoxb-fake")
	f := &fakeUnreadClient{}

	_, stderr, err := execUnread(t, f)
	if err == nil {
		t.Fatal("expected auth error for bot token")
	}
	if !strings.Contains(stderr, "user token") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestUnreadProgressOnStderr(t *testing.T) {
	t.Setenv("SLACK_TOKEN", "xoxp-fake")
	f := &fakeUnreadClient{
		conversations: []slack.Channel{unreadChannel("C02AAAAAAAA", "busy")},
		unread:        map[string]int{"C02AAAAAAAA": 1},
	}

	stdout, stderr, err := execUnread(t, f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "checked 1/1") {
		t.Errorf("stderr = %q", stderr)
	}
	if strings.Contains(stdout, "checked") {
		t.Errorf("progress leaked to stdout: %q", stdout)
	}
}

func TestUnreadJSON(t *testing.T) {
	t.Setenv("SLACK_TOKEN", "xoxp-fake")
	f := &fakeUnreadClient{
		conversations: []slack.Channel{unreadChannel("C02AAAAAAAA", "busy")},
		unread:        map[string]int{"C02AAAAAAAA": 4},
	}

	stdout, _, err := execUnread(t, f, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"unread_display": 4`) {
		t.Errorf("json = %q", stdout)
	}
}
