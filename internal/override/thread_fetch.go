package override

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/slack-go/slack"
)

const (
	defaultThreadPageLimit         = 15
	maxConsecutiveRateLimitRetries = 3
)

var errRepeatedThreadCursor = errors.New("repeated thread pagination cursor")

type threadClient interface {
	GetConversationRepliesContext(context.Context, *slack.GetConversationRepliesParameters) ([]slack.Message, bool, string, error)
}

type threadWaitFunc func(context.Context, time.Duration) error

type threadFetchOptions struct {
	Cursor             string
	Oldest             string
	Latest             string
	Inclusive          bool
	Limit              int
	MaxResults         int
	IncludeAllMetadata bool
	WaitOnRateLimit    bool
}

type threadFetchResult struct {
	Messages   []slack.Message
	Complete   bool
	NextCursor string
}

func fetchThread(
	ctx context.Context,
	client threadClient,
	reference threadReference,
	options threadFetchOptions,
	wait threadWaitFunc,
) (threadFetchResult, error) {
	unique := make(map[string]slack.Message)
	requestedCursors := make(map[string]struct{})
	cursor := options.Cursor

	for {
		if _, repeated := requestedCursors[cursor]; repeated {
			return threadFetchResult{}, fmt.Errorf("%w: %q", errRepeatedThreadCursor, cursor)
		}
		requestedCursors[cursor] = struct{}{}

		params := &slack.GetConversationRepliesParameters{
			ChannelID: reference.ConversationID, Timestamp: reference.ThreadTS, Cursor: cursor,
			Inclusive: options.Inclusive, Latest: options.Latest,
			Limit:  threadPageLimit(options.Limit, options.MaxResults, len(unique)),
			Oldest: options.Oldest, IncludeAllMetadata: options.IncludeAllMetadata,
		}
		messages, nextCursor, err := fetchThreadPage(ctx, client, params, options.WaitOnRateLimit, wait)
		if err != nil {
			return threadFetchResult{}, err
		}
		for _, message := range messages {
			if _, exists := unique[message.Timestamp]; exists {
				continue
			}
			unique[message.Timestamp] = message
			if options.MaxResults > 0 && len(unique) == options.MaxResults {
				break
			}
		}
		if nextCursor == "" {
			return threadFetchResult{Messages: sortedThreadMessages(unique), Complete: true}, nil
		}
		if options.MaxResults > 0 && len(unique) == options.MaxResults {
			return threadFetchResult{
				Messages: sortedThreadMessages(unique), Complete: false, NextCursor: nextCursor,
			}, nil
		}
		cursor = nextCursor
	}
}

func fetchThreadPage(
	ctx context.Context,
	client threadClient,
	params *slack.GetConversationRepliesParameters,
	waitOnRateLimit bool,
	wait threadWaitFunc,
) ([]slack.Message, string, error) {
	retries := 0
	for {
		messages, _, nextCursor, err := client.GetConversationRepliesContext(ctx, params)
		if err == nil {
			return messages, nextCursor, nil
		}
		var rateLimited *slack.RateLimitedError
		if !errors.As(err, &rateLimited) || !waitOnRateLimit {
			return nil, "", err
		}
		if retries == maxConsecutiveRateLimitRetries {
			return nil, "", err
		}
		retries++
		if err := wait(ctx, rateLimited.RetryAfter); err != nil {
			return nil, "", err
		}
	}
}

func threadPageLimit(limit, maxResults, selected int) int {
	effective := limit
	if effective == 0 {
		effective = defaultThreadPageLimit
	}
	if maxResults == 0 {
		return effective
	}
	remaining := maxResults - selected
	if remaining < effective {
		return remaining
	}
	return effective
}

func sortedThreadMessages(unique map[string]slack.Message) []slack.Message {
	messages := make([]slack.Message, 0, len(unique))
	for _, message := range unique {
		messages = append(messages, message)
	}
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp < messages[j].Timestamp
	})
	return messages
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
