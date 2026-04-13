package cache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

// maxRetries is the number of times to retry an API call after a rate limit.
const maxRetries = 3

// SlackFetcher abstracts the Slack API calls needed for cache warming.
// This allows tests to provide a mock implementation.
type SlackFetcher interface {
	GetConversationsContext(ctx context.Context, params *slack.GetConversationsParameters) ([]slack.Channel, string, error)
	GetUsersContext(ctx context.Context, options ...slack.GetUsersOption) ([]slack.User, error)
	GetUserGroupsContext(ctx context.Context, options ...slack.GetUserGroupsOption) ([]slack.UserGroup, error)
}

// WarmResult holds the counts from a warm operation.
type WarmResult struct {
	Channels   int
	Users      int
	Usergroups int
}

// Warm fetches all channels, users, and usergroups from Slack and writes
// them to separate cache files. It acquires an exclusive lock for the
// duration of the writes.
func Warm(ctx context.Context, fetcher SlackFetcher) (*WarmResult, error) {
	channels, err := fetchAllChannels(ctx, fetcher)
	if err != nil {
		return nil, fmt.Errorf("warm channels: %w", err)
	}

	people, err := fetchAllPeople(ctx, fetcher)
	if err != nil {
		return nil, fmt.Errorf("warm people: %w", err)
	}

	usergroups, err := fetchAllUsergroups(ctx, fetcher)
	if err != nil {
		return nil, fmt.Errorf("warm usergroups: %w", err)
	}

	idToName := buildIDToNameMap(people)

	lock, err := AcquireExclusive()
	if err != nil {
		return nil, fmt.Errorf("warm lock: %w", err)
	}
	defer lock.Close()

	if err := SaveEntity(ChannelsFileName, channels); err != nil {
		return nil, err
	}
	if err := SaveEntity(PeopleFileName, people); err != nil {
		return nil, err
	}
	if err := SaveEntity(UsergroupsFileName, usergroups); err != nil {
		return nil, err
	}
	if err := SaveEntity(IDToNameFileName, idToName); err != nil {
		return nil, err
	}
	if err := SaveMeta(CacheMeta{Version: CurrentVersion}); err != nil {
		return nil, err
	}

	return &WarmResult{
		Channels:   len(channels),
		Users:      len(people),
		Usergroups: len(usergroups),
	}, nil
}

// buildIDToNameMap derives a userID→displayName reverse index from a
// PeopleCache. Prefers DisplayName; falls back to the Slack username key.
// The map is preallocated to the size of people to avoid rehashing.
func buildIDToNameMap(people PeopleCache) map[string]string {
	m := make(map[string]string, len(people))
	for username, entry := range people {
		if entry.ID == "" {
			continue
		}
		name := entry.DisplayName
		if name == "" {
			name = username
		}
		m[entry.ID] = name
	}
	return m
}

// EnrichOnly re-fetches people and usergroups for enrichment without
// touching channels. Used during v1→v2 migration.
func EnrichOnly(ctx context.Context, fetcher SlackFetcher) error {
	people, err := fetchAllPeople(ctx, fetcher)
	if err != nil {
		return fmt.Errorf("enrich people: %w", err)
	}

	usergroups, err := fetchAllUsergroups(ctx, fetcher)
	if err != nil {
		return fmt.Errorf("enrich usergroups: %w", err)
	}

	lock, err := AcquireExclusive()
	if err != nil {
		return fmt.Errorf("enrich lock: %w", err)
	}
	defer lock.Close()

	if err := SaveEntity(PeopleFileName, people); err != nil {
		return err
	}
	if err := SaveEntity(UsergroupsFileName, usergroups); err != nil {
		return err
	}
	return SaveMeta(CacheMeta{Version: CurrentVersion})
}

// withRetry retries fn up to maxRetries times when a rate limit error is
// encountered, waiting the duration specified by the Slack API.
func withRetry[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error
	for attempt := range maxRetries {
		result, err := fn()
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isRateLimited(err) || attempt == maxRetries-1 {
			return zero, err
		}
		waitForRateLimit(ctx, err)
	}
	return zero, lastErr
}

// fetchAllChannels paginates through conversations.list and builds a
// name-to-ID map. Uses inline retry because the API returns multiple
// values (channels + cursor) which don't fit the generic withRetry helper.
func fetchAllChannels(ctx context.Context, f SlackFetcher) (ChannelCache, error) {
	m := make(ChannelCache)
	cursor := ""
	for {
		params := &slack.GetConversationsParameters{
			Cursor: cursor,
			Limit:  200,
			Types:  []string{"public_channel", "private_channel"},
		}

		var channels []slack.Channel
		var nextCursor string
		var err error
		for attempt := range maxRetries {
			channels, nextCursor, err = f.GetConversationsContext(ctx, params)
			if !isRateLimited(err) || attempt == maxRetries-1 {
				break
			}
			waitForRateLimit(ctx, err)
		}
		if err != nil {
			return nil, err
		}

		for _, ch := range channels {
			if ch.Name != "" {
				m[ch.Name] = ch.ID
			}
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor

		select {
		case <-ctx.Done():
			return m, ctx.Err()
		default:
		}
	}

	return m, nil
}

// fetchAllPeople calls users.list and builds enriched people entries.
func fetchAllPeople(ctx context.Context, f SlackFetcher) (PeopleCache, error) {
	users, err := withRetry(ctx, func() ([]slack.User, error) {
		return f.GetUsersContext(ctx)
	})
	if err != nil {
		return nil, err
	}

	m := make(PeopleCache, len(users))
	for _, u := range users {
		if u.Name != "" && !u.Deleted {
			m[u.Name] = UserEntry{
				ID:          u.ID,
				Email:       u.Profile.Email,
				DisplayName: u.Profile.DisplayName,
				Title:       u.Profile.Title,
			}
		}
	}
	return m, nil
}

// fetchAllUsergroups calls usergroups.list with member inclusion and
// builds enriched usergroup entries.
func fetchAllUsergroups(ctx context.Context, f SlackFetcher) (UsergroupCache, error) {
	groups, err := withRetry(ctx, func() ([]slack.UserGroup, error) {
		return f.GetUserGroupsContext(ctx, slack.GetUserGroupsOptionIncludeUsers(true))
	})
	if err != nil {
		return nil, err
	}

	m := make(UsergroupCache, len(groups))
	for _, g := range groups {
		if g.Handle != "" {
			m[g.Handle] = UsergroupEntry{
				ID:          g.ID,
				Description: g.Description,
				Members:     g.Users,
			}
		}
	}
	return m, nil
}

// isRateLimited returns true if err is a Slack rate limit error.
func isRateLimited(err error) bool {
	if err == nil {
		return false
	}
	var rl *slack.RateLimitedError
	if errors.As(err, &rl) {
		return true
	}
	var slackErr slack.SlackErrorResponse
	if errors.As(err, &slackErr) {
		return slackErr.Err == "ratelimited"
	}
	return strings.Contains(err.Error(), "rate limit")
}

// waitForRateLimit sleeps for the duration specified by a rate limit error,
// or a default of 30 seconds if the duration cannot be extracted.
func waitForRateLimit(ctx context.Context, err error) {
	wait := 30 * time.Second

	var rl *slack.RateLimitedError
	if errors.As(err, &rl) && rl.RetryAfter > 0 {
		wait = rl.RetryAfter
	} else if d := parseRetryAfter(err.Error()); d > 0 {
		wait = d
	}

	select {
	case <-time.After(wait):
	case <-ctx.Done():
	}
}

// parseRetryAfter extracts a duration from error messages like
// "slack rate limit exceeded, retry after 30s".
func parseRetryAfter(msg string) time.Duration {
	idx := strings.Index(msg, "retry after ")
	if idx < 0 {
		return 0
	}
	d, err := time.ParseDuration(msg[idx+len("retry after "):])
	if err != nil {
		return 0
	}
	return d
}
