package override

import (
	"context"
	"errors"
	"time"

	"github.com/slack-go/slack"
)

// historyPageLimit is the per-request message limit for history and replies
// pagination.
const historyPageLimit = 200

// historyFetcher abstracts the conversations.history call so tests can
// provide a fake (same pattern as cache.SlackFetcher).
type historyFetcher interface {
	GetConversationHistoryContext(ctx context.Context, params *slack.GetConversationHistoryParameters) (*slack.GetConversationHistoryResponse, error)
}

// repliesFetcher abstracts the conversations.replies call.
type repliesFetcher interface {
	GetConversationRepliesContext(ctx context.Context, params *slack.GetConversationRepliesParameters) ([]slack.Message, bool, string, error)
}

// historyRepliesFetcher combines history and replies fetching; *slack.Client
// satisfies it.
type historyRepliesFetcher interface {
	historyFetcher
	repliesFetcher
}

// sleepOnRateLimit waits out a *slack.RateLimitedError, respecting ctx.
// Returns true if err was a rate limit and the wait completed.
func sleepOnRateLimit(ctx context.Context, err error) (bool, error) {
	var rle *slack.RateLimitedError
	if !errors.As(err, &rle) {
		return false, nil
	}
	select {
	case <-time.After(rle.RetryAfter):
		return true, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// fetchChannelMessages pulls up to limit messages from channelID and returns
// them oldest-first. oldest may be "" for no lower bound; limit <= 0 means
// no cap. When includeThreads is true, replies for each root message with a
// thread are fetched and spliced in after their root.
func fetchChannelMessages(ctx context.Context, c historyRepliesFetcher, channelID, oldest string, limit int, includeThreads bool) ([]slack.Message, error) {
	var collected []slack.Message
	cursor := ""
	for {
		pageLimit := historyPageLimit
		if limit > 0 && limit-len(collected) < pageLimit {
			pageLimit = limit - len(collected)
		}
		resp, err := c.GetConversationHistoryContext(ctx, &slack.GetConversationHistoryParameters{
			ChannelID: channelID,
			Cursor:    cursor,
			Oldest:    oldest,
			Limit:     pageLimit,
		})
		if err != nil {
			if retried, rerr := sleepOnRateLimit(ctx, err); retried {
				continue
			} else if rerr != nil {
				return nil, rerr
			}
			return nil, err
		}
		collected = append(collected, resp.Messages...)
		if limit > 0 && len(collected) >= limit {
			collected = collected[:limit]
			break
		}
		cursor = resp.ResponseMetaData.NextCursor
		if !resp.HasMore || cursor == "" {
			break
		}
	}

	// conversations.history returns newest-first; reverse to chronological.
	reverseMessages(collected)

	if !includeThreads {
		return collected, nil
	}

	var out []slack.Message
	for _, msg := range collected {
		out = append(out, msg)
		if msg.ReplyCount == 0 || msg.ThreadTimestamp != msg.Timestamp {
			continue
		}
		replies, err := fetchThreadMessages(ctx, c, channelID, msg.ThreadTimestamp)
		if err != nil {
			return nil, err
		}
		// The first reply message is the thread root itself; skip it.
		for _, r := range replies {
			if r.Timestamp != msg.Timestamp {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

// fetchThreadMessages pulls a full thread (all pages), oldest-first. The
// thread root is the first message.
func fetchThreadMessages(ctx context.Context, c repliesFetcher, channelID, threadTS string) ([]slack.Message, error) {
	var collected []slack.Message
	cursor := ""
	for {
		msgs, hasMore, next, err := c.GetConversationRepliesContext(ctx, &slack.GetConversationRepliesParameters{
			ChannelID: channelID,
			Timestamp: threadTS,
			Cursor:    cursor,
			Limit:     historyPageLimit,
		})
		if err != nil {
			if retried, rerr := sleepOnRateLimit(ctx, err); retried {
				continue
			} else if rerr != nil {
				return nil, rerr
			}
			return nil, err
		}
		collected = append(collected, msgs...)
		cursor = next
		if !hasMore || cursor == "" {
			break
		}
	}
	return collected, nil
}

// reverseMessages reverses msgs in place.
func reverseMessages(msgs []slack.Message) {
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
}
