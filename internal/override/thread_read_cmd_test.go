package override

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// executeThreadRead runs "thread-read" with the given args using a nil client
// and returns stdout, stderr, and the error.
func executeThreadRead(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newThreadReadCmd(nil))

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"thread-read"}, args...))

	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestThreadReadNilClientReturnsAuthError(t *testing.T) {
	_, stderr, err := executeThreadRead(t, "--url", "https://x.slack.com/archives/C0AFM69EB1B/p1775827095264229")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if !strings.Contains(stderr, "SLACK_TOKEN") {
		t.Errorf("stderr missing SLACK_TOKEN: %s", stderr)
	}
}

func TestThreadReadBadURLReturnsInputError(t *testing.T) {
	// A nil client check happens first, so we can't reach URL parsing with nil.
	// Test URL parsing independently via parseSlackURL (white-box).
	_, _, err := parseSlackURL("https://x.slack.com/nope")
	if err == nil {
		t.Fatal("expected error for bad URL")
	}
	if !strings.Contains(err.Error(), "invalid slack url") {
		t.Errorf("error = %q, want 'invalid slack url'", err.Error())
	}
}

func TestThreadReadFlagContract(t *testing.T) {
	// --channel without --ts should fail at Cobra flag validation.
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newThreadReadCmd(nil))
	root.SetArgs([]string{"thread-read", "--channel", "C01"})

	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error: --channel without --ts")
	}
}
