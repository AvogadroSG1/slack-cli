package override

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/slack-go/slack"
)

type fakeThreadPage struct {
	messages   []slack.Message
	nextCursor string
	err        error
}

type fakeThreadClient struct {
	pages []fakeThreadPage
	calls []slack.GetConversationRepliesParameters
}

func (f *fakeThreadClient) GetConversationRepliesContext(
	_ context.Context,
	params *slack.GetConversationRepliesParameters,
) ([]slack.Message, bool, string, error) {
	f.calls = append(f.calls, *params)
	if len(f.pages) == 0 {
		return nil, false, "", fmt.Errorf("unexpected conversations.replies call")
	}
	page := f.pages[0]
	f.pages = f.pages[1:]
	return page.messages, page.nextCursor != "", page.nextCursor, page.err
}

func slackMessage(ts string) slack.Message {
	return slack.Message{Msg: slack.Msg{Timestamp: ts, Text: ts}}
}

func noWait(context.Context, time.Duration) error { return nil }

func TestFetchThreadPaginatesDeduplicatesSortsAndCarriesFilters(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{
		{messages: []slack.Message{slackMessage("1784131630.101010"), slackMessage("1784131538.270229")}, nextCursor: "page-2"},
		{messages: []slack.Message{slackMessage("1784131630.101010"), slackMessage("1784131700.000001")}},
	}}
	options := threadFetchOptions{
		Cursor: "resume-here", Oldest: "1784131500.000000", Latest: "1784131800.000000",
		Inclusive: true, Limit: 2, MaxResults: 10, IncludeAllMetadata: true,
	}
	got, err := fetchThread(context.Background(), client, threadReference{
		ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229",
	}, options, noWait)
	if err != nil {
		t.Fatalf("fetchThread() error = %v", err)
	}
	gotTS := make([]string, len(got.Messages))
	for i, message := range got.Messages {
		gotTS[i] = message.Timestamp
	}
	wantTS := []string{"1784131538.270229", "1784131630.101010", "1784131700.000001"}
	if diff := cmp.Diff(wantTS, gotTS); diff != "" {
		t.Errorf("timestamps mismatch (-want +got):\n%s", diff)
	}
	if !got.Complete || got.NextCursor != "" {
		t.Errorf("complete/cursor = %v/%q, want true/empty", got.Complete, got.NextCursor)
	}
	if len(client.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(client.calls))
	}
	for i, call := range client.calls {
		if call.ChannelID != "C09M260TY7Q" || call.Timestamp != "1784131538.270229" ||
			call.Oldest != options.Oldest || call.Latest != options.Latest ||
			!call.Inclusive || !call.IncludeAllMetadata || call.Limit != 2 {
			t.Errorf("call %d lost options: %#v", i, call)
		}
	}
	if client.calls[0].Cursor != "resume-here" || client.calls[1].Cursor != "page-2" {
		t.Errorf("cursor sequence = %q, %q", client.calls[0].Cursor, client.calls[1].Cursor)
	}
}

func TestFetchThreadMaximumResultSemantics(t *testing.T) {
	tests := []struct {
		name         string
		options      threadFetchOptions
		pages        []fakeThreadPage
		wantCount    int
		wantComplete bool
		wantCursor   string
		wantLimits   []int
	}{
		{
			name: "exact cap with empty cursor", options: threadFetchOptions{Limit: 2, MaxResults: 2},
			pages:     []fakeThreadPage{{messages: []slack.Message{slackMessage("1784131538.270229"), slackMessage("1784131630.101010")}}},
			wantCount: 2, wantComplete: true, wantLimits: []int{2},
		},
		{
			name: "cap with next cursor", options: threadFetchOptions{Limit: 5, MaxResults: 2},
			pages:     []fakeThreadPage{{messages: []slack.Message{slackMessage("1784131538.270229"), slackMessage("1784131630.101010")}, nextCursor: "resume"}},
			wantCount: 2, wantCursor: "resume", wantLimits: []int{2},
		},
		{
			name: "remaining capacity shrinks final page", options: threadFetchOptions{Limit: 2, MaxResults: 3},
			pages: []fakeThreadPage{
				{messages: []slack.Message{slackMessage("1784131538.270229"), slackMessage("1784131630.101010")}, nextCursor: "next"},
				{messages: []slack.Message{slackMessage("1784131700.000001")}},
			},
			wantCount: 3, wantComplete: true, wantLimits: []int{2, 1},
		},
		{
			name: "zero limit uses remaining capacity", options: threadFetchOptions{MaxResults: 3},
			pages:     []fakeThreadPage{{messages: []slack.Message{slackMessage("1784131538.270229")}}},
			wantCount: 1, wantComplete: true, wantLimits: []int{3},
		},
		{
			name: "zero maximum is unlimited", options: threadFetchOptions{Limit: 2},
			pages: []fakeThreadPage{
				{messages: []slack.Message{slackMessage("1784131538.270229"), slackMessage("1784131630.101010")}, nextCursor: "next"},
				{messages: []slack.Message{slackMessage("1784131700.000001")}},
			},
			wantCount: 3, wantComplete: true, wantLimits: []int{2, 2},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeThreadClient{pages: tt.pages}
			got, err := fetchThread(context.Background(), client, threadReference{
				ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229",
			}, tt.options, noWait)
			if err != nil {
				t.Fatalf("fetchThread() error = %v", err)
			}
			if len(got.Messages) != tt.wantCount || got.Complete != tt.wantComplete || got.NextCursor != tt.wantCursor {
				t.Errorf("result = %d/%v/%q, want %d/%v/%q", len(got.Messages), got.Complete, got.NextCursor, tt.wantCount, tt.wantComplete, tt.wantCursor)
			}
			gotLimits := make([]int, len(client.calls))
			for i, call := range client.calls {
				gotLimits[i] = call.Limit
			}
			if diff := cmp.Diff(tt.wantLimits, gotLimits); diff != "" {
				t.Errorf("limits mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFetchThreadStandaloneMessageIsComplete(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{{
		messages: []slack.Message{slackMessage("1784131538.270229")},
	}}}
	got, err := fetchThread(context.Background(), client, threadReference{
		ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229",
	}, threadFetchOptions{}, noWait)
	if err != nil {
		t.Fatalf("fetchThread() error = %v", err)
	}
	if !got.Complete || got.NextCursor != "" || len(got.Messages) != 1 {
		t.Errorf("result = %d/%v/%q, want 1/true/empty", len(got.Messages), got.Complete, got.NextCursor)
	}
}

func TestFetchThreadRejectsRepeatedCursor(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{
		{messages: []slack.Message{slackMessage("1784131538.270229")}, nextCursor: "repeat"},
		{messages: []slack.Message{slackMessage("1784131630.101010")}, nextCursor: "repeat"},
	}}
	got, err := fetchThread(context.Background(), client, threadReference{
		ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229",
	}, threadFetchOptions{}, noWait)
	if !errors.Is(err, errRepeatedThreadCursor) {
		t.Fatalf("error = %v, want errRepeatedThreadCursor", err)
	}
	if len(got.Messages) != 0 {
		t.Fatalf("returned %d partial messages", len(got.Messages))
	}
}

func TestFetchThreadRateLimitPolicy(t *testing.T) {
	rateLimit := &slack.RateLimitedError{RetryAfter: 25 * time.Millisecond}
	tests := []struct {
		name        string
		wait        bool
		pages       []fakeThreadPage
		wantCalls   int
		wantWaits   int
		wantErr     bool
		wantMessage int
	}{
		{name: "no wait", pages: []fakeThreadPage{{err: rateLimit}}, wantCalls: 1, wantErr: true},
		{
			name: "wait and succeed", wait: true,
			pages:     []fakeThreadPage{{err: rateLimit}, {err: rateLimit}, {messages: []slack.Message{slackMessage("1784131538.270229")}}},
			wantCalls: 3, wantWaits: 2, wantMessage: 1,
		},
		{
			name: "fourth response exhausts three retries", wait: true,
			pages:     []fakeThreadPage{{err: rateLimit}, {err: rateLimit}, {err: rateLimit}, {err: rateLimit}},
			wantCalls: 4, wantWaits: 3, wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeThreadClient{pages: tt.pages}
			waits := 0
			wait := func(_ context.Context, delay time.Duration) error {
				waits++
				if delay != 25*time.Millisecond {
					t.Errorf("delay = %s, want 25ms", delay)
				}
				return nil
			}
			got, err := fetchThread(context.Background(), client, threadReference{
				ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229",
			}, threadFetchOptions{WaitOnRateLimit: tt.wait}, wait)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(client.calls) != tt.wantCalls || waits != tt.wantWaits || len(got.Messages) != tt.wantMessage {
				t.Errorf("calls/waits/messages = %d/%d/%d, want %d/%d/%d", len(client.calls), waits, len(got.Messages), tt.wantCalls, tt.wantWaits, tt.wantMessage)
			}
		})
	}
}

func TestWaitForRetryIsCancellable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitForRetry(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestFetchThreadRateLimitRetryCountResetsAfterSuccessfulPage(t *testing.T) {
	rateLimit := &slack.RateLimitedError{RetryAfter: time.Millisecond}
	client := &fakeThreadClient{pages: []fakeThreadPage{
		{err: rateLimit},
		{err: rateLimit},
		{err: rateLimit},
		{messages: []slack.Message{slackMessage("1784131538.270229")}, nextCursor: "next"},
		{err: rateLimit},
		{err: rateLimit},
		{err: rateLimit},
		{messages: []slack.Message{slackMessage("1784131630.101010")}},
	}}
	waits := 0
	got, err := fetchThread(context.Background(), client, threadReference{
		ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229",
	}, threadFetchOptions{WaitOnRateLimit: true}, func(context.Context, time.Duration) error {
		waits++
		return nil
	})
	if err != nil {
		t.Fatalf("fetchThread() error = %v", err)
	}
	if waits != 6 || len(got.Messages) != 2 || !got.Complete {
		t.Errorf("waits/messages/complete = %d/%d/%v, want 6/2/true", waits, len(got.Messages), got.Complete)
	}
}

func TestFetchThreadLaterPageFailureReturnsNoPartialResult(t *testing.T) {
	client := &fakeThreadClient{pages: []fakeThreadPage{
		{messages: []slack.Message{slackMessage("1784131538.270229")}, nextCursor: "next"},
		{err: slack.SlackErrorResponse{Err: "thread_not_found"}},
	}}
	got, err := fetchThread(context.Background(), client, threadReference{
		ConversationID: "C09M260TY7Q", ThreadTS: "1784131538.270229",
	}, threadFetchOptions{}, noWait)
	if err == nil {
		t.Fatal("error = nil")
	}
	if len(got.Messages) != 0 {
		t.Fatalf("returned %d partial messages", len(got.Messages))
	}
}
