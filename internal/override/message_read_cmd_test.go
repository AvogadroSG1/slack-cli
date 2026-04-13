package override

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// executeMessageRead runs "message-read" with the given args using a nil client.
func executeMessageRead(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newMessageReadCmd(nil))

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(append([]string{"message-read"}, args...))

	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestMessageReadNilClientReturnsAuthError(t *testing.T) {
	_, stderr, err := executeMessageRead(t, "--url", "https://x.slack.com/archives/C0AFM69EB1B/p1775827095264229")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if !strings.Contains(stderr, "SLACK_TOKEN") {
		t.Errorf("stderr missing SLACK_TOKEN: %s", stderr)
	}
}

func TestMessageReadFlagContract(t *testing.T) {
	// --ts without --channel should fail at Cobra flag validation.
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newMessageReadCmd(nil))
	root.SetArgs([]string{"message-read", "--ts", "1775827095.264229"})

	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error: --ts without --channel")
	}
}

func TestMessageReadMutualExclusivity(t *testing.T) {
	// --url with --channel should fail at Cobra flag validation.
	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newMessageReadCmd(nil))
	root.SetArgs([]string{"message-read", "--url", "https://x.slack.com/archives/C01/p1234567890123456", "--channel", "C01"})

	var errBuf bytes.Buffer
	root.SetErr(&errBuf)
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error: --url and --channel are mutually exclusive")
	}
}
