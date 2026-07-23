package override

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
)

// fakeFetcher implements historyRepliesFetcher with canned pages.
type fakeFetcher struct {
	historyPages []historyPage
	historyCall  int
	// replies maps thread ts to reply pages.
	replies map[string][]repliesPage
	// rateLimitFirst makes the first history call fail with a rate limit.
	rateLimitFirst bool
}

type historyPage struct {
	messages   []slack.Message
	hasMore    bool
	nextCursor string
}

type repliesPage struct {
	messages   []slack.Message
	hasMore    bool
	nextCursor string
}

func (f *fakeFetcher) GetConversationHistoryContext(ctx context.Context, params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
	if f.rateLimitFirst {
		f.rateLimitFirst = false
		return nil, &slack.RateLimitedError{RetryAfter: 0}
	}
	if f.historyCall >= len(f.historyPages) {
		return &slack.GetConversationHistoryResponse{}, nil
	}
	page := f.historyPages[f.historyCall]
	f.historyCall++
	resp := &slack.GetConversationHistoryResponse{
		Messages: page.messages,
		HasMore:  page.hasMore,
	}
	resp.ResponseMetaData.NextCursor = page.nextCursor
	return resp, nil
}

func (f *fakeFetcher) GetConversationRepliesContext(ctx context.Context, params *slack.GetConversationRepliesParameters) ([]slack.Message, bool, string, error) {
	pages := f.replies[params.Timestamp]
	idx := 0
	if params.Cursor != "" {
		for i, p := range pages {
			if p.nextCursor == params.Cursor {
				idx = i + 1
				break
			}
		}
	}
	if idx >= len(pages) {
		return nil, false, "", nil
	}
	p := pages[idx]
	return p.messages, p.hasMore, p.nextCursor, nil
}

func msg(ts, text string) slack.Message {
	return slack.Message{Msg: slack.Msg{Timestamp: ts, Text: text}}
}

func threadRoot(ts, text string, replyCount int) slack.Message {
	m := msg(ts, text)
	m.ThreadTimestamp = ts
	m.ReplyCount = replyCount
	return m
}

func timestamps(msgs []slack.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Timestamp
	}
	return out
}

func TestFetchChannelMessagesReversesAndPaginates(t *testing.T) {
	f := &fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{msg("4.0", "d"), msg("3.0", "c")}, hasMore: true, nextCursor: "cur1"},
		{messages: []slack.Message{msg("2.0", "b"), msg("1.0", "a")}},
	}}

	got, err := fetchChannelMessages(context.Background(), f, "C01", "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1.0", "2.0", "3.0", "4.0"}
	if diff := cmp.Diff(want, timestamps(got)); diff != "" {
		t.Errorf("order mismatch (-want +got):\n%s", diff)
	}
}

func TestFetchChannelMessagesLimit(t *testing.T) {
	f := &fakeFetcher{historyPages: []historyPage{
		{messages: []slack.Message{msg("3.0", "c"), msg("2.0", "b"), msg("1.0", "a")}, hasMore: true, nextCursor: "cur1"},
	}}

	got, err := fetchChannelMessages(context.Background(), f, "C01", "", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d messages, want 2", len(got))
	}
}

func TestFetchChannelMessagesSplicesThreads(t *testing.T) {
	root := threadRoot("1.0", "root", 2)
	f := &fakeFetcher{
		historyPages: []historyPage{{messages: []slack.Message{msg("3.0", "later"), root}}},
		replies: map[string][]repliesPage{
			"1.0": {{messages: []slack.Message{msg("1.0", "root"), msg("1.5", "r1"), msg("2.5", "r2")}}},
		},
	}

	got, err := fetchChannelMessages(context.Background(), f, "C01", "", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1.0", "1.5", "2.5", "3.0"}
	if diff := cmp.Diff(want, timestamps(got)); diff != "" {
		t.Errorf("splice mismatch (-want +got):\n%s", diff)
	}
}

func TestFetchChannelMessagesRetriesRateLimit(t *testing.T) {
	f := &fakeFetcher{
		rateLimitFirst: true,
		historyPages:   []historyPage{{messages: []slack.Message{msg("1.0", "a")}}},
	}

	got, err := fetchChannelMessages(context.Background(), f, "C01", "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d messages, want 1", len(got))
	}
}

func TestFetchChannelMessagesRateLimitRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fetchChannelMessages(ctx, &slowRateLimiter{}, "C01", "", 0, false)
	if err == nil {
		t.Error("expected context error")
	}
}

// slowRateLimiter always returns a rate limit with a long RetryAfter.
type slowRateLimiter struct{}

func (s *slowRateLimiter) GetConversationHistoryContext(ctx context.Context, params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error) {
	return nil, &slack.RateLimitedError{RetryAfter: time.Hour}
}

func (s *slowRateLimiter) GetConversationRepliesContext(ctx context.Context, params *slack.GetConversationRepliesParameters) ([]slack.Message, bool, string, error) {
	return nil, false, "", &slack.RateLimitedError{RetryAfter: time.Hour}
}

func TestFetchThreadMessagesPaginates(t *testing.T) {
	f := &fakeFetcher{replies: map[string][]repliesPage{
		"1.0": {
			{messages: []slack.Message{msg("1.0", "root"), msg("2.0", "r1")}, hasMore: true, nextCursor: "cur1"},
			{messages: []slack.Message{msg("3.0", "r2")}},
		},
	}}

	got, err := fetchThreadMessages(context.Background(), f, "C01", "1.0")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1.0", "2.0", "3.0"}
	if diff := cmp.Diff(want, timestamps(got)); diff != "" {
		t.Errorf("thread mismatch (-want +got):\n%s", diff)
	}
}
