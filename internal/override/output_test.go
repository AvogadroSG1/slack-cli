package override

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
)

func TestStripMrkdwn(t *testing.T) {
	idMap := map[string]string{"U0AAAAAAAAA": "alice"}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"known user mention", "hi <@U0AAAAAAAAA>", "hi @alice"},
		{"unknown user mention", "hi <@U0BBBBBBBBB>", "hi @U0BBBBBBBBB"},
		{"channel mention", "see <#C0AAAAAAAAA|general>", "see #general"},
		{"special mention", "<!here> please", "@here please"},
		{"labelled link", "docs at <https://example.com|the docs>", "docs at the docs (https://example.com)"},
		{"bare link", "see <https://example.com>", "see https://example.com"},
		{"bold italic strike code", "*bold* and _ital_ and ~gone~ and `code`", "bold and ital and gone and code"},
		{"emoji removed", "ship it :rocket:", "ship it "},
		{"html entities", "a &lt;b&gt; &amp; c", "a <b> & c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripMrkdwn(tt.in, idMap); got != tt.want {
				t.Errorf("stripMrkdwn(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRenderOutputModes(t *testing.T) {
	rows := []semMessage{{User: "alice", Ts: "1.000001", Text: "hello"}}
	textFn := func(w io.Writer) error {
		_, err := io.WriteString(w, "TEXT\n")
		return err
	}

	t.Run("default calls textFn", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderOutput(&buf, outputOpts{}, rows, textFn); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "TEXT\n" {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("json emits array", func(t *testing.T) {
		var buf bytes.Buffer
		if err := renderOutput(&buf, outputOpts{JSON: true}, rows, textFn); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), `"user": "alice"`) {
			t.Errorf("json output missing user: %s", buf.String())
		}
	})

	t.Run("template renders rows", func(t *testing.T) {
		var buf bytes.Buffer
		opts := outputOpts{Template: `{{range .}}{{.User}}: {{.Text}}{{end}}`}
		if err := renderOutput(&buf, opts, rows, textFn); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "alice: hello" {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("bad template errors", func(t *testing.T) {
		var buf bytes.Buffer
		err := renderOutput(&buf, outputOpts{Template: "{{"}, rows, textFn)
		if err == nil || !strings.Contains(err.Error(), "--template") {
			t.Errorf("expected template error, got %v", err)
		}
	})
}

func TestAddOutputFlagsMutualExclusion(t *testing.T) {
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
	addOutputFlags(cmd)
	cmd.SetArgs([]string{"--json", "--plain"})
	cmd.SetErr(io.Discard)
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for --json with --plain")
	}
}

func TestFormatSemMessagesText(t *testing.T) {
	ts := time.Date(2026, 7, 23, 10, 30, 0, 0, time.Local)
	msgs := []semMessage{
		{User: "alice", Ts: "1.0", Time: ts.Format(time.RFC3339), Text: "root", ThreadTs: "1.0"},
		{User: "bob", Ts: "2.0", Time: ts.Format(time.RFC3339), Text: "reply", ThreadTs: "1.0"},
	}

	var buf bytes.Buffer
	if err := formatSemMessagesText(msgs, true, false, nil, &buf); err != nil {
		t.Fatal(err)
	}
	want := "alice [" + ts.Format("2006-01-02 15:04") + "]: root\n" +
		"    ↳ bob [" + ts.Format("2006-01-02 15:04") + "]: reply\n"
	if diff := cmp.Diff(want, buf.String()); diff != "" {
		t.Errorf("output mismatch (-want +got):\n%s", diff)
	}
}

func TestToSemMessages(t *testing.T) {
	idMap := map[string]string{"U0AAAAAAAAA": "alice"}
	msgs := []slack.Message{
		{Msg: slack.Msg{User: "U0AAAAAAAAA", Timestamp: "1775827095.264229", Text: "hi", ThreadTimestamp: "1775827095.264229"}},
		{Msg: slack.Msg{BotID: "B01", Timestamp: "1775827096.000000", Text: "beep"}},
	}

	got := toSemMessages(msgs, "#general", idMap)
	if len(got) != 2 {
		t.Fatalf("got %d rows", len(got))
	}
	if got[0].User != "alice" || got[0].Channel != "#general" || got[0].ThreadTs == "" {
		t.Errorf("row 0 = %+v", got[0])
	}
	if got[1].User != "[bot]" {
		t.Errorf("row 1 user = %q, want [bot]", got[1].User)
	}
}
