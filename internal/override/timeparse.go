package override

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/poconnor/slack-cli/internal/validate"
)

// dayWeekRe matches day/week duration shorthand like "2d" or "1w", which
// time.ParseDuration does not support.
var dayWeekRe = regexp.MustCompile(`^(\d+)([dw])$`)

// dateLayouts are the absolute-time formats accepted by parseTimeSpec, tried
// in order. Layouts without a zone are interpreted in local time.
var dateLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02",
}

// parseTimeSpec converts a user-supplied time spec into a Slack timestamp
// string ("1234567890.000000"). Accepted forms:
//   - a raw Slack timestamp, passed through unchanged
//   - Go durations interpreted as "ago": 2h, 30m, 90s
//   - day/week shorthand: 2d, 1w
//   - dates: 2006-01-02, 2006-01-02T15:04[:05], RFC3339
func parseTimeSpec(spec string, now time.Time) (string, error) {
	t, err := parseTimeSpecTime(spec, now)
	if err != nil {
		return "", err
	}
	if validate.Timestamp(spec) == nil {
		return spec, nil
	}
	return slackTS(t), nil
}

// parseTimeSpecAsDate is the same parser but returns "YYYY-MM-DD" for use in
// Slack search query modifiers (after:/before:), which only accept dates.
func parseTimeSpecAsDate(spec string, now time.Time) (string, error) {
	t, err := parseTimeSpecTime(spec, now)
	if err != nil {
		return "", err
	}
	return t.Local().Format("2006-01-02"), nil
}

// parseTimeSpecTime resolves a spec to a time.Time.
func parseTimeSpecTime(spec string, now time.Time) (time.Time, error) {
	if spec == "" {
		return time.Time{}, fmt.Errorf("empty time spec")
	}

	if validate.Timestamp(spec) == nil {
		return parseSlackTimestamp(spec), nil
	}

	if m := dayWeekRe.FindStringSubmatch(spec); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time spec %q: %w", spec, err)
		}
		days := n
		if m[2] == "w" {
			days = n * 7
		}
		return now.Add(-time.Duration(days) * 24 * time.Hour), nil
	}

	if d, err := time.ParseDuration(spec); err == nil {
		if d < 0 {
			return time.Time{}, fmt.Errorf("invalid time spec %q: duration must be positive", spec)
		}
		return now.Add(-d), nil
	}

	for _, layout := range dateLayouts {
		loc := time.Local
		if t, err := time.ParseInLocation(layout, spec, loc); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid time spec %q (expected a duration like 2h or 3d, a date like 2026-07-01, or a Slack timestamp)", spec)
}

// slackTS formats a time.Time as a Slack timestamp string.
func slackTS(t time.Time) string {
	return fmt.Sprintf("%d.%06d", t.Unix(), t.Nanosecond()/1000)
}
