package override

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fakeConversationLister satisfies cache.SlackFetcher.
type fakeConversationLister struct {
	pages       [][]slack.Channel
	call        int
	gotTypes    []string
	gotExclArch bool
}

func (f *fakeConversationLister) GetConversationsContext(ctx context.Context, params *slack.GetConversationsParameters) ([]slack.Channel, string, error) {
	f.gotTypes = params.Types
	f.gotExclArch = params.ExcludeArchived
	if f.call >= len(f.pages) {
		return nil, "", nil
	}
	page := f.pages[f.call]
	f.call++
	next := ""
	if f.call < len(f.pages) {
		next = "cur"
	}
	return page, next, nil
}

func (f *fakeConversationLister) GetUsersContext(ctx context.Context, options ...slack.GetUsersOption) ([]slack.User, error) {
	return nil, nil
}

func (f *fakeConversationLister) GetUserGroupsContext(ctx context.Context, options ...slack.GetUserGroupsOption) ([]slack.UserGroup, error) {
	return nil, nil
}

func listChannel(name string, private bool) slack.Channel {
	ch := slack.Channel{}
	ch.ID = "C0AAAAAAAAA"
	ch.Name = name
	ch.IsPrivate = private
	ch.NumMembers = 7
	return ch
}

func execChannels(t *testing.T, client *fakeConversationLister, args ...string) (stdout string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "channels", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runChannels(cmd, client)
	}
	cmd.Flags().Bool("private", false, "")
	cmd.Flags().Bool("archived", false, "")
	cmd.Flags().String("match", "", "")
	cmd.Flags().Int("limit", 0, "")
	addOutputFlags(cmd)

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), err
}

func TestChannelsListsAcrossPages(t *testing.T) {
	f := &fakeConversationLister{pages: [][]slack.Channel{
		{listChannel("general", false)},
		{listChannel("dev-team", true)},
	}}

	stdout, err := execChannels(t, f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "#general") || !strings.Contains(stdout, "#dev-team") {
		t.Errorf("stdout = %q", stdout)
	}
	if len(f.gotTypes) != 1 || f.gotTypes[0] != "public_channel" {
		t.Errorf("types = %v, want public only by default", f.gotTypes)
	}
	if !f.gotExclArch {
		t.Error("archived channels should be excluded by default")
	}
}

func TestChannelsPrivateFlagAddsType(t *testing.T) {
	f := &fakeConversationLister{}
	if _, err := execChannels(t, f, "--private"); err != nil {
		t.Fatal(err)
	}
	if len(f.gotTypes) != 2 || f.gotTypes[1] != "private_channel" {
		t.Errorf("types = %v", f.gotTypes)
	}
}

func TestChannelsMatchFilter(t *testing.T) {
	f := &fakeConversationLister{pages: [][]slack.Channel{
		{listChannel("general", false), listChannel("dev-team", false)},
	}}

	stdout, err := execChannels(t, f, "--match", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "#general") || !strings.Contains(stdout, "#dev-team") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestChannelsJSON(t *testing.T) {
	f := &fakeConversationLister{pages: [][]slack.Channel{{listChannel("general", false)}}}

	stdout, err := execChannels(t, f, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"name": "general"`) || !strings.Contains(stdout, `"num_members": 7`) {
		t.Errorf("json = %q", stdout)
	}
}
