package override

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// errWriter always fails writes — used to test error propagation.
type errWriter struct{}

func (e *errWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write failed")
}

func TestFormatMessagesText(t *testing.T) {
	msgs := []readMessage{
		{User: "Peter O'Connor", Time: time.Unix(1712746695, 0), Text: "Hello world"},
		{User: "[bot]", Time: time.Unix(1712746800, 0), Text: "Bot response"},
		{User: "U01UNKNOWN", Time: time.Unix(1712746900, 0), Text: "Unresolved user"},
	}

	var buf bytes.Buffer
	if err := formatMessages(msgs, false, &buf); err != nil {
		t.Fatalf("formatMessages: %v", err)
	}

	out := buf.String()
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %s", len(lines), out)
	}

	for _, want := range []string{"Peter O'Connor", "[bot]", "U01UNKNOWN", "Hello world", "Bot response", "Unresolved user"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}

	// Verify format: "Name [YYYY-MM-DD HH:MM]: text\n"
	if !strings.Contains(lines[0], "[") || !strings.Contains(lines[0], "]: Hello world") {
		t.Errorf("unexpected line format: %q", lines[0])
	}
}

func TestFormatMessagesJSON(t *testing.T) {
	msgs := []readMessage{
		{User: "Peter O'Connor", Time: time.Unix(1712746695, 0), Text: "Hello world"},
		{User: "U99", Time: time.Unix(1712746800, 0), Text: "Raw ID user"},
	}

	var buf bytes.Buffer
	if err := formatMessages(msgs, true, &buf); err != nil {
		t.Fatalf("formatMessages JSON: %v", err)
	}

	var items []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &items); err != nil {
		t.Fatalf("JSON unmarshal: %v\noutput: %s", err, buf.String())
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// First message.
	if items[0]["user"] != "Peter O'Connor" {
		t.Errorf("user = %q, want Peter O'Connor", items[0]["user"])
	}
	if items[0]["text"] != "Hello world" {
		t.Errorf("text = %q, want Hello world", items[0]["text"])
	}
	if _, err := time.Parse(time.RFC3339, items[0]["ts"]); err != nil {
		t.Errorf("ts not RFC3339: %q, err: %v", items[0]["ts"], err)
	}

	// Second message: raw user ID in "user" field.
	if items[1]["user"] != "U99" {
		t.Errorf("user = %q, want U99 (raw ID fallback)", items[1]["user"])
	}
}

func TestFormatMessagesWriteError(t *testing.T) {
	msgs := []readMessage{
		{User: "test", Time: time.Now(), Text: "hello"},
	}
	if err := formatMessages(msgs, false, &errWriter{}); err == nil {
		t.Fatal("expected write error to propagate in text mode")
	}
	if err := formatMessages(msgs, true, &errWriter{}); err == nil {
		t.Fatal("expected write error to propagate in JSON mode")
	}
}

func TestResolveUser(t *testing.T) {
	idMap := map[string]string{
		"U01": "Peter O'Connor",
		"U02": "Jane Smith",
	}

	tests := []struct {
		name   string
		userID string
		botID  string
		want   string
	}{
		{name: "resolved user", userID: "U01", want: "Peter O'Connor"},
		{name: "resolved second", userID: "U02", want: "Jane Smith"},
		{name: "unresolved falls back to ID", userID: "U99", want: "U99"},
		{name: "bot by BotID", userID: "U01", botID: "B01", want: "[bot]"},
		{name: "bot by empty userID", userID: "", want: "[bot]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveUser(tt.userID, tt.botID, idMap)
			if got != tt.want {
				t.Errorf("resolveUser(%q, %q) = %q, want %q", tt.userID, tt.botID, got, tt.want)
			}
		})
	}
}

func TestParseSlackTimestamp(t *testing.T) {
	tests := []struct {
		ts      string
		wantSec int64
	}{
		{ts: "1775827095.264229", wantSec: 1775827095},
		{ts: "1712746695.000000", wantSec: 1712746695},
		{ts: "1712746695", wantSec: 1712746695}, // no fractional part
	}

	for _, tt := range tests {
		t.Run(tt.ts, func(t *testing.T) {
			got := parseSlackTimestamp(tt.ts)
			if got.Unix() != tt.wantSec {
				t.Errorf("parseSlackTimestamp(%q).Unix() = %d, want %d", tt.ts, got.Unix(), tt.wantSec)
			}
		})
	}
}

func TestResolveChannelTS(t *testing.T) {
	tests := []struct {
		name        string
		urlFlag     string
		channelFlag string
		tsFlag      string
		wantChannel string
		wantTS      string
		wantErrStr  string
	}{
		{
			name:        "via url",
			urlFlag:     "https://x.slack.com/archives/C0AFM69/p1234567890123456",
			wantChannel: "C0AFM69",
			wantTS:      "1234567890.123456",
		},
		{
			name:        "via channel and ts flags",
			channelFlag: "C0AFM69",
			tsFlag:      "1234567890.123456",
			wantChannel: "C0AFM69",
			wantTS:      "1234567890.123456",
		},
		{
			name:       "neither url nor flags",
			wantErrStr: "provide either --url or both --channel and --ts",
		},
		{
			name:       "bad url",
			urlFlag:    "https://x.slack.com/nope",
			wantErrStr: "invalid slack url: missing /archives/ path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, ts, err := resolveChannelTSFromValues(tt.urlFlag, tt.channelFlag, tt.tsFlag)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErrStr)
				}
				if err.Error() != tt.wantErrStr {
					t.Errorf("error = %q, want %q", err.Error(), tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if channel != tt.wantChannel {
				t.Errorf("channel = %q, want %q", channel, tt.wantChannel)
			}
			if ts != tt.wantTS {
				t.Errorf("ts = %q, want %q", ts, tt.wantTS)
			}
		})
	}
}
