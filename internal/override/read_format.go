package override

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// readMessage is the normalised representation of a single Slack message
// used by both thread-read and message-read formatters.
type readMessage struct {
	User string    // display name, "[bot]", or raw Slack user ID
	Time time.Time // from time.Unix (UTC); formatter applies local timezone
	Text string
}

// readMessageJSON is the JSON wire format for --json output.
type readMessageJSON struct {
	User string `json:"user"`
	Ts   string `json:"ts"`
	Text string `json:"text"`
}

// formatMessages writes msgs to w. When asJSON is false the output is
// "Name [YYYY-MM-DD HH:MM]: text\n" per message in local time.
// When asJSON is true the output is a JSON array of objects with
// "user", "ts" (RFC3339), and "text" fields.
// All write errors are propagated to the caller.
func formatMessages(msgs []readMessage, asJSON bool, w io.Writer) error {
	if asJSON {
		return formatMessagesJSON(msgs, w)
	}
	return formatMessagesText(msgs, w)
}

func formatMessagesText(msgs []readMessage, w io.Writer) error {
	for _, m := range msgs {
		localTime := m.Time.Local().Format("2006-01-02 15:04")
		if _, err := fmt.Fprintf(w, "%s [%s]: %s\n", m.User, localTime, m.Text); err != nil {
			return err
		}
	}
	return nil
}

func formatMessagesJSON(msgs []readMessage, w io.Writer) error {
	out := make([]readMessageJSON, len(msgs))
	for i, m := range msgs {
		out[i] = readMessageJSON{
			User: m.User,
			Ts:   m.Time.Format(time.RFC3339),
			Text: m.Text,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// resolveUser returns the display name for a Slack message sender.
// Bot detection: if botID is non-empty or userID is empty, returns "[bot]".
// If userID is in idMap, returns the mapped name; otherwise returns userID.
func resolveUser(userID, botID string, idMap map[string]string) string {
	if botID != "" || userID == "" {
		return "[bot]"
	}
	if name, ok := idMap[userID]; ok {
		return name
	}
	return userID
}

// parseSlackTimestamp converts a Slack float timestamp string (e.g.
// "1775827095.264229") to time.Time. Returns the zero value for empty input.
// Assumes well-formed Slack SDK timestamps; a malformed integer part silently
// returns time.Unix(0, 0) rather than an error.
func parseSlackTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	parts := strings.SplitN(ts, ".", 2)
	sec, _ := strconv.ParseInt(parts[0], 10, 64)
	var nsec int64
	if len(parts) == 2 {
		frac := parts[1]
		// Pad to 9 digits (nanoseconds).
		for len(frac) < 9 {
			frac += "0"
		}
		nsec, _ = strconv.ParseInt(frac[:9], 10, 64)
	}
	return time.Unix(sec, nsec)
}

// resolveChannelTSFromValues extracts channel and ts from pre-read flag values.
// If rawURL is non-empty, it is parsed via parseSlackURL.
// Otherwise, channelFlag and tsFlag must both be non-empty.
func resolveChannelTSFromValues(rawURL, channelFlag, tsFlag string) (channel, ts string, err error) {
	if rawURL != "" {
		return parseSlackURL(rawURL)
	}
	if channelFlag == "" || tsFlag == "" {
		return "", "", fmt.Errorf("provide either --url or both --channel and --ts")
	}
	return channelFlag, tsFlag, nil
}
