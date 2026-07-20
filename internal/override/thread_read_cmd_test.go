package override

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/poconnor/slack-cli/internal/dispatch"
	"github.com/poconnor/slack-cli/internal/exitcode"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

func newThreadReadTestRoot(dependencies threadReadDependencies) *cobra.Command {
	root := &cobra.Command{Use: "slack-cli", SilenceUsage: true, SilenceErrors: true}
	flags := root.PersistentFlags()
	flags.Bool("pretty", false, "")
	flags.Bool("all", false, "")
	flags.Int("limit", 0, "")
	flags.String("cursor", "", "")
	flags.Bool("wait-on-rate-limit", false, "")
	flags.Int("max-results", 10000, "")
	root.AddCommand(newThreadReadCmdWithDependencies(dependencies))
	return root
}

func executeThreadRead(
	t *testing.T,
	dependencies threadReadDependencies,
	args ...string,
) (stdout, stderr string, err error) {
	t.Helper()
	root := newThreadReadTestRoot(dependencies)
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"thread-read"}, args...))
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func successfulThreadDependencies(client threadClient) threadReadDependencies {
	return threadReadDependencies{
		client:    client,
		warnCache: func(*cobra.Command) {},
		loadIDToNameMap: func() (map[string]string, error) {
			return map[string]string{"U09PETER01": "Peter O'Connor"}, nil
		},
		wait: noWait,
	}
}

func TestThreadReadPreferredAndLegacyInputs(t *testing.T) {
	permalink := "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229"
	tests := []struct {
		name string
		args []string
	}{
		{name: "positional", args: []string{permalink}},
		{name: "url", args: []string{"--url", permalink}},
		{name: "flags", args: []string{"--channel", "C09M260TY7Q", "--ts", "1784131538.270229"}},
		{name: "redundant all with human pretty", args: []string{permalink, "--all", "--pretty"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeThreadClient{pages: []fakeThreadPage{{messages: []slack.Message{{
				Msg: slack.Msg{
					User: "U09PETER01", Timestamp: "1784131538.270229",
					Text:      "Deployment complete",
					Reactions: []slack.ItemReaction{{Name: "eyes", Count: 2, Users: []string{"U01"}}},
				},
			}}}}}
			stdout, stderr, err := executeThreadRead(t, successfulThreadDependencies(client), tt.args...)
			if err != nil {
				t.Fatalf("error = %v, stderr = %s", err, stderr)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
			if !strings.Contains(stdout, "Peter O'Connor") ||
				!strings.Contains(stdout, "  Reactions: :eyes: 2") {
				t.Errorf("stdout = %q", stdout)
			}
			if len(client.calls) != 1 ||
				client.calls[0].ChannelID != "C09M260TY7Q" ||
				client.calls[0].Timestamp != "1784131538.270229" {
				t.Errorf("Slack calls = %#v", client.calls)
			}
		})
	}
}

func TestThreadReadReplyPermalinkAnchorsAtParent(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{{
		messages: []slack.Message{slackMessage("1784131538.270229")},
	}}}
	permalink := "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?thread_ts=1784131538.270229&cid=C09M260TY7Q"
	_, stderr, err := executeThreadRead(t, successfulThreadDependencies(client), permalink)
	if err != nil {
		t.Fatalf("error = %v, stderr = %s", err, stderr)
	}
	if got := client.calls[0].Timestamp; got != "1784131538.270229" {
		t.Errorf("thread timestamp = %q, want parent", got)
	}
}

func TestThreadReadInputErrorsUseJSONEnvelopeBeforeAuth(t *testing.T) {
	permalink := "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229"
	tests := []struct {
		name string
		args []string
	}{
		{name: "no mode"},
		{name: "too many arguments", args: []string{permalink, permalink}},
		{name: "mode conflict", args: []string{permalink, "--url", permalink}},
		{name: "missing timestamp", args: []string{"--channel", "C09M260TY7Q"}},
		{name: "negative limit", args: []string{permalink, "--limit", "-1"}},
		{name: "non-integer limit", args: []string{permalink, "--limit", "many"}},
		{name: "negative maximum", args: []string{permalink, "--max-results", "-1"}},
		{
			name: "reversed range",
			args: []string{
				permalink, "--oldest", "1784131700.000001",
				"--latest", "1784131538.270229",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := executeThreadRead(t, threadReadDependencies{}, tt.args...)
			if dispatch.ExitCode(err) != exitcode.InputError {
				t.Fatalf("exit code = %d, want %d", dispatch.ExitCode(err), exitcode.InputError)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			var envelope struct {
				OK       bool   `json:"ok"`
				Error    string `json:"error"`
				ExitCode int    `json:"exit_code"`
			}
			if json.Unmarshal([]byte(stderr), &envelope) != nil ||
				envelope.OK || envelope.ExitCode != exitcode.InputError {
				t.Errorf("stderr = %q, want input-error JSON envelope", stderr)
			}
		})
	}
}

func TestThreadReadValidInputWithoutClientReturnsAuthError(t *testing.T) {
	permalink := "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229"
	stdout, stderr, err := executeThreadRead(t, threadReadDependencies{}, permalink)
	if stdout != "" ||
		dispatch.ExitCode(err) != exitcode.AuthError ||
		!strings.Contains(stderr, "SLACK_TOKEN") {
		t.Fatalf("stdout/stderr/code = %q/%q/%d", stdout, stderr, dispatch.ExitCode(err))
	}
}

func TestThreadReadJSONTakesPrecedenceAndReportsIncompleteResult(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{{
		messages:   []slack.Message{slackMessage("1784131538.270229")},
		nextCursor: "resume-cursor",
	}}}
	stdout, stderr, err := executeThreadRead(
		t,
		successfulThreadDependencies(client),
		"https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229",
		"--json", "--pretty", "--max-results", "1",
	)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	var messages []map[string]any
	if err := json.Unmarshal([]byte(stdout), &messages); err != nil || len(messages) != 1 {
		t.Fatalf("stdout = %q: %v", stdout, err)
	}
	if _, present := messages[0]["slack_ts"]; !present {
		t.Errorf("stdout lacks slack_ts: %s", stdout)
	}
	var status threadIncompleteStatus
	if err := json.Unmarshal([]byte(stderr), &status); err != nil {
		t.Fatalf("stderr = %q: %v", stderr, err)
	}
	if status.Complete || status.Reason != "max_results" || status.NextCursor != "resume-cursor" {
		t.Errorf("status = %#v", status)
	}
}

func TestThreadReadHumanReportsIncompleteResult(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{{
		messages:   []slack.Message{slackMessage("1784131538.270229")},
		nextCursor: "resume-cursor",
	}}}
	_, stderr, err := executeThreadRead(
		t,
		successfulThreadDependencies(client),
		"https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229",
		"--max-results", "1",
	)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	want := "Warning: result limited by --max-results; resume with --cursor resume-cursor\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestThreadReadFailuresNeverWritePartialStdout(t *testing.T) {
	tests := []struct {
		name     string
		pages    []fakeThreadPage
		wantCode int
	}{
		{
			name: "later API error",
			pages: []fakeThreadPage{
				{messages: []slack.Message{slackMessage("1784131538.270229")}, nextCursor: "next"},
				{err: slack.SlackErrorResponse{Err: "thread_not_found"}},
			},
			wantCode: exitcode.APIError,
		},
		{
			name: "repeated cursor",
			pages: []fakeThreadPage{
				{messages: []slack.Message{slackMessage("1784131538.270229")}, nextCursor: "repeat"},
				{messages: []slack.Message{slackMessage("1784131630.101010")}, nextCursor: "repeat"},
			},
			wantCode: exitcode.APIError,
		},
		{
			name:     "context cancellation",
			pages:    []fakeThreadPage{{err: context.Canceled}},
			wantCode: exitcode.NetError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeThreadClient{pages: tt.pages}
			stdout, stderr, err := executeThreadRead(
				t,
				successfulThreadDependencies(client),
				"https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229",
			)
			if stdout != "" || dispatch.ExitCode(err) != tt.wantCode {
				t.Fatalf("stdout/code = %q/%d, want empty/%d; stderr = %s", stdout, dispatch.ExitCode(err), tt.wantCode, stderr)
			}
			if !strings.Contains(stderr, "\"ok\": false") {
				t.Errorf("stderr = %q, want error envelope", stderr)
			}
		})
	}
}

func TestThreadReadEmptyResponsePreservesInputExitCode(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{{}}}
	stdout, stderr, err := executeThreadRead(
		t,
		successfulThreadDependencies(client),
		"https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229",
	)
	if stdout != "" ||
		dispatch.ExitCode(err) != exitcode.InputError ||
		!strings.Contains(stderr, "no thread found") {
		t.Fatalf("stdout/stderr/code = %q/%q/%d", stdout, stderr, dispatch.ExitCode(err))
	}
}

func TestThreadReadLoadsCacheOnce(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{
		{messages: []slack.Message{slackMessage("1784131538.270229")}, nextCursor: "next"},
		{messages: []slack.Message{slackMessage("1784131630.101010")}},
	}}
	dependencies := successfulThreadDependencies(client)
	loads := 0
	readinessChecks := 0
	dependencies.warnCache = func(*cobra.Command) { readinessChecks++ }
	dependencies.loadIDToNameMap = func() (map[string]string, error) {
		loads++
		return nil, errors.New("cache unavailable")
	}
	_, stderr, err := executeThreadRead(
		t,
		dependencies,
		"https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229",
	)
	if err != nil {
		t.Fatalf("error = %v, stderr = %s", err, stderr)
	}
	if loads != 1 || readinessChecks != 1 {
		t.Errorf("cache loads/readiness checks = %d/%d, want 1/1", loads, readinessChecks)
	}
}

func TestThreadReadOutputFailureUsesNetworkExitCode(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{{
		messages: []slack.Message{slackMessage("1784131538.270229")},
	}}}
	root := newThreadReadTestRoot(successfulThreadDependencies(client))
	var stderr bytes.Buffer
	root.SetOut(&errWriter{})
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"thread-read",
		"https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229",
	})
	err := root.Execute()
	if dispatch.ExitCode(err) != exitcode.NetError {
		t.Fatalf("exit code = %d, want %d", dispatch.ExitCode(err), exitcode.NetError)
	}
	if !strings.Contains(stderr.String(), "write failed") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
