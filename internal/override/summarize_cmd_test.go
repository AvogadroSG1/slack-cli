package override

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

// fakeSummarizer records the transcript and returns a fixed summary.
type fakeSummarizer struct {
	gotModel      string
	gotTranscript string
	calls         int
}

func (f *fakeSummarizer) Summarize(ctx context.Context, model, transcript string) (string, error) {
	f.calls++
	f.gotModel = model
	f.gotTranscript = transcript
	return "- decided things", nil
}

func execSummarize(t *testing.T, client readClient, llm summarizer, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	cmd := &cobra.Command{Use: "summarize", Args: cobra.ExactArgs(1)}
	cmd.RunE = func(cmd *cobra.Command, a []string) error {
		return runSummarize(cmd, client, llm, a[0])
	}
	cmd.Flags().String("since", "24h", "")
	cmd.Flags().Int("limit", 200, "")
	cmd.Flags().Bool("include-threads", false, "")
	cmd.Flags().String("model", summarizeDefaultModel, "")
	cmd.Flags().Bool("json", false, "")

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func userMsg(ts, user, text string) slack.Message {
	return slack.Message{Msg: slack.Msg{Timestamp: ts, User: user, Text: text}}
}

func TestSummarizeChannelTranscript(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{fakeFetcher: fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{userMsg("1775827095.264229", "U0AAAAAAAAA", "we should ship <@U0AAAAAAAAA>")}},
	}}}
	llm := &fakeSummarizer{}

	stdout, _, err := execSummarize(t, f, llm, "#general")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "- decided things") {
		t.Errorf("stdout = %q", stdout)
	}
	if llm.gotModel != summarizeDefaultModel {
		t.Errorf("model = %q", llm.gotModel)
	}
	// Transcript is mrkdwn-stripped and name-resolved (idMap is empty in the
	// seeded cache, so the mention keeps the raw ID with an @).
	if strings.Contains(llm.gotTranscript, "<@") {
		t.Errorf("transcript not stripped: %q", llm.gotTranscript)
	}
	if !strings.Contains(llm.gotTranscript, "we should ship") {
		t.Errorf("transcript = %q", llm.gotTranscript)
	}
}

func TestSummarizeThreadPermalink(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{fakeFetcher: fakeFetcher{replies: map[string][]repliesPage{
		"1775827095.264229": {{messages: []slack.Message{
			userMsg("1775827095.264229", "U0AAAAAAAAA", "root"),
			userMsg("1775827096.000000", "U0BBBBBBBBB", "reply"),
		}}},
	}}}
	llm := &fakeSummarizer{}

	_, _, err := execSummarize(t, f, llm, "https://x.slack.com/archives/C0AAAAAAAAA/p1775827095264229")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(llm.gotTranscript, "root") || !strings.Contains(llm.gotTranscript, "reply") {
		t.Errorf("transcript = %q", llm.gotTranscript)
	}
}

func TestSummarizeEmptyWindowSkipsLLM(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{}
	llm := &fakeSummarizer{}

	_, stderr, err := execSummarize(t, f, llm, "#general")
	if err == nil {
		t.Fatal("expected error for empty window")
	}
	if llm.calls != 0 {
		t.Errorf("LLM called %d times for empty window", llm.calls)
	}
	if !strings.Contains(stderr, "nothing to summarize") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestSummarizeJSON(t *testing.T) {
	seedResolverCache(t)
	f := &fakeReadClient{fakeFetcher: fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{userMsg("1775827095.264229", "U0AAAAAAAAA", "hi")}},
	}}}
	llm := &fakeSummarizer{}

	stdout, _, err := execSummarize(t, f, llm, "#general", "--json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"summary": "- decided things"`, `"message_count": 1`, `"model": "claude-opus-4-8"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("json missing %s: %q", want, stdout)
		}
	}
}

func TestSummarizeMissingKeyIsAuthError(t *testing.T) {
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newSummarizeCmd(&slack.Client{}))
	t.Setenv("ANTHROPIC_API_KEY", "")

	var errBuf bytes.Buffer
	root.SetOut(&errBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"summarize", "#general"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing ANTHROPIC_API_KEY")
	}
	if !strings.Contains(errBuf.String(), "ANTHROPIC_API_KEY") {
		t.Errorf("stderr = %q", errBuf.String())
	}
}

func TestAnthropicErrCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"unauthorized", &anthropic.Error{StatusCode: 401}, exitcode.AuthError},
		{"forbidden", &anthropic.Error{StatusCode: 403}, exitcode.AuthError},
		{"rate limited", &anthropic.Error{StatusCode: 429}, exitcode.APIError},
		{"server error", &anthropic.Error{StatusCode: 500}, exitcode.APIError},
		{"transport", context.DeadlineExceeded, exitcode.NetError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := anthropicErrCode(tt.err); got != tt.want {
				t.Errorf("anthropicErrCode = %d, want %d", got, tt.want)
			}
		})
	}
}
