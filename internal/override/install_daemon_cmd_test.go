package override

import (
	"bytes"
	"encoding/xml"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRenderPlist(t *testing.T) {
	tests := []struct {
		name      string
		data      plistData
		wantRunAt string // the RunAtLoad element expected in the output
	}{
		{
			name: "run at load false",
			data: plistData{
				Label:      "com.slack-cli.prime",
				ExecPath:   "/usr/local/bin/slack-cli",
				StdoutPath: "/Users/me/Library/Logs/slack-cli/prime.out.log",
				StderrPath: "/Users/me/Library/Logs/slack-cli/prime.err.log",
				RunAtLoad:  false,
			},
			wantRunAt: "<false/>",
		},
		{
			name: "run at load true",
			data: plistData{
				Label:      "com.slack-cli.prime",
				ExecPath:   "/usr/local/bin/slack-cli",
				StdoutPath: "/Users/me/Library/Logs/slack-cli/prime.out.log",
				StderrPath: "/Users/me/Library/Logs/slack-cli/prime.err.log",
				RunAtLoad:  true,
			},
			wantRunAt: "<true/>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := renderPlist(tt.data)
			if err != nil {
				t.Fatalf("renderPlist: %v", err)
			}

			// Well-formed XML.
			if err := xml.Unmarshal([]byte(out), new(struct{})); err != nil {
				t.Fatalf("rendered plist is not well-formed XML: %v\n%s", err, out)
			}

			wants := []string{
				"<string>com.slack-cli.prime</string>",
				"<string>/bin/zsh</string>",
				"<string>-lc</string>",
				"<string>&#39;/usr/local/bin/slack-cli&#39; cache warm</string>",
				"<key>Minute</key>",
				"<integer>0</integer>",
				tt.wantRunAt,
				"<string>/Users/me/Library/Logs/slack-cli/prime.out.log</string>",
				"<string>/Users/me/Library/Logs/slack-cli/prime.err.log</string>",
			}
			for _, want := range wants {
				if !strings.Contains(out, want) {
					t.Errorf("plist missing %q\n--- full output ---\n%s", want, out)
				}
			}
		})
	}
}

func TestRenderPlistEscaping(t *testing.T) {
	out, err := renderPlist(plistData{
		Label:      "com.acme.daemon",
		ExecPath:   "/Users/a b/Tools & Bin/slack-cli",
		StdoutPath: "/tmp/out.log",
		StderrPath: "/tmp/err.log",
	})
	if err != nil {
		t.Fatalf("renderPlist: %v", err)
	}

	// Must stay well-formed even with spaces and an ampersand in the path.
	if err := xml.Unmarshal([]byte(out), new(struct{})); err != nil {
		t.Fatalf("rendered plist is not well-formed XML: %v\n%s", err, out)
	}

	// The raw ampersand must be XML-escaped, not left bare.
	if strings.Contains(out, "Bin & Bin") || strings.Contains(out, "& Bin") {
		t.Errorf("ampersand not escaped in output:\n%s", out)
	}
	if !strings.Contains(out, "&amp;") {
		t.Errorf("expected escaped ampersand (&amp;) in output:\n%s", out)
	}
	// The single-quoting for the shell should survive (as XML entities).
	if !strings.Contains(out, "&#39;/Users/a b/Tools &amp; Bin/slack-cli&#39; cache warm") {
		t.Errorf("quoted exec path not rendered as expected:\n%s", out)
	}
}

func TestPlistPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	got, err := plistPath("com.slack-cli.prime")
	if err != nil {
		t.Fatalf("plistPath: %v", err)
	}
	want := filepath.Join(tmp, "Library", "LaunchAgents", "com.slack-cli.prime.plist")
	if got != want {
		t.Errorf("plistPath = %q, want %q", got, want)
	}
}

func TestLogPaths(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	out, errPath, err := logPaths()
	if err != nil {
		t.Fatalf("logPaths: %v", err)
	}
	wantOut := filepath.Join(tmp, "Library", "Logs", "slack-cli", "prime.out.log")
	wantErr := filepath.Join(tmp, "Library", "Logs", "slack-cli", "prime.err.log")
	if out != wantOut {
		t.Errorf("out = %q, want %q", out, wantOut)
	}
	if errPath != wantErr {
		t.Errorf("err = %q, want %q", errPath, wantErr)
	}
}

func TestInstallDaemonNonDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("non-darwin guard test is meaningless on darwin")
	}

	root := &cobra.Command{Use: "slack-cli"}
	root.AddCommand(newInstallDaemonCmd())

	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"install-daemon"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error on non-darwin platform")
	}
	if !strings.Contains(errBuf.String(), "macOS") {
		t.Errorf("stderr missing 'macOS': %s", errBuf.String())
	}
}
