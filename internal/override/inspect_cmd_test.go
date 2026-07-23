package override

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fakeInspectClient returns canned channel/user info.
type fakeInspectClient struct {
	channel *slack.Channel
	user    *slack.User
}

func (f *fakeInspectClient) GetConversationInfoContext(ctx context.Context, input *slack.GetConversationInfoInput) (*slack.Channel, error) {
	return f.channel, nil
}

func (f *fakeInspectClient) GetUserInfoContext(ctx context.Context, user string) (*slack.User, error) {
	return f.user, nil
}

func execInspect(t *testing.T, client inspectClient, args ...string) (stdout string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "inspect", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runInspect(cmd, client, a[0])
	}
	addOutputFlags(cmd)

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), err
}

func testChannel(name string) *slack.Channel {
	ch := &slack.Channel{}
	ch.ID = "C0AAAAAAAAA"
	ch.Name = name
	ch.NumMembers = 42
	ch.Topic = slack.Topic{Value: "the topic"}
	return ch
}

func TestInspectChannel(t *testing.T) {
	seedResolverCache(t)
	f := &fakeInspectClient{channel: testChannel("general")}

	stdout, err := execInspect(t, f, "#general")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#general", "C0AAAAAAAAA", "42", "the topic", "public_channel"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestInspectUser(t *testing.T) {
	seedResolverCache(t)
	u := &slack.User{ID: "U0AAAAAAAAA", Name: "alice", RealName: "Alice A"}
	u.Profile.Email = "alice@example.com"
	f := &fakeInspectClient{user: u}

	stdout, err := execInspect(t, f, "@alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alice", "U0AAAAAAAAA", "Alice A", "alice@example.com"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestInspectUsergroupFromCache(t *testing.T) {
	seedResolverCache(t)
	f := &fakeInspectClient{}

	stdout, err := execInspect(t, f, "oncall")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "@oncall") || !strings.Contains(stdout, "S0AAAAAAAAA") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestInspectChannelJSON(t *testing.T) {
	seedResolverCache(t)
	f := &fakeInspectClient{channel: testChannel("general")}

	stdout, err := execInspect(t, f, "#general", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"num_members": 42`) {
		t.Errorf("json = %q", stdout)
	}
}

func TestInspectUnknownEntity(t *testing.T) {
	seedResolverCache(t)
	f := &fakeInspectClient{}
	if _, err := execInspect(t, f, "nobody"); err == nil {
		t.Error("expected error for unknown entity")
	}
}
