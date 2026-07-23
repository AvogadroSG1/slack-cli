package override

import (
	"bytes"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// execThread builds the thread command around a fake replies fetcher.
func execThread(t *testing.T, client repliesFetcher, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "thread", Args: cobra.RangeArgs(1, 2)}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runThread(cmd, client, a)
	}
	addOutputFlags(cmd)

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func threadMsg(ts, rootTS, user, text string) slack.Message {
	return slack.Message{Msg: slack.Msg{Timestamp: ts, ThreadTimestamp: rootTS, User: user, Text: text}}
}

func TestThreadFromPermalink(t *testing.T) {
	seedResolverCache(t)
	f := &fakeFetcher{replies: map[string][]repliesPage{
		"1775827095.264229": {{messages: []slack.Message{
			threadMsg("1775827095.264229", "1775827095.264229", "U0AAAAAAAAA", "root msg"),
			threadMsg("1775827096.000000", "1775827095.264229", "U0BBBBBBBBB", "a reply"),
		}}},
	}}

	stdout, _, err := execThread(t, f, "https://x.slack.com/archives/C0AAAAAAAAA/p1775827095264229")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "root msg") || !strings.Contains(stdout, "↳") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestThreadFromChannelAndTS(t *testing.T) {
	seedResolverCache(t)
	f := &fakeFetcher{replies: map[string][]repliesPage{
		"1775827095.264229": {{messages: []slack.Message{
			threadMsg("1775827095.264229", "1775827095.264229", "U0AAAAAAAAA", "root msg"),
		}}},
	}}

	stdout, _, err := execThread(t, f, "#general", "1775827095.264229")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "root msg") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestThreadBadTimestamp(t *testing.T) {
	seedResolverCache(t)
	f := &fakeFetcher{}
	if _, _, err := execThread(t, f, "#general", "not-a-ts"); err == nil {
		t.Error("expected input error for bad timestamp")
	}
}

func TestThreadEmptyIsInputError(t *testing.T) {
	seedResolverCache(t)
	f := &fakeFetcher{}
	_, stderr, err := execThread(t, f, "#general", "1775827095.264229")
	if err == nil {
		t.Fatal("expected error for empty thread")
	}
	if !strings.Contains(stderr, "no thread found") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestThreadJSONIncludesThreadTs(t *testing.T) {
	seedResolverCache(t)
	f := &fakeFetcher{replies: map[string][]repliesPage{
		"1775827095.264229": {{messages: []slack.Message{
			threadMsg("1775827095.264229", "1775827095.264229", "U0AAAAAAAAA", "root"),
			threadMsg("1775827096.000000", "1775827095.264229", "U0AAAAAAAAA", "reply"),
		}}},
	}}

	stdout, _, err := execThread(t, f, "#general", "1775827095.264229", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"thread_ts": "1775827095.264229"`) {
		t.Errorf("json missing thread_ts: %q", stdout)
	}
}
