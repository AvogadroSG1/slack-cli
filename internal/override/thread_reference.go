package override

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/poconnor/slack-cli/internal/validate"
)

var permalinkTimestampPattern = regexp.MustCompile(`^p(\d{10})(\d{6})$`)

type threadReference struct {
	ConversationID string
	ThreadTS       string
}

func resolveThreadReference(args []string, rawURL, channel, ts string) (threadReference, error) {
	if len(args) > 1 {
		return threadReference{}, fmt.Errorf("thread-read accepts at most one positional permalink")
	}
	hasPositional := len(args) == 1
	hasURL := rawURL != ""
	hasExplicit := channel != "" || ts != ""
	modes := 0
	for _, present := range []bool{hasPositional, hasURL, hasExplicit} {
		if present {
			modes++
		}
	}
	if modes != 1 {
		return threadReference{}, fmt.Errorf("provide exactly one of a positional permalink, --url, or --channel with --ts")
	}
	if hasPositional {
		return parseThreadPermalink(args[0])
	}
	if hasURL {
		return parseThreadPermalink(rawURL)
	}
	if channel == "" || ts == "" {
		return threadReference{}, fmt.Errorf("--channel and --ts must be provided together")
	}
	if err := validate.ChannelID(channel); err != nil {
		return threadReference{}, err
	}
	if err := validate.Timestamp(ts); err != nil {
		return threadReference{}, err
	}
	return threadReference{ConversationID: channel, ThreadTS: ts}, nil
}

func parseThreadPermalink(rawURL string) (threadReference, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return threadReference{}, fmt.Errorf("invalid Slack permalink: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return threadReference{}, fmt.Errorf("invalid Slack permalink: scheme must be http or https")
	}
	if !isSlackHostname(u.Hostname()) {
		return threadReference{}, fmt.Errorf("invalid Slack permalink: expected a Slack hostname")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "archives" {
		return threadReference{}, fmt.Errorf("invalid Slack permalink: expected /archives/<conversation>/<message> path")
	}
	conversationID := parts[1]
	if err := validate.ChannelID(conversationID); err != nil {
		return threadReference{}, err
	}
	match := permalinkTimestampPattern.FindStringSubmatch(parts[2])
	if match == nil {
		return threadReference{}, fmt.Errorf("invalid Slack permalink timestamp %q", parts[2])
	}
	threadTS := match[1] + "." + match[2]

	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return threadReference{}, fmt.Errorf("invalid Slack permalink query: %w", err)
	}
	if value, present, err := singleQueryValue(query, "thread_ts"); err != nil {
		return threadReference{}, err
	} else if present {
		if err := validate.Timestamp(value); err != nil {
			return threadReference{}, fmt.Errorf("invalid thread_ts: %w", err)
		}
		threadTS = value
	}
	if value, present, err := singleQueryValue(query, "cid"); err != nil {
		return threadReference{}, err
	} else if present {
		if err := validate.ChannelID(value); err != nil {
			return threadReference{}, fmt.Errorf("invalid cid: %w", err)
		}
		if value != conversationID {
			return threadReference{}, fmt.Errorf("cid %q does not match path conversation %q", value, conversationID)
		}
	}
	if err := validate.Timestamp(threadTS); err != nil {
		return threadReference{}, err
	}
	return threadReference{ConversationID: conversationID, ThreadTS: threadTS}, nil
}

func isSlackHostname(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "slack.com" || strings.HasSuffix(host, ".slack.com")
}

func singleQueryValue(values url.Values, key string) (string, bool, error) {
	items, present := values[key]
	if !present {
		return "", false, nil
	}
	if len(items) != 1 || items[0] == "" {
		return "", false, fmt.Errorf("invalid %s: expected exactly one non-empty value", key)
	}
	return items[0], true, nil
}

func validateThreadFilters(oldest, latest string, limit, maxResults int) error {
	if limit < 0 || limit >= 1000 {
		return fmt.Errorf("--limit must be between 0 and 999")
	}
	if maxResults < 0 {
		return fmt.Errorf("--max-results must be zero or greater")
	}
	if oldest != "" {
		if err := validate.Timestamp(oldest); err != nil {
			return fmt.Errorf("invalid --oldest: %w", err)
		}
	}
	if latest != "" {
		if err := validate.Timestamp(latest); err != nil {
			return fmt.Errorf("invalid --latest: %w", err)
		}
	}
	if oldest != "" && latest != "" && oldest > latest {
		return fmt.Errorf("--oldest must not be after --latest")
	}
	return nil
}
