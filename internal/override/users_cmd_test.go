package override

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fakeUsersLister returns a canned user list.
type fakeUsersLister struct {
	users []slack.User
}

func (f *fakeUsersLister) GetUsersContext(ctx context.Context, options ...slack.GetUsersOption) ([]slack.User, error) {
	return f.users, nil
}

func listUser(name, realName, email string, deleted, bot bool) slack.User {
	u := slack.User{ID: "U0AAAAAAAAA", Name: name, RealName: realName, Deleted: deleted, IsBot: bot}
	u.Profile.Email = email
	return u
}

func execUsers(t *testing.T, client usersLister, args ...string) (stdout string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "users"}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runUsers(cmd, client)
	}
	cmd.Flags().String("email", "", "")
	cmd.Flags().String("match", "", "")
	cmd.Flags().Bool("deleted", false, "")
	cmd.Flags().Bool("bots", false, "")
	cmd.Flags().Int("limit", 0, "")
	addOutputFlags(cmd)

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), err
}

func TestUsersExcludesDeletedAndBotsByDefault(t *testing.T) {
	f := &fakeUsersLister{users: []slack.User{
		listUser("alice", "Alice A", "alice@corp.com", false, false),
		listUser("gone", "Gone G", "gone@corp.com", true, false),
		listUser("robot", "Robot R", "", false, true),
	}}

	stdout, err := execUsers(t, f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "alice") || strings.Contains(stdout, "gone") || strings.Contains(stdout, "robot") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestUsersIncludeFlags(t *testing.T) {
	f := &fakeUsersLister{users: []slack.User{
		listUser("gone", "Gone G", "", true, false),
		listUser("robot", "Robot R", "", false, true),
	}}

	stdout, err := execUsers(t, f, "--deleted", "--bots")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "gone") || !strings.Contains(stdout, "robot") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestUsersEmailFilter(t *testing.T) {
	f := &fakeUsersLister{users: []slack.User{
		listUser("alice", "Alice A", "alice@corp.com", false, false),
		listUser("bob", "Bob B", "bob@other.io", false, false),
	}}

	stdout, err := execUsers(t, f, "--email", "@corp.com")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "alice") || strings.Contains(stdout, "bob") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestUsersMatchFilter(t *testing.T) {
	f := &fakeUsersLister{users: []slack.User{
		listUser("alice", "Alice A", "", false, false),
		listUser("bob", "Bob B", "", false, false),
	}}

	stdout, err := execUsers(t, f, "--match", "ali")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "alice") || strings.Contains(stdout, "bob") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestUsersJSON(t *testing.T) {
	f := &fakeUsersLister{users: []slack.User{
		listUser("alice", "Alice A", "alice@corp.com", false, false),
	}}

	stdout, err := execUsers(t, f, "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"username": "alice"`) || !strings.Contains(stdout, `"email": "alice@corp.com"`) {
		t.Errorf("json = %q", stdout)
	}
}
