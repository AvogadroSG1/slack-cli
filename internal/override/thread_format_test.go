package override

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
)

func TestNormalizeThreadMessagesSortsReactionsAndResolvesAuthors(t *testing.T) {
	source := []slack.Message{
		{Msg: slack.Msg{
			User: "U09PETER01", Timestamp: "1784131538.270229", Text: "Deployment complete",
			Reactions: []slack.ItemReaction{
				{Name: "white_check_mark", Count: 4, Users: []string{"U01"}},
				{Name: "eyes", Count: 2, Users: []string{"U01", "U02"}},
			},
			Metadata: slack.SlackMetadata{
				EventType: "deployment_completed", EventPayload: map[string]any{},
			},
		}},
		{Msg: slack.Msg{BotID: "B09BOT000", Timestamp: "1784131630.101010", Text: "Bot reply"}},
		{Msg: slack.Msg{User: "U09UNKNOWN", Timestamp: "1784131700.000001", Text: "Unknown reply"}},
	}

	got := normalizeThreadMessages(source, map[string]string{"U09PETER01": "Peter O'Connor"}, true)
	if got[0].User != "Peter O'Connor" || got[1].User != "[bot]" || got[2].User != "U09UNKNOWN" {
		t.Fatalf("author fallbacks = %q, %q, %q", got[0].User, got[1].User, got[2].User)
	}
	wantReactions := []threadReaction{
		{Name: "eyes", Count: 2},
		{Name: "white_check_mark", Count: 4},
	}
	if diff := cmp.Diff(wantReactions, got[0].Reactions); diff != "" {
		t.Errorf("reactions mismatch (-want +got):\n%s", diff)
	}
	if got[0].Metadata == nil || got[0].Metadata.EventType != "deployment_completed" {
		t.Errorf("metadata = %#v", got[0].Metadata)
	}
	if got[1].Reactions == nil || got[2].Reactions == nil {
		t.Error("messages without reactions MUST use non-nil empty slices")
	}
	if len(source[0].Reactions[0].Users) != 1 {
		t.Error("normalization mutated the SDK message")
	}
}

func TestNormalizeThreadMessagesOmitsUnrequestedAndEmptyMetadata(t *testing.T) {
	withMetadata := []slack.Message{{Msg: slack.Msg{
		Timestamp: "1784131538.270229",
		Metadata: slack.SlackMetadata{
			EventType: "deployment_completed", EventPayload: map[string]any{},
		},
	}}}
	if got := normalizeThreadMessages(withMetadata, nil, false); got[0].Metadata != nil {
		t.Fatalf("unrequested metadata = %#v, want nil", got[0].Metadata)
	}

	withoutMetadata := []slack.Message{{Msg: slack.Msg{Timestamp: "1784131538.270229"}}}
	if got := normalizeThreadMessages(withoutMetadata, nil, true); got[0].Metadata != nil {
		t.Fatalf("empty metadata = %#v, want nil", got[0].Metadata)
	}
}

func TestFormatThreadMessagesHumanSnapshot(t *testing.T) {
	location := time.FixedZone("EDT", -4*60*60)
	previousLocation := time.Local
	time.Local = location
	t.Cleanup(func() { time.Local = previousLocation })
	messages := []threadMessage{
		{
			User: "Peter O'Connor", Time: time.Date(2026, 7, 15, 13, 5, 38, 0, location),
			SlackTS: "1784131538.270229", Text: "The deployment is complete.",
			Reactions: []threadReaction{
				{Name: "eyes", Count: 2},
				{Name: "white_check_mark", Count: 4},
			},
		},
		{
			User: "Brendan Rosage", Time: time.Date(2026, 7, 15, 13, 7, 10, 0, location),
			SlackTS: "1784131630.101010", Text: "Confirmed.", Reactions: []threadReaction{},
		},
	}
	var out bytes.Buffer
	if err := formatThreadMessages(messages, false, &out); err != nil {
		t.Fatalf("formatThreadMessages() error = %v", err)
	}
	want := "Peter O'Connor [2026-07-15 13:05]: The deployment is complete.\n" +
		"  Reactions: :eyes: 2, :white_check_mark: 4\n" +
		"Brendan Rosage [2026-07-15 13:07]: Confirmed.\n"
	if diff := cmp.Diff(want, out.String()); diff != "" {
		t.Errorf("human output mismatch (-want +got):\n%s", diff)
	}
}

func TestFormatThreadMessagesJSONSnapshot(t *testing.T) {
	location := time.FixedZone("EDT", -4*60*60)
	messages := []threadMessage{
		{
			User: "Peter O'Connor", Time: time.Date(2026, 7, 15, 13, 5, 38, 0, location),
			SlackTS: "1784131538.270229", Text: "The deployment is complete.",
			Reactions: []threadReaction{{Name: "eyes", Count: 2}},
			Metadata: &slack.SlackMetadata{
				EventType: "deployment_completed", EventPayload: map[string]any{},
			},
		},
		{
			User: "Brendan Rosage", Time: time.Date(2026, 7, 15, 13, 7, 10, 0, location),
			SlackTS: "1784131630.101010", Text: "Confirmed.", Reactions: []threadReaction{},
		},
	}
	var out bytes.Buffer
	if err := formatThreadMessages(messages, true, &out); err != nil {
		t.Fatalf("formatThreadMessages() error = %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("JSON output = %q: %v", out.String(), err)
	}
	if _, present := got[1]["metadata"]; present {
		t.Error("metadata unexpectedly present on second message")
	}
	if reactions, ok := got[1]["reactions"].([]any); !ok || len(reactions) != 0 {
		t.Errorf("empty reactions = %#v, want []", got[1]["reactions"])
	}
	if got[0]["slack_ts"] != "1784131538.270229" ||
		got[0]["user"] != "Peter O'Connor" ||
		got[0]["text"] != "The deployment is complete." {
		t.Errorf("stable fields = %#v", got[0])
	}
	metadata, ok := got[0]["metadata"].(map[string]any)
	if !ok || metadata["event_type"] != "deployment_completed" {
		t.Errorf("metadata = %#v", got[0]["metadata"])
	}
}

func TestWriteThreadIncompleteStatus(t *testing.T) {
	tests := []struct {
		name string
		json bool
		want string
	}{
		{
			name: "human",
			want: "Warning: result limited by --max-results; resume with --cursor dXNlcjp...\n",
		},
		{
			name: "json", json: true,
			want: "{\n  \"complete\": false,\n  \"reason\": \"max_results\",\n  \"next_cursor\": \"dXNlcjp...\"\n}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeThreadIncompleteStatus(&out, tt.json, "dXNlcjp..."); err != nil {
				t.Fatalf("writeThreadIncompleteStatus() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, out.String()); diff != "" {
				t.Errorf("status mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestThreadFormattersPropagateWriteErrors(t *testing.T) {
	messages := []threadMessage{{User: "Peter", Time: time.Now(), Reactions: []threadReaction{}}}
	if err := formatThreadMessages(messages, false, &errWriter{}); err == nil {
		t.Error("human formatter error = nil")
	}
	if err := formatThreadMessages(messages, true, &errWriter{}); err == nil {
		t.Error("JSON formatter error = nil")
	}
	if err := writeThreadIncompleteStatus(&errWriter{}, false, "next"); err == nil {
		t.Error("status formatter error = nil")
	}
}
