package override

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
)

// tailFake feeds pollTail a fixed page per call.
type tailFake struct {
	pages [][]slack.Message
	call  int
	// oldest records the Oldest param of each call.
	oldest []string
}

func (f *tailFake) GetConversationHistoryContext(ctx context.Context, params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
	f.oldest = append(f.oldest, params.Oldest)
	var msgs []slack.Message
	if f.call < len(f.pages) {
		msgs = f.pages[f.call]
	}
	f.call++
	return &slack.GetConversationHistoryResponse{Messages: msgs}, nil
}

func subtypeMsg(ts, subtype string) slack.Message {
	m := msg(ts, "x")
	m.SubType = subtype
	return m
}

func TestPollTailEmitsNewChronologically(t *testing.T) {
	f := &tailFake{pages: [][]slack.Message{
		{msg("3.0", "c"), msg("2.0", "b")}, // newest-first, as the API returns
	}}
	st := newTailState()
	st.lastTS = "1.0"

	got, err := pollTail(context.Background(), f, "C01", st)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"2.0", "3.0"}
	if diff := cmp.Diff(want, timestamps(got)); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
	if st.lastTS != "3.0" {
		t.Errorf("lastTS = %q, want 3.0", st.lastTS)
	}
	if f.oldest[0] != "1.0" {
		t.Errorf("Oldest param = %q, want 1.0", f.oldest[0])
	}
}

func TestPollTailDedupes(t *testing.T) {
	f := &tailFake{pages: [][]slack.Message{
		{msg("2.0", "b")},
		{msg("2.0", "b"), msg("3.0", "c")},
	}}
	st := newTailState()

	first, err := pollTail(context.Background(), f, "C01", st)
	if err != nil {
		t.Fatal(err)
	}
	second, err := pollTail(context.Background(), f, "C01", st)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || second[0].Timestamp != "3.0" {
		t.Errorf("first = %v, second = %v", timestamps(first), timestamps(second))
	}
}

func TestPollTailSkipsSubtypesButAdvances(t *testing.T) {
	f := &tailFake{pages: [][]slack.Message{
		{subtypeMsg("5.0", "channel_join"), msg("4.0", "real")},
	}}
	st := newTailState()

	got, err := pollTail(context.Background(), f, "C01", st)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Timestamp != "4.0" {
		t.Errorf("got %v, want only 4.0", timestamps(got))
	}
	// The skipped join message still advances the high-water mark.
	if st.lastTS != "5.0" {
		t.Errorf("lastTS = %q, want 5.0", st.lastTS)
	}
}

func TestPollTailSeenPruning(t *testing.T) {
	st := newTailState()
	for i := 0; i <= tailSeenLimit; i++ {
		st.seen[fmt.Sprintf("1000000%03d.000000", i)] = struct{}{}
	}

	f := &tailFake{pages: [][]slack.Message{{}}}
	if _, err := pollTail(context.Background(), f, "C01", st); err != nil {
		t.Fatal(err)
	}
	if len(st.seen) > tailSeenLimit {
		t.Errorf("seen not pruned: %d entries", len(st.seen))
	}
}
