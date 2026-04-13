//go:build e2e

package slack_cli_test

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

const binary = "./bin/slack-cli"

// TestE2EVersion verifies that "slack-cli version" exits 0 and produces output.
func TestE2EVersion(t *testing.T) {
	out, err := exec.Command(binary, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("version command failed: %v\noutput: %s", err, out)
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		t.Fatal("version command produced no output")
	}
}

// TestE2EHelp verifies that "slack-cli --help" exits 0 and output contains "slack-cli".
func TestE2EHelp(t *testing.T) {
	out, err := exec.Command(binary, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help command failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "slack-cli") {
		t.Fatalf("expected output to contain %q, got:\n%s", "slack-cli", out)
	}
}

// TestE2EAPIList verifies that "slack-cli api list --json" exits 0 and output
// is a valid JSON array with more than zero entries.
func TestE2EAPIList(t *testing.T) {
	out, err := exec.Command(binary, "api", "list", "--json").CombinedOutput()
	if err != nil {
		t.Fatalf("api list --json failed: %v\noutput: %s", err, out)
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(out, &items); err != nil {
		t.Fatalf("output is not valid JSON array: %v\noutput: %s", err, out)
	}
	if len(items) == 0 {
		t.Fatal("api list --json returned an empty array")
	}
}

// TestE2EAPIListCategory verifies that "slack-cli api list --category chat"
// exits 0 and output contains "chat.postMessage".
func TestE2EAPIListCategory(t *testing.T) {
	out, err := exec.Command(binary, "api", "list", "--category", "chat").CombinedOutput()
	if err != nil {
		t.Fatalf("api list --category chat failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "chat.postMessage") {
		t.Fatalf("expected output to contain %q, got:\n%s", "chat.postMessage", out)
	}
}

// TestE2EChatHelp verifies that "slack-cli chat --help" exits 0 and output
// contains "post-message".
func TestE2EChatHelp(t *testing.T) {
	out, err := exec.Command(binary, "chat", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("chat --help failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "post-message") {
		t.Fatalf("expected output to contain %q, got:\n%s", "post-message", out)
	}
}

// TestE2EConversationsHelp verifies that "slack-cli conversations --help"
// exits 0 and output contains "list".
func TestE2EConversationsHelp(t *testing.T) {
	out, err := exec.Command(binary, "conversations", "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("conversations --help failed: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "list") {
		t.Fatalf("expected output to contain %q, got:\n%s", "list", out)
	}
}
