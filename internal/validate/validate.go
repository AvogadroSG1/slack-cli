// Package validate provides input validation for Slack API identifiers and values.
package validate

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

var (
	channelIDPattern = regexp.MustCompile(`^[CDG][A-Z0-9]{8,}$`)
	userIDPattern    = regexp.MustCompile(`^[UWB][A-Z0-9]{8,}$`)
	timestampPattern = regexp.MustCompile(`^\d{10}\.\d{6}$`)

	maxTimeout = 5 * time.Minute
)

// ChannelID validates that v is a valid Slack channel ID.
// Channel IDs start with C, D, or G followed by at least 8 uppercase alphanumeric characters.
func ChannelID(v string) error {
	if v == "" {
		return fmt.Errorf("channel ID must not be empty")
	}
	if !channelIDPattern.MatchString(v) {
		return fmt.Errorf("invalid channel ID %q: must match %s", v, channelIDPattern.String())
	}
	return nil
}

// UserID validates that v is a valid Slack user ID.
// User IDs start with U, W, or B followed by at least 8 uppercase alphanumeric characters.
func UserID(v string) error {
	if v == "" {
		return fmt.Errorf("user ID must not be empty")
	}
	if !userIDPattern.MatchString(v) {
		return fmt.Errorf("invalid user ID %q: must match %s", v, userIDPattern.String())
	}
	return nil
}

// Timestamp validates that v is a valid Slack message timestamp.
// Timestamps have the format: 10 digits, a dot, then 6 digits (e.g. "1234567890.123456").
func Timestamp(v string) error {
	if v == "" {
		return fmt.Errorf("timestamp must not be empty")
	}
	if !timestampPattern.MatchString(v) {
		return fmt.Errorf("invalid timestamp %q: must match %s", v, timestampPattern.String())
	}
	return nil
}

// Timeout validates that v is a valid duration string greater than 0 and at most 5 minutes.
func Timeout(v string) error {
	if v == "" {
		return fmt.Errorf("timeout must not be empty")
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %v", v, err)
	}
	if d <= 0 {
		return fmt.Errorf("timeout must be greater than 0, got %s", d)
	}
	if d > maxTimeout {
		return fmt.Errorf("timeout must not exceed %s, got %s", maxTimeout, d)
	}
	return nil
}

// JSONValue validates that v contains valid JSON.
func JSONValue(v string) error {
	if !json.Valid([]byte(v)) {
		return fmt.Errorf("invalid JSON value")
	}
	return nil
}
