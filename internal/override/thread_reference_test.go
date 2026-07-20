package override

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseThreadPermalink(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		want    threadReference
		wantErr string
	}{
		{
			name:   "public channel root",
			rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229",
			want:   threadReference{ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229"},
		},
		{
			name:   "direct message root",
			rawURL: "https://stackexchange.slack.com/archives/D09M260TY7Q/p1784131538270229",
			want:   threadReference{ConversationID: "D09M260TY7Q", ThreadTS: "1784131538.270229"},
		},
		{
			name:   "private conversation root",
			rawURL: "http://stackexchange.slack.com/archives/G09M260TY7Q/p1784131538270229",
			want:   threadReference{ConversationID: "G09M260TY7Q", ThreadTS: "1784131538.270229"},
		},
		{
			name:   "reply uses parent thread timestamp",
			rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?thread_ts=1784131538.270229&cid=C09M260TY7Q",
			want:   threadReference{ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229"},
		},
		{name: "missing scheme", rawURL: "stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229", wantErr: "http or https"},
		{name: "non Slack host", rawURL: "https://slack.example.com/archives/C09M260TY7Q/p1784131538270229", wantErr: "Slack hostname"},
		{name: "spoofed suffix", rawURL: "https://stackexchange.slack.com.example.org/archives/C09M260TY7Q/p1784131538270229", wantErr: "Slack hostname"},
		{name: "missing archives", rawURL: "https://stackexchange.slack.com/channels/C09M260TY7Q/p1784131538270229", wantErr: "/archives/"},
		{name: "invalid conversation", rawURL: "https://stackexchange.slack.com/archives/E09M260TY7Q/p1784131538270229", wantErr: "invalid channel ID"},
		{name: "malformed path timestamp", rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538.270229", wantErr: "permalink timestamp"},
		{name: "malformed thread timestamp", rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?thread_ts=bad", wantErr: "thread_ts"},
		{name: "malformed query escape", rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?thread_ts=%ZZ", wantErr: "query"},
		{name: "empty thread timestamp", rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?thread_ts=", wantErr: "thread_ts"},
		{name: "duplicate thread timestamp", rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?thread_ts=1784131538.270229&thread_ts=1784131538.270229", wantErr: "thread_ts"},
		{name: "invalid cid", rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?cid=bad", wantErr: "cid"},
		{name: "mismatched cid", rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?cid=C09M260DIFF", wantErr: "does not match"},
		{name: "duplicate cid", rawURL: "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131630101010?cid=C09M260TY7Q&cid=C09M260TY7Q", wantErr: "cid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseThreadPermalink(tt.rawURL)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("parseThreadPermalink() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseThreadPermalink() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("parseThreadPermalink() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveThreadReference(t *testing.T) {
	permalink := "https://stackexchange.slack.com/archives/C09M260TY7Q/p1784131538270229"
	want := threadReference{ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229"}
	tests := []struct {
		name    string
		args    []string
		rawURL  string
		channel string
		ts      string
		want    threadReference
		wantErr string
	}{
		{name: "positional", args: []string{permalink}, want: want},
		{name: "legacy url", rawURL: permalink, want: want},
		{name: "legacy flags", channel: "C09M260TY7Q", ts: "1784131538.270229", want: want},
		{name: "no mode", wantErr: "exactly one"},
		{name: "two positional", args: []string{permalink, permalink}, wantErr: "at most one"},
		{name: "positional and url", args: []string{permalink}, rawURL: permalink, wantErr: "exactly one"},
		{name: "positional and flags", args: []string{permalink}, channel: "C09M260TY7Q", ts: "1784131538.270229", wantErr: "exactly one"},
		{name: "url and flags", rawURL: permalink, channel: "C09M260TY7Q", ts: "1784131538.270229", wantErr: "exactly one"},
		{name: "channel only", channel: "C09M260TY7Q", wantErr: "together"},
		{name: "timestamp only", ts: "1784131538.270229", wantErr: "together"},
		{name: "bare timestamp", args: []string{"1784131538.270229"}, wantErr: "permalink"},
		{name: "invalid channel", channel: "C01", ts: "1784131538.270229", wantErr: "invalid channel ID"},
		{name: "invalid timestamp", channel: "C09M260TY7Q", ts: "1784131538", wantErr: "invalid timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveThreadReference(tt.args, tt.rawURL, tt.channel, tt.ts)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveThreadReference() error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveThreadReference() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("resolveThreadReference() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateThreadFilters(t *testing.T) {
	tests := []struct {
		name       string
		oldest     string
		latest     string
		limit      int
		maxResults int
		wantErr    string
	}{
		{name: "defaults"},
		{name: "range", oldest: "1784131538.270229", latest: "1784131630.101010", limit: 20, maxResults: 100},
		{name: "largest valid page", limit: 999},
		{name: "Slack cursor limit boundary", limit: 1000, wantErr: "between 0 and 999"},
		{name: "above Slack cursor limit", limit: 1500, wantErr: "between 0 and 999"},
		{name: "negative limit", limit: -1, wantErr: "limit"},
		{name: "negative maximum", maxResults: -1, wantErr: "max-results"},
		{name: "invalid oldest", oldest: "yesterday", wantErr: "oldest"},
		{name: "invalid latest", latest: "tomorrow", wantErr: "latest"},
		{name: "reversed range", oldest: "1784131630.101010", latest: "1784131538.270229", wantErr: "must not be after"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateThreadFilters(tt.oldest, tt.latest, tt.limit, tt.maxResults)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("validateThreadFilters() error = %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("validateThreadFilters() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
